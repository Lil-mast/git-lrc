package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gofrs/flock"
	"github.com/urfave/cli/v2"
)

// =============================================================================
// SELF-UPDATE FUNCTIONALITY
// =============================================================================

// Pre-compiled regexes for version parsing
var (
	semverRe        = regexp.MustCompile(`v?(\d+)\.(\d+)\.(\d+)`)
	b2VersionPathRe = regexp.MustCompile(`^lrc/(v\d+\.\d+\.\d+)/`)
)

// b2AuthResponse models the B2 authorization response
type b2AuthResponse struct {
	AuthorizationToken string `json:"authorizationToken"`
	APIURL             string `json:"apiUrl"`
	DownloadURL        string `json:"downloadUrl"`
}

type pendingUpdateState struct {
	Version          string `json:"version"`
	StagedBinaryPath string `json:"staged_binary_path"`
	DownloadedAt     string `json:"downloaded_at"`
}

type updateLockMetadata struct {
	PID       int    `json:"pid"`
	UID       string `json:"uid,omitempty"`
	Username  string `json:"username,omitempty"`
	Command   string `json:"command"`
	Version   string `json:"version"`
	StartedAt string `json:"started_at"`
}

var autoUpdateStartOnce sync.Once

// b2ListRequest models the B2 list files request
type b2ListRequest struct {
	BucketID      string `json:"bucketId"`
	StartFileName string `json:"startFileName"`
	Prefix        string `json:"prefix"`
	MaxFileCount  int    `json:"maxFileCount"`
}

// b2ListResponse models the B2 list files response
type b2ListResponse struct {
	Files []struct {
		FileName string `json:"fileName"`
	} `json:"files"`
}

// semverParse extracts major, minor, patch from a version string like "v0.1.14"
func semverParse(v string) (int, int, int, bool) {
	match := semverRe.FindStringSubmatch(strings.TrimSpace(v))
	if match == nil {
		return 0, 0, 0, false
	}
	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	patch, _ := strconv.Atoi(match[3])
	return major, minor, patch, true
}

// semverCompare compares two version strings, returns:
// 1 if a > b, -1 if a < b, 0 if equal, error if parsing fails
func semverCompare(a, b string) (int, error) {
	a1, a2, a3, okA := semverParse(a)
	b1, b2, b3, okB := semverParse(b)
	if !okA {
		return 0, fmt.Errorf("invalid version format: %q", a)
	}
	if !okB {
		return 0, fmt.Errorf("invalid version format: %q", b)
	}
	if a1 != b1 {
		if a1 > b1 {
			return 1, nil
		}
		return -1, nil
	}
	if a2 != b2 {
		if a2 > b2 {
			return 1, nil
		}
		return -1, nil
	}
	if a3 != b3 {
		if a3 > b3 {
			return 1, nil
		}
		return -1, nil
	}
	return 0, nil
}

