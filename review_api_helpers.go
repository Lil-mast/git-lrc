package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func runGitCommand(args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("git command failed: %s\nstderr: %s", err, string(exitErr.Stderr))
		}
		return nil, err
	}
	return output, nil
}

func currentTreeHash() (string, error) {
	out, err := runGitCommand("write-tree")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// resolveGitDir returns the absolute path to the repository's .git directory.
func resolveGitDir() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to locate git directory: %w", err)
	}

	gitDir := strings.TrimSpace(string(out))
	if gitDir == "" {
		return "", fmt.Errorf("git directory path is empty")
	}

	if filepath.IsAbs(gitDir) {
		return gitDir, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to resolve working directory: %w", err)
	}

	return filepath.Join(cwd, gitDir), nil
}

func createZipArchive(diffContent []byte) ([]byte, error) {
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	fileWriter, err := zipWriter.Create("diff.txt")
	if err != nil {
		return nil, fmt.Errorf("failed to create zip entry: %w", err)
	}

	if _, err := fileWriter.Write(diffContent); err != nil {
		return nil, fmt.Errorf("failed to write to zip: %w", err)
	}

	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close zip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// formatJSONParseError creates a helpful error message when JSON parsing fails.
// It includes hints about common causes like wrong API URL/port.
func formatJSONParseError(body []byte, contentType string, parseErr error) error {
	bodyStr := string(body)
	preview := bodyStr
	if len(preview) > 200 {
		preview = preview[:200] + "..."
	}

	if strings.HasPrefix(strings.TrimSpace(bodyStr), "<") || strings.Contains(contentType, "text/html") {
		return fmt.Errorf("received HTML instead of JSON (Content-Type: %s).\n"+
			"This usually means api_url in ~/.lrc.toml points to the frontend UI instead of the API.\n"+
			"Check that api_url uses the correct port (default API port is 8888, not 8081).\n"+
			"Response preview: %s", contentType, preview)
	}

	return fmt.Errorf("failed to parse response as JSON: %w\nContent-Type: %s\nResponse preview: %s",
		parseErr, contentType, preview)
}

func submitReview(apiURL, apiKey, base64Diff, repoName string, verbose bool) (diffReviewCreateResponse, error) {
	endpoint := strings.TrimSuffix(apiURL, "/") + "/api/v1/diff-review"

	payload := diffReviewRequest{
		DiffZipBase64: base64Diff,
		RepoName:      repoName,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return diffReviewCreateResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return diffReviewCreateResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)

	if verbose {
		log.Printf("POST %s", endpoint)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return diffReviewCreateResponse{}, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return diffReviewCreateResponse{}, fmt.Errorf("failed to read response: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")

	if resp.StatusCode != http.StatusOK {
		return diffReviewCreateResponse{}, &APIError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	var result diffReviewCreateResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return diffReviewCreateResponse{}, formatJSONParseError(body, contentType, err)
	}

	if result.ReviewID == "" {
		return diffReviewCreateResponse{}, fmt.Errorf("review_id not found in response")
	}

	return result, nil
}

// trackCLIUsage sends a telemetry ping to the backend to track CLI usage
// This is a best-effort call and failures are silently ignored
func trackCLIUsage(apiURL, apiKey string, verbose bool) {
	endpoint := strings.TrimSuffix(apiURL, "/") + "/api/v1/diff-review/cli-used"

	req, err := http.NewRequest("POST", endpoint, nil)
	if err != nil {
		if verbose {
			log.Printf("Failed to create telemetry request: %v", err)
		}
		return
	}

	req.Header.Set("X-API-Key", apiKey)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		if verbose {
			log.Printf("Failed to send telemetry: %v", err)
		}
		return
	}
	defer resp.Body.Close()

	if verbose && resp.StatusCode == http.StatusOK {
		log.Println("CLI usage tracked successfully")
	}
}

var errPollCancelled = errors.New("poll cancelled")
var errInputCancelled = errors.New("terminal input cancelled")

func pollReview(apiURL, apiKey, reviewID string, pollInterval, timeout time.Duration, verbose bool, cancel <-chan struct{}) (*diffReviewResponse, error) {
	endpoint := strings.TrimSuffix(apiURL, "/") + "/api/v1/diff-review/" + reviewID
	deadline := time.Now().Add(timeout)
	start := time.Now()
	fmt.Printf("Waiting for review completion (poll every %s, timeout %s)...\r\n", pollInterval, timeout)
	os.Stdout.Sync()

	if verbose {
		log.Printf("Polling for review completion (timeout: %v)...", timeout)
	}

	for time.Now().Before(deadline) {
		select {
		case <-cancel:
			fmt.Printf("\r\n")
			os.Stdout.Sync()
			return nil, errPollCancelled
		default:
		}

		req, err := http.NewRequest("GET", endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("X-API-Key", apiKey)

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to send request: %w", err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		contentType := resp.Header.Get("Content-Type")

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
		}

		var result diffReviewResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, formatJSONParseError(body, contentType, err)
		}

		statusLine := fmt.Sprintf("Status: %s | elapsed: %s", result.Status, time.Since(start).Truncate(time.Second))
		fmt.Printf("\r%-80s", statusLine)
		os.Stdout.Sync()
		if verbose {
			log.Printf("%s", statusLine)
		}

		if result.Status == "completed" {
			fmt.Printf("\r%-80s\r\n", statusLine)
			os.Stdout.Sync()
			return &result, nil
		}

		if result.Status == "failed" {
			fmt.Printf("\r%-80s\r\n", statusLine)
			os.Stdout.Sync()
			reason := strings.TrimSpace(result.Message)
			if reason == "" {
				reason = "no additional details provided"
			}
			result.Summary = fmt.Sprintf("Review failed: %s", reason)
			return &result, fmt.Errorf("review failed: %s", reason)
		}

		select {
		case <-cancel:
			return nil, errPollCancelled
		case <-time.After(pollInterval):
		}
	}

	fmt.Println()
	return nil, fmt.Errorf("timeout waiting for review completion")
}
