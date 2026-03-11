package appui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileAtomicallyReplacesExistingContent(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, ".lrc.toml")

	if err := os.WriteFile(targetPath, []byte("api_key = \"old\"\n"), 0600); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	newContent := []byte("api_key = \"new\"\norg_id = \"o1\"\n")
	if err := writeFileAtomically(targetPath, newContent, 0600); err != nil {
		t.Fatalf("write atomically: %v", err)
	}

	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(got) != string(newContent) {
		t.Fatalf("unexpected content: %q", string(got))
	}
}

func TestBackupExistingConfigBacksUpNonEmptyConfig(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)

	configPath := filepath.Join(tmpHome, ".lrc.toml")
	configBody := "jwt = \"stale\"\norg_id = \"org-1\"\n"
	if err := os.WriteFile(configPath, []byte(configBody), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	slog := newSetupLog()
	if err := backupExistingConfig(slog); err != nil {
		t.Fatalf("backup existing config: %v", err)
	}

	matches, err := filepath.Glob(configPath + ".bak.*")
	if err != nil {
		t.Fatalf("glob backup files: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected exactly one backup file, got %d", len(matches))
	}

	backupBody, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read backup file: %v", err)
	}
	if string(backupBody) != configBody {
		t.Fatalf("backup mismatch: got %q", string(backupBody))
	}
}

func TestWriteConfigIncludesSessionFields(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)

	result := &setupResult{
		PlainAPIKey:  "lr_key_123",
		Email:        "user@example.com",
		FirstName:    "Jane",
		LastName:     "Doe",
		AvatarURL:    "https://cdn.hexmos.com/u/jane.png",
		UserID:       "u-1",
		OrgID:        "o-1",
		OrgName:      "Acme Org",
		AccessToken:  "jwt-1",
		RefreshToken: "ref-1",
	}

	if err := writeConfig(result); err != nil {
		t.Fatalf("write config: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpHome, ".lrc.toml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	content := string(data)

	for _, expected := range []string{
		`api_key = "lr_key_123"`,
		`user_email = "user@example.com"`,
		`user_first_name = "Jane"`,
		`user_last_name = "Doe"`,
		`avatar_url = "https://cdn.hexmos.com/u/jane.png"`,
		`org_id = "o-1"`,
		`org_name = "Acme Org"`,
		`jwt = "jwt-1"`,
		`refresh_token = "ref-1"`,
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("config missing %s", expected)
		}
	}
}