// fetchLatestVersionFromB2 queries B2 to find the latest lrc version
func fetchLatestVersionFromB2() (string, error) {
	authReq, err := http.NewRequest("GET", b2AuthURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create auth request: %w", err)
	}
	authReq.SetBasicAuth(b2KeyID, b2AppKey)

	client := &http.Client{Timeout: 30 * time.Second}
	authResp, err := client.Do(authReq)
	if err != nil {
		return "", fmt.Errorf("B2 auth request failed: %w", err)
	}
	defer authResp.Body.Close()

	if authResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(authResp.Body)
		return "", fmt.Errorf("B2 auth failed with status %d: %s", authResp.StatusCode, string(body))
	}

	var authData b2AuthResponse
	if err := json.NewDecoder(authResp.Body).Decode(&authData); err != nil {
		return "", fmt.Errorf("failed to decode B2 auth response: %w", err)
	}

	listURL := authData.APIURL + "/b2api/v2/b2_list_file_names"
	listReqBody := b2ListRequest{
		BucketID:      b2BucketID,
		StartFileName: b2Prefix + "/",
		Prefix:        b2Prefix + "/",
		MaxFileCount:  1000,
	}
	listBodyBytes, err := json.Marshal(listReqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal list request: %w", err)
	}

	listReq, err := http.NewRequest("POST", listURL, bytes.NewReader(listBodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create list request: %w", err)
	}
	listReq.Header.Set("Authorization", authData.AuthorizationToken)
	listReq.Header.Set("Content-Type", "application/json")

	listResp, err := client.Do(listReq)
	if err != nil {
		return "", fmt.Errorf("B2 list request failed: %w", err)
	}
	defer listResp.Body.Close()

	if listResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(listResp.Body)
		return "", fmt.Errorf("B2 list failed with status %d: %s", listResp.StatusCode, string(body))
	}

	var listData b2ListResponse
	if err := json.NewDecoder(listResp.Body).Decode(&listData); err != nil {
		return "", fmt.Errorf("failed to decode B2 list response: %w", err)
	}

	seen := make(map[string]bool)
	var latestVersion string

	for _, f := range listData.Files {
		match := b2VersionPathRe.FindStringSubmatch(f.FileName)
		if match != nil {
			v := match[1]
			if !seen[v] {
				seen[v] = true
				if latestVersion == "" {
					latestVersion = v
				} else if cmp, err := semverCompare(v, latestVersion); err == nil && cmp > 0 {
					latestVersion = v
				}
			}
		}
	}

	if latestVersion == "" {
		return "", fmt.Errorf("no versions found in B2 bucket")
	}

	return latestVersion, nil
}

func selfUpdateStateDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %w", err)
	}
	return filepath.Join(homeDir, ".lrc", "update"), nil
}

func pendingUpdateStatePath() (string, error) {
	stateDir, err := selfUpdateStateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(stateDir, "pending-update.json"), nil
}

func updateLockPath() (string, error) {
	stateDir, err := selfUpdateStateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(stateDir, "update.lock"), nil
}

func ensureSelfUpdateStateDir() error {
	stateDir, err := selfUpdateStateDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create self-update state directory: %w", err)
	}
	return nil
}

func loadPendingUpdateState() (*pendingUpdateState, error) {
	statePath, err := pendingUpdateStatePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read pending update state: %w", err)
	}

	var state pendingUpdateState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse pending update state: %w", err)
	}

	if strings.TrimSpace(state.Version) == "" || strings.TrimSpace(state.StagedBinaryPath) == "" {
		return nil, fmt.Errorf("pending update state is incomplete")
	}

	return &state, nil
}

func savePendingUpdateState(state *pendingUpdateState) error {
	if state == nil {
		return fmt.Errorf("pending update state is nil")
	}
	if err := ensureSelfUpdateStateDir(); err != nil {
		return err
	}

	statePath, err := pendingUpdateStatePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode pending update state: %w", err)
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(statePath), "pending-update-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary pending update state file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to write pending update state: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to finalize pending update state: %w", err)
	}

	if err := os.Chmod(tmpPath, 0644); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to set permissions on pending update state: %w", err)
	}

	if err := os.Rename(tmpPath, statePath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to atomically write pending update state: %w", err)
	}

	return nil
}

func clearPendingUpdateState() error {
	statePath, err := pendingUpdateStatePath()
	if err != nil {
		return err
	}
	if err := os.Remove(statePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clear pending update state: %w", err)
	}
	return nil
}

func currentUserIdentity() (string, string) {
	usr, err := user.Current()
	if err != nil {
		return "", ""
	}
	return strings.TrimSpace(usr.Uid), strings.TrimSpace(usr.Username)
}

func readUpdateLockMetadata() (*updateLockMetadata, error) {
	lockPath, err := updateLockPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read update lock metadata: %w", err)
	}

	if strings.TrimSpace(string(data)) == "" {
		return nil, nil
	}

	var metadata updateLockMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse update lock metadata: %w", err)
	}

	return &metadata, nil
}

func writeUpdateLockMetadata(lockPath string, metadata *updateLockMetadata) {
	if metadata == nil {
		return
	}
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(lockPath, data, 0644)
}

func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	if runtime.GOOS == "windows" {
		check := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid))
		out, err := check.Output()
		if err != nil {
			return false
		}
		return strings.Contains(string(out), fmt.Sprintf(" %d", pid))
	}

	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func waitForProcessExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !isProcessRunning(pid) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return !isProcessRunning(pid)
}

