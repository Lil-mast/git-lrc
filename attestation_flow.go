package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"
)

type attestationPayload struct {
	Action           string  `json:"action"`
	Iterations       int     `json:"iterations"`
	PriorAICovPct    float64 `json:"prior_ai_coverage_pct"`
	PriorReviewCount int     `json:"prior_review_count"`
}

func ensureAttestation(action string, verbose bool, written *bool) error {
	return ensureAttestationFull(attestationPayload{Action: action}, verbose, written)
}

// recordCoverageAndAttest parses the diff, records a review session with coverage stats,
// and writes a full attestation. Used by both the "reviewed" and "vouched" interactive paths.
func recordCoverageAndAttest(action string, diffContent []byte, reviewID string, verbose bool, attestationWritten *bool) error {
	parsedFiles, parseErr := parseDiffToFiles(diffContent)
	if parseErr != nil {
		return fmt.Errorf("could not parse diff for coverage tracking: %w", parseErr)
	}
	cov, covErr := recordAndComputeCoverage(action, parsedFiles, reviewID, verbose)
	if covErr != nil {
		return fmt.Errorf("coverage computation failed: %w", covErr)
	}
	if cov.Iterations == 0 {
		cov.Iterations = 1
	}
	return ensureAttestationFull(attestationPayload{
		Action:           action,
		Iterations:       cov.Iterations,
		PriorAICovPct:    cov.PriorAICovPct,
		PriorReviewCount: cov.PriorReviewCount,
	}, verbose, attestationWritten)
}

func ensureAttestationFull(payload attestationPayload, verbose bool, written *bool) error {
	if written != nil && *written {
		return nil
	}
	if strings.TrimSpace(payload.Action) == "" {
		return nil
	}

	path, err := writeAttestationFullForCurrentTree(payload)
	if err != nil {
		return fmt.Errorf("failed to write attestation: %w", err)
	}
	if verbose {
		log.Printf("Attestation written: %s (action=%s, iter:%d, coverage:%.0f%%)",
			path, payload.Action, payload.Iterations, payload.PriorAICovPct)
	}
	if written != nil {
		*written = true
	}
	return nil
}

// existingAttestationAction returns the attestation action for the current tree, if present.
func existingAttestationAction() (string, error) {
	treeHash, err := currentTreeHash()
	if err != nil {
		return "", err
	}
	if treeHash == "" {
		return "", nil
	}

	gitDir, err := resolveGitDir()
	if err != nil {
		return "", err
	}

	attestPath := filepath.Join(gitDir, "lrc", "attestations", fmt.Sprintf("%s.json", treeHash))
	data, err := os.ReadFile(attestPath)
	if err != nil {
		return "", nil
	}

	var payload attestationPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", nil
	}

	return strings.TrimSpace(payload.Action), nil
}

// readCurrentAttestation reads and parses the full attestation payload for the current tree.
func readCurrentAttestation() (*attestationPayload, error) {
	treeHash, err := currentTreeHash()
	if err != nil {
		return nil, err
	}
	if treeHash == "" {
		return nil, nil
	}

	gitDir, err := resolveGitDir()
	if err != nil {
		return nil, err
	}

	attestPath := filepath.Join(gitDir, "lrc", "attestations", fmt.Sprintf("%s.json", treeHash))
	data, err := os.ReadFile(attestPath)
	if err != nil {
		return nil, nil
	}

	var payload attestationPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("malformed attestation JSON: %w", err)
	}

	return &payload, nil
}

// runAttestationTrailer outputs the formatted commit trailer from the current
// attestation. Called by the commit-msg hook to avoid fragile sed JSON parsing.
// Outputs nothing (and exits 0) if no attestation is present.
func runAttestationTrailer(c *cli.Context) error {
	payload, err := readCurrentAttestation()
	if err != nil {
		return err
	}
	if payload == nil || strings.TrimSpace(payload.Action) == "" {
		return nil
	}

	var trailerVal string
	switch payload.Action {
	case "reviewed":
		trailerVal = "ran"
	case "skipped":
		trailerVal = "skipped"
	case "vouched":
		trailerVal = "vouched"
	default:
		trailerVal = payload.Action
	}

	if payload.Iterations > 0 {
		covPct := int(payload.PriorAICovPct + 0.5)
		trailerVal = fmt.Sprintf("%s (iter:%d, coverage:%d%%)", trailerVal, payload.Iterations, covPct)
	}

	fmt.Printf("LiveReview Pre-Commit Check: %s", trailerVal)
	return nil
}

func writeAttestationForCurrentTree(action string) (string, error) {
	return writeAttestationFullForCurrentTree(attestationPayload{Action: action})
}

func writeAttestationFullForCurrentTree(payload attestationPayload) (string, error) {
	if strings.TrimSpace(payload.Action) == "" {
		return "", fmt.Errorf("attestation action cannot be empty")
	}

	treeHash, err := currentTreeHash()
	if err != nil {
		return "", fmt.Errorf("failed to compute tree hash: %w", err)
	}
	if treeHash == "" {
		return "", fmt.Errorf("empty tree hash")
	}

	gitDir, err := resolveGitDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve git dir: %w", err)
	}
	if !filepath.IsAbs(gitDir) {
		gitDir, err = filepath.Abs(gitDir)
		if err != nil {
			return "", fmt.Errorf("failed to absolutize git dir: %w", err)
		}
	}

	attestDir := filepath.Join(gitDir, "lrc", "attestations")
	if err := os.MkdirAll(attestDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create attestation directory: %w", err)
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal attestation: %w", err)
	}

	tmpFile, err := os.CreateTemp(attestDir, fmt.Sprintf("%s.*.json", treeHash))
	if err != nil {
		return "", fmt.Errorf("failed to create temp attestation file: %w", err)
	}

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("failed to write attestation: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("failed to finalize attestation: %w", err)
	}

	target := filepath.Join(attestDir, fmt.Sprintf("%s.json", treeHash))
	if err := os.Rename(tmpFile.Name(), target); err != nil {
		return "", fmt.Errorf("failed to move attestation into place: %w", err)
	}

	return target, nil
}

func deleteAttestationForCurrentTree() error {
	treeHash, err := currentTreeHash()
	if err != nil {
		return fmt.Errorf("failed to compute tree hash: %w", err)
	}
	if treeHash == "" {
		return nil
	}

	gitDir, err := resolveGitDir()
	if err != nil {
		return fmt.Errorf("failed to resolve git dir: %w", err)
	}

	attestPath := filepath.Join(gitDir, "lrc", "attestations", fmt.Sprintf("%s.json", treeHash))
	if err := os.Remove(attestPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("failed to delete attestation %s: %w", attestPath, err)
	}

	return nil
}
