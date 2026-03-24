package simulator

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/HexmosTech/git-lrc/storage"
)

const (
	simCommitMessageFile = "livereview_commit_message"
	simPushRequestFile   = "livereview_push_request"
)

func applyPrecommitArtifacts(gitDir string, result *Result) error {
	if gitDir == "" || result == nil {
		return nil
	}

	result.CommitMessagePath = filepath.Join(gitDir, simCommitMessageFile)
	result.PushMarkerPath = filepath.Join(gitDir, simPushRequestFile)

	if result.CommitMessageOverride != "" {
		msg := []byte(result.CommitMessageOverride + "\n")
		if err := storage.WriteFile(result.CommitMessagePath, msg, 0600); err != nil {
			return fmt.Errorf("write commit message override: %w", err)
		}
	} else {
		if err := storage.RemoveCommitMessageOverrideFile(result.CommitMessagePath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}

	if result.PushMarkerPersisted {
		if err := storage.WriteFile(result.PushMarkerPath, []byte("push"), 0600); err != nil {
			return fmt.Errorf("write push marker: %w", err)
		}
	} else {
		if err := storage.RemoveCommitPushRequestFile(result.PushMarkerPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}

	return nil
}