func terminateProcessForForceUnlock(pid int, verbose bool) error {
	if pid <= 0 {
		return fmt.Errorf("invalid lock holder pid: %d", pid)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to resolve lock holder process %d: %w", pid, err)
	}

	if !isProcessRunning(pid) {
		return nil
	}

	if verbose {
		log.Printf("self-update --force: stopping updater process pid=%d", pid)
	}

	if runtime.GOOS == "windows" {
		cmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to terminate updater process %d: %w", pid, err)
		}
		_ = waitForProcessExit(pid, 3*time.Second)
		return nil
	}

	if err := process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("failed to send SIGTERM to updater process %d: %w", pid, err)
	}
	if waitForProcessExit(pid, 2*time.Second) {
		return nil
	}

	if verbose {
		log.Printf("self-update --force: process pid=%d ignored SIGTERM; sending SIGKILL", pid)
	}
	if err := process.Signal(syscall.SIGKILL); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("failed to SIGKILL updater process %d: %w", pid, err)
	}
	_ = waitForProcessExit(pid, 2*time.Second)
	return nil
}

func acquireUpdateLock(force bool, command string, verbose bool) (func(), bool, error) {
	if err := ensureSelfUpdateStateDir(); err != nil {
		return nil, false, err
	}

	lockPath, err := updateLockPath()
	if err != nil {
		return nil, false, err
	}

	lock := flock.New(lockPath)

	tryAcquire := func() (bool, error) {
		locked, err := lock.TryLock()
		if err != nil {
			return false, fmt.Errorf("failed to acquire update lock: %w", err)
		}
		return locked, nil
	}

	locked, err := tryAcquire()
	if err != nil {
		return nil, false, err
	}

	if !locked {
		if !force {
			return nil, false, nil
		}

		metadata, metaErr := readUpdateLockMetadata()
		if metaErr != nil {
			if verbose {
				log.Printf("self-update --force: lock metadata unavailable: %v", metaErr)
			}
			return nil, false, fmt.Errorf("self-update lock is held and owner metadata is unreadable; rerun after current updater exits")
		}

		if metadata == nil || metadata.PID <= 0 {
			return nil, false, fmt.Errorf("self-update lock is held and owner PID is unavailable; rerun after current updater exits")
		}

		currentUID, _ := currentUserIdentity()
		if currentUID != "" && metadata.UID != "" && currentUID != metadata.UID {
			return nil, false, fmt.Errorf("refusing to terminate updater process pid=%d owned by another user (%s)", metadata.PID, metadata.Username)
		}

		if err := terminateProcessForForceUnlock(metadata.PID, verbose); err != nil {
			return nil, false, err
		}

		locked, err = tryAcquire()
		if err != nil {
			return nil, false, err
		}
		if !locked {
			return nil, false, fmt.Errorf("self-update lock is still held after --force recovery attempt")
		}
	}

	uid, username := currentUserIdentity()
	writeUpdateLockMetadata(lockPath, &updateLockMetadata{
		PID:       os.Getpid(),
		UID:       uid,
		Username:  username,
		Command:   command,
		Version:   version,
		StartedAt: time.Now().UTC().Format(time.RFC3339),
	})

	release := func() {
		_ = lock.Unlock()
	}

	return release, true, nil
}

func selfUpdatePlatformID() (string, error) {
	platformOS := runtime.GOOS
	switch platformOS {
	case "linux", "darwin", "windows":
	default:
		return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	platformArch := ""
	switch runtime.GOARCH {
	case "amd64":
		platformArch = "amd64"
	case "arm64":
		platformArch = "arm64"
	default:
		return "", fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}

	return fmt.Sprintf("%s-%s", platformOS, platformArch), nil
}

func b2Authorize(client *http.Client) (*b2AuthResponse, error) {
	authReq, err := http.NewRequest("GET", b2AuthURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth request: %w", err)
	}
	authReq.SetBasicAuth(b2KeyID, b2AppKey)

	authResp, err := client.Do(authReq)
	if err != nil {
		return nil, fmt.Errorf("B2 auth request failed: %w", err)
	}
	defer authResp.Body.Close()

	if authResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(authResp.Body)
		return nil, fmt.Errorf("B2 auth failed with status %d: %s", authResp.StatusCode, string(body))
	}

	var authData b2AuthResponse
	if err := json.NewDecoder(authResp.Body).Decode(&authData); err != nil {
		return nil, fmt.Errorf("failed to decode B2 auth response: %w", err)
	}

	if strings.TrimSpace(authData.AuthorizationToken) == "" || strings.TrimSpace(authData.DownloadURL) == "" {
		return nil, fmt.Errorf("B2 auth response missing required fields")
	}

	return &authData, nil
}

