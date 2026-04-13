package appcore

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HexmosTech/git-lrc/internal/reviewmodel"
)

func TestIsLiveReviewAPIKeyInvalid(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "valid unauthorized code",
			err: &reviewmodel.APIError{
				StatusCode: http.StatusUnauthorized,
				Body:       `{"error_code":"LIVE_REVIEW_API_KEY_INVALID","error":"invalid"}`,
			},
			want: true,
		},
		{
			name: "unauthorized but different error code",
			err: &reviewmodel.APIError{
				StatusCode: http.StatusUnauthorized,
				Body:       `{"error_code":"SOMETHING_ELSE","error":"nope"}`,
			},
			want: false,
		},
		{
			name: "non-401 should not trigger recovery",
			err: &reviewmodel.APIError{
				StatusCode: http.StatusTooManyRequests,
				Body:       `{"error_code":"LIVE_REVIEW_API_KEY_INVALID"}`,
			},
			want: false,
		},
		{
			name: "malformed json should not trigger recovery",
			err: &reviewmodel.APIError{
				StatusCode: http.StatusUnauthorized,
				Body:       `not-json`,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isLiveReviewAPIKeyInvalid(tt.err)
			if got != tt.want {
				t.Fatalf("isLiveReviewAPIKeyInvalid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseAPIErrorCode(t *testing.T) {
	code, err := parseAPIErrorCode(`{"error_code":"LIVE_REVIEW_API_KEY_INVALID"}`)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if code != "LIVE_REVIEW_API_KEY_INVALID" {
		t.Fatalf("unexpected code: %s", code)
	}

	if _, err := parseAPIErrorCode(`{not-json}`); err == nil {
		t.Fatal("expected parse error for malformed json")
	}
}

func TestPersistConfigUpdatesPreservesExistingAPIURL(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".lrc.toml")
	original := "api_url = \"http://localhost:8888\"\njwt = \"old\"\n"
	if err := os.WriteFile(configPath, []byte(original), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := persistConfigUpdates(configPath, map[string]string{"jwt": "new"}); err != nil {
		t.Fatalf("persist config updates: %v", err)
	}

	updatedBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	updated := string(updatedBytes)

	if !strings.Contains(updated, `api_url = "http://localhost:8888"`) {
		t.Fatalf("expected api_url to remain unchanged: %s", updated)
	}
	if !strings.Contains(updated, `jwt = "new"`) {
		t.Fatalf("expected jwt to be updated: %s", updated)
	}
}

func TestPersistConfigUpdatesRejectsAPIURLMutation(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".lrc.toml")
	if err := os.WriteFile(configPath, []byte("jwt = \"old\"\n"), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	err := persistConfigUpdates(configPath, map[string]string{"api_url": "https://livereview.hexmos.com"})
	if err == nil {
		t.Fatal("expected api_url mutation guard to fail")
	}
	if !strings.Contains(err.Error(), "api_url updates are not allowed") {
		t.Fatalf("unexpected error: %v", err)
	}
}
