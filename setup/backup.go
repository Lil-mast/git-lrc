package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// BackupExistingConfig backs up ~/.lrc.toml if it exists and is non-empty.
// Returns backupPath when a backup is written, or an empty path when skipped.
func BackupExistingConfig(logf func(format string, args ...interface{})) (string, error) {
	log := func(format string, args ...interface{}) {
		if logf != nil {
			logf(format, args...)
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log("cannot determine home directory: %v", err)
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".lrc.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			log("no existing config found")
			return "", nil
		}
		log("failed to read existing config: %v", err)
		return "", fmt.Errorf("failed to read existing config: %w", err)
	}

	if strings.TrimSpace(string(data)) == "" {
		log("existing config is empty; skipping backup")
		return "", nil
	}

	backupPath := configPath + ".bak." + time.Now().Format("20060102-150405")
	if err := WriteFileAtomically(backupPath, data, 0600); err != nil {
		return "", fmt.Errorf("failed to backup existing config: %w", err)
	}

	log("backed up existing config to %s", backupPath)
	return backupPath, nil
}