func downloadVersionBinaryFromB2(versionTag string) (string, error) {
	platformID, err := selfUpdatePlatformID()
	if err != nil {
		return "", err
	}

	binaryName := "lrc"
	if runtime.GOOS == "windows" {
		binaryName = "lrc.exe"
	}

	client := &http.Client{Timeout: 60 * time.Second}
	authData, err := b2Authorize(client)
	if err != nil {
		return "", err
	}

	downloadPath := fmt.Sprintf("%s/%s/%s/%s", b2Prefix, versionTag, platformID, binaryName)
	fullURL := fmt.Sprintf("%s/file/%s/%s", authData.DownloadURL, b2BucketName, downloadPath)

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create download request: %w", err)
	}
	req.Header.Set("Authorization", authData.AuthorizationToken)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download update binary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to download binary (status %d): %s", resp.StatusCode, string(body))
	}

	tmpFile, err := os.CreateTemp("", "lrc-update-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file for update: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to write downloaded update binary: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to finalize downloaded update binary: %w", err)
	}

	if err := os.Chmod(tmpPath, 0755); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to mark downloaded binary executable: %w", err)
	}

	return tmpPath, nil
}

func stageUpdateVersion(versionTag string, force bool, verbose bool) (*pendingUpdateState, error) {
	release, acquired, err := acquireUpdateLock(force, "self-update-stage", verbose)
	if err != nil {
		return nil, err
	}
	if !acquired {
		if verbose {
			log.Println("self-update lock is held by another process; skipping stage")
		}
		return nil, nil
	}
	defer release()

	if strings.TrimSpace(versionTag) == "" {
		return nil, fmt.Errorf("version tag is empty")
	}

	existing, err := loadPendingUpdateState()
	if err == nil && existing != nil && existing.Version == versionTag && !force {
		if st, statErr := os.Stat(existing.StagedBinaryPath); statErr == nil && st.Size() > 0 {
			if verbose {
				log.Printf("reusing already-downloaded update artifact for %s", versionTag)
			}
			return existing, nil
		}
	}

	stagedBinaryPath, err := downloadVersionBinaryFromB2(versionTag)
	if err != nil {
		return nil, err
	}

	state := &pendingUpdateState{
		Version:          versionTag,
		StagedBinaryPath: stagedBinaryPath,
		DownloadedAt:     time.Now().UTC().Format(time.RFC3339),
	}

	if err := savePendingUpdateState(state); err != nil {
		_ = os.Remove(stagedBinaryPath)
		return nil, err
	}

	if verbose {
		log.Printf("staged update binary for %s at %s", versionTag, stagedBinaryPath)
	}

	return state, nil
}

func currentBinaryTargets() (string, string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve current executable path: %w", err)
	}

	resolvedExe, err := filepath.EvalSymlinks(exePath)
	if err == nil && strings.TrimSpace(resolvedExe) != "" {
		exePath = resolvedExe
	}

	installDir := filepath.Dir(exePath)
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}

	return filepath.Join(installDir, "lrc"+ext), filepath.Join(installDir, "git-lrc"+ext), nil
}

func pathDirWritable(path string) bool {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".lrc-write-check-")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}

func copyFileContents(srcPath, dstPath string, mode fs.FileMode) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", srcPath, err)
	}
	defer src.Close()

	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return fmt.Errorf("failed to open destination file %s: %w", dstPath, err)
	}

	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return fmt.Errorf("failed to copy %s to %s: %w", srcPath, dstPath, err)
	}

	if err := dst.Close(); err != nil {
		return fmt.Errorf("failed to close destination file %s: %w", dstPath, err)
	}

	return nil
}

