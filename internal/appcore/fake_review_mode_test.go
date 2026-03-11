package appcore

import (
	"os"
	"testing"
	"time"

	"github.com/HexmosTech/git-lrc/internal/reviewapi"
)

func TestIsFakeReviewBuild(t *testing.T) {
	oldMode := reviewMode
	defer func() { reviewMode = oldMode }()

	reviewMode = "fake"
	if !isFakeReviewBuild() {
		t.Fatalf("expected fake mode to be enabled")
	}

	reviewMode = "prod"
	if isFakeReviewBuild() {
		t.Fatalf("expected fake mode to be disabled")
	}
}

func TestFakeReviewWaitDuration(t *testing.T) {
	const envKey = "LRC_FAKE_REVIEW_WAIT"
	old := os.Getenv(envKey)
	defer func() {
		if old == "" {
			_ = os.Unsetenv(envKey)
			return
		}
		_ = os.Setenv(envKey, old)
	}()

	_ = os.Unsetenv(envKey)
	d, err := fakeReviewWaitDuration()
	if err != nil {
		t.Fatalf("unexpected error for default wait: %v", err)
	}
	if d != 30*time.Second {
		t.Fatalf("default wait = %s, want %s", d, 30*time.Second)
	}

	if err := os.Setenv(envKey, "3s"); err != nil {
		t.Fatalf("setenv failed: %v", err)
	}
	d, err = fakeReviewWaitDuration()
	if err != nil {
		t.Fatalf("unexpected error for valid wait: %v", err)
	}
	if d != 3*time.Second {
		t.Fatalf("wait = %s, want %s", d, 3*time.Second)
	}

	if err := os.Setenv(envKey, "not-a-duration"); err != nil {
		t.Fatalf("setenv failed: %v", err)
	}
	if _, err := fakeReviewWaitDuration(); err == nil {
		t.Fatalf("expected error for invalid duration")
	}

	if err := os.Setenv(envKey, "0s"); err != nil {
		t.Fatalf("setenv failed: %v", err)
	}
	if _, err := fakeReviewWaitDuration(); err == nil {
		t.Fatalf("expected error for zero duration")
	}
}

func TestBuildFakeCompletedResult(t *testing.T) {
	result := buildFakeCompletedResult()
	if result == nil {
		t.Fatalf("expected fake result")
	}
	if result.Status != "completed" {
		t.Fatalf("status = %q, want completed", result.Status)
	}
	if result.Summary == "" {
		t.Fatalf("expected non-empty fake summary")
	}
	if len(result.Files) != 0 {
		t.Fatalf("expected zero files in fake result, got %d", len(result.Files))
	}
}

func TestPollReviewFakeCompletes(t *testing.T) {
	result, err := pollReviewFake("fake-test", 2*time.Millisecond, 1*time.Millisecond, false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatalf("expected fake poll result")
	}
	if result.Status != "completed" {
		t.Fatalf("status = %q, want completed", result.Status)
	}
	if result.Summary == "" {
		t.Fatalf("expected non-empty summary")
	}
}

func TestPollReviewFakeCancelled(t *testing.T) {
	cancel := make(chan struct{})
	close(cancel)

	_, err := pollReviewFake("fake-test", 10*time.Millisecond, 1*time.Second, false, cancel)
	if err == nil {
		t.Fatalf("expected cancellation error")
	}
	if err != reviewapi.ErrPollCancelled {
		t.Fatalf("error = %v, want %v", err, reviewapi.ErrPollCancelled)
	}
}
