package main

import (
	"bytes"
	"net/http"
	"testing"
)

func TestActionAllowedInPhase(t *testing.T) {
	tests := []struct {
		name  string
		code  int
		phase interactivePhase
		want  bool
	}{
		{name: "abort allowed while reviewing", code: decisionAbort, phase: phaseReviewRunning, want: true},
		{name: "abort allowed after review", code: decisionAbort, phase: phaseReviewComplete, want: true},
		{name: "skip allowed while reviewing", code: decisionSkip, phase: phaseReviewRunning, want: true},
		{name: "skip blocked after review", code: decisionSkip, phase: phaseReviewComplete, want: false},
		{name: "vouch allowed while reviewing", code: decisionVouch, phase: phaseReviewRunning, want: true},
		{name: "vouch blocked after review", code: decisionVouch, phase: phaseReviewComplete, want: false},
		{name: "commit blocked while reviewing", code: decisionCommit, phase: phaseReviewRunning, want: false},
		{name: "commit allowed after review", code: decisionCommit, phase: phaseReviewComplete, want: true},
		{name: "unknown action blocked", code: 999, phase: phaseReviewComplete, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := actionAllowedInPhase(tt.code, tt.phase)
			if got != tt.want {
				t.Fatalf("actionAllowedInPhase(%d, %v) = %v, want %v", tt.code, tt.phase, got, tt.want)
			}
		})
	}
}

func TestValidateInteractiveDecisionRequest(t *testing.T) {
	tests := []struct {
		name       string
		code       int
		message    string
		phase      interactivePhase
		wantStatus int
		wantErr    bool
	}{
		{name: "commit with message after review", code: decisionCommit, message: "feat: ok", phase: phaseReviewComplete, wantErr: false},
		{name: "commit empty message rejected", code: decisionCommit, message: "   ", phase: phaseReviewComplete, wantErr: true, wantStatus: http.StatusBadRequest},
		{name: "commit while reviewing rejected", code: decisionCommit, message: "feat: no", phase: phaseReviewRunning, wantErr: true, wantStatus: http.StatusConflict},
		{name: "skip while reviewing allowed", code: decisionSkip, message: "", phase: phaseReviewRunning, wantErr: false},
		{name: "skip after review rejected", code: decisionSkip, message: "", phase: phaseReviewComplete, wantErr: true, wantStatus: http.StatusConflict},
		{name: "abort while reviewing allowed", code: decisionAbort, message: "", phase: phaseReviewRunning, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInteractiveDecisionRequest(tt.code, tt.message, tt.phase)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantErr {
				reqErr, ok := err.(*decisionRequestError)
				if !ok {
					t.Fatalf("expected *decisionRequestError, got %T", err)
				}
				if reqErr.status != tt.wantStatus {
					t.Fatalf("status = %d, want %d", reqErr.status, tt.wantStatus)
				}
			}
		})
	}
}

func TestReadCommitMessageFromRequest(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "empty body", body: "", want: ""},
		{name: "invalid json", body: "{not-json}", want: ""},
		{name: "trims trailing newline", body: `{"message":"hello\n"}`, want: "hello"},
		{name: "keeps internal newlines", body: `{"message":"hello\nworld"}`, want: "hello\nworld"},
		{name: "strips control chars", body: `{"message":"hi\u0001there"}`, want: "hithere"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, "/commit", bytes.NewBufferString(tt.body))
			if err != nil {
				t.Fatalf("NewRequest failed: %v", err)
			}
			got := readCommitMessageFromRequest(req)
			if got != tt.want {
				t.Fatalf("readCommitMessageFromRequest() = %q, want %q", got, tt.want)
			}
		})
	}
}