func runHooksInstallWithBinary(binaryPath string, verbose bool) error {
	cleaned := filepath.Clean(strings.TrimSpace(binaryPath))
	if cleaned == "" {
		return fmt.Errorf("hooks install binary path is empty")
	}
	base := filepath.Base(cleaned)
	if base != "lrc" && base != "lrc.exe" {
		return fmt.Errorf("invalid hooks install binary name: %s", base)
	}
	if _, err := os.Stat(cleaned); err != nil {
		return fmt.Errorf("hooks install binary not accessible: %w", err)
	}

	cmd := exec.Command(cleaned, "hooks", "install")
	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run hooks install with new binary: %w", err)
	}
	return nil
}

func applyPendingUpdateUnix(state *pendingUpdateState, verbose bool) error {
	lrcBinaryPath, gitLRCBinaryPath, err := currentBinaryTargets()
	if err != nil {
		return err
	}

	if !pathDirWritable(lrcBinaryPath) {
		return fmt.Errorf("install directory is not writable: %s", filepath.Dir(lrcBinaryPath))
	}

	replaceTmpPath := filepath.Join(filepath.Dir(lrcBinaryPath), fmt.Sprintf(".lrc.new.%d", time.Now().UnixNano()))
	if err := copyFileContents(state.StagedBinaryPath, replaceTmpPath, 0755); err != nil {
		return err
	}

	if err := os.Chmod(replaceTmpPath, 0755); err != nil {
		_ = os.Remove(replaceTmpPath)
		return fmt.Errorf("failed to set executable permissions on replacement binary: %w", err)
	}

	if err := os.Rename(replaceTmpPath, lrcBinaryPath); err != nil {
		_ = os.Remove(replaceTmpPath)
		return fmt.Errorf("failed to replace lrc binary: %w", err)
	}

	if err := copyFileContents(lrcBinaryPath, gitLRCBinaryPath, 0755); err != nil {
		return fmt.Errorf("failed to sync git-lrc binary: %w", err)
	}
	if err := os.Chmod(gitLRCBinaryPath, 0755); err != nil {
		return fmt.Errorf("failed to set executable permissions on git-lrc binary: %w", err)
	}

	if err := runHooksInstallWithBinary(lrcBinaryPath, verbose); err != nil {
		return err
	}

	_ = os.Remove(state.StagedBinaryPath)
	if err := clearPendingUpdateState(); err != nil {
		return err
	}

	fmt.Printf("%s✓ Updated to %s and reinstalled global hooks%s\n", colorGreen, state.Version, colorReset)
	return nil
}

func psSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func applyPendingUpdateWindows(state *pendingUpdateState, verbose bool) error {
	lrcBinaryPath, gitLRCBinaryPath, err := currentBinaryTargets()
	if err != nil {
		return err
	}

	statePath, err := pendingUpdateStatePath()
	if err != nil {
		return err
	}

	script := fmt.Sprintf("$src='%s';$dst='%s';$git='%s';$state='%s';for($i=0;$i -lt 120;$i++){try{Move-Item -Force $src $dst;Copy-Item -Force $dst $git;& $dst hooks install *> $null;Remove-Item -Force $state -ErrorAction SilentlyContinue;exit 0}catch{Start-Sleep -Milliseconds 500}};exit 1",
		psSingleQuote(state.StagedBinaryPath),
		psSingleQuote(lrcBinaryPath),
		psSingleQuote(gitLRCBinaryPath),
		psSingleQuote(statePath),
	)

	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	if verbose {
		log.Println("starting Windows self-update helper process")
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start Windows update helper: %w", err)
	}

	fmt.Printf("%sUpdate to %s scheduled and will finalize as this process exits.%s\n", colorYellow, state.Version, colorReset)
	return nil
}

func applyPendingUpdateState(state *pendingUpdateState, verbose bool) error {
	if state == nil {
		return nil
	}
	if st, err := os.Stat(state.StagedBinaryPath); err != nil || st.Size() == 0 {
		_ = clearPendingUpdateState()
		if err == nil {
			return fmt.Errorf("staged update binary is empty: %s", state.StagedBinaryPath)
		}
		return fmt.Errorf("staged update binary unavailable: %w", err)
	}

	if runtime.GOOS == "windows" {
		return applyPendingUpdateWindows(state, verbose)
	}
	return applyPendingUpdateUnix(state, verbose)
}

func applyPendingUpdateIfAny(verbose bool) error {
	release, acquired, err := acquireUpdateLock(false, "self-update-apply", verbose)
	if err != nil {
		return err
	}
	if !acquired {
		if verbose {
			log.Println("self-update lock is held by another process; skipping apply")
		}
		return nil
	}
	defer release()

	state, err := loadPendingUpdateState()
	if err != nil {
		return err
	}
	if state == nil {
		return nil
	}

	return applyPendingUpdateState(state, verbose)
}

func startAutoUpdateCheck(verbose bool) {
	autoUpdateStartOnce.Do(func() {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					if verbose {
						log.Printf("auto-update check panicked: %v", r)
					}
				}
			}()

			latestVersion, err := fetchLatestVersionFromB2()
			if err != nil {
				if verbose {
					log.Printf("auto-update check failed: %v", err)
				}
				return
			}

			cmp, err := semverCompare(version, latestVersion)
			if err != nil {
				if verbose {
					log.Printf("auto-update version compare failed: %v", err)
				}
				return
			}
			if cmp >= 0 {
				return
			}

			_, err = stageUpdateVersion(latestVersion, false, verbose)
			if err != nil {
				if verbose {
					log.Printf("auto-update staging failed: %v", err)
				}
				return
			}

			if verbose {
				log.Printf("auto-update staged version %s for apply-on-exit", latestVersion)
			}
		}()
	})
}

// platformInstallCommand returns the appropriate installer command for the current platform
func platformInstallCommand() string {
	if runtime.GOOS == "windows" {
		return `powershell -Command "iwr -useb https://hexmos.com/lrc-install.ps1 | iex"`
	}
	return "curl -fsSL https://hexmos.com/lrc-install.sh | bash"
}

// ANSI color codes for terminal output
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

// runSelfUpdate handles the self-update command
func runSelfUpdate(c *cli.Context) error {
	checkOnly := c.Bool("check")
	force := c.Bool("force")

	fmt.Printf("Current version: %s%s%s\n", colorCyan, version, colorReset)
	fmt.Println("Checking for updates...")

	latestVersion, err := fetchLatestVersionFromB2()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	fmt.Printf("Latest version:  %s%s%s\n", colorCyan, latestVersion, colorReset)

	cmp, err := semverCompare(version, latestVersion)
	if err != nil {
		return fmt.Errorf("failed to compare versions: %w", err)
	}
	if cmp >= 0 && !force {
		fmt.Printf("\n%s✓ lrc is already up to date!%s\n", colorGreen, colorReset)
		return nil
	}

	if cmp >= 0 && force {
		fmt.Printf("\n%sForce recovery requested (this may terminate another active lrc self-update process)%s\n", colorYellow, colorReset)
	} else {
		fmt.Printf("\n%s⬆ Update available: %s → %s%s\n", colorYellow, version, latestVersion, colorReset)
		if force {
			fmt.Printf("%sWarning: --force may terminate another active lrc self-update process.%s\n", colorYellow, colorReset)
		}
	}

	if checkOnly {
		fmt.Println("\nRun 'lrc self-update' (without --check) to install the update.")
		return nil
	}

	fmt.Println("Downloading update artifact...")
	state, err := stageUpdateVersion(latestVersion, force, true)
	if err != nil {
		return fmt.Errorf("failed to stage update: %w", err)
	}
	if state == nil {
		fmt.Printf("%sAnother lrc self-update process is active. Re-run with --force to recover.%s\n", colorYellow, colorReset)
		return nil
	}

	fmt.Println("Applying update...")
	if err := applyPendingUpdateState(state, true); err != nil {
		return fmt.Errorf("failed to apply update: %w", err)
	}

	fmt.Printf("\n%s✓ Update complete! Run 'lrc version' to verify.%s\n", colorGreen, colorReset)
	return nil
}
