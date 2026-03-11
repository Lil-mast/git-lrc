package appcore

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/HexmosTech/git-lrc/internal/decisionflow"
)

func TestActionAllowedInPhase(t *testing.T) {
	tests := []struct {
		name  string
		code  int
		phase decisionflow.Phase
		want  bool
	}{
		{name: "abort allowed while reviewing", code: decisionflow.DecisionAbort, phase: decisionflow.PhaseReviewRunning, want: true},
		{name: "abort allowed after review", code: decisionflow.DecisionAbort, phase: decisionflow.PhaseReviewComplete, want: true},
		{name: "skip allowed while reviewing", code: decisionflow.DecisionSkip, phase: decisionflow.PhaseReviewRunning, want: true},
		{name: "skip blocked after review", code: decisionflow.DecisionSkip, phase: decisionflow.PhaseReviewComplete, want: false},
		{name: "vouch allowed while reviewing", code: decisionflow.DecisionVouch, phase: decisionflow.PhaseReviewRunning, want: true},
		{name: "vouch blocked after review", code: decisionflow.DecisionVouch, phase: decisionflow.PhaseReviewComplete, want: false},
		{name: "commit blocked while reviewing", code: decisionflow.DecisionCommit, phase: decisionflow.PhaseReviewRunning, want: false},
		{name: "commit allowed after review", code: decisionflow.DecisionCommit, phase: decisionflow.PhaseReviewComplete, want: true},
		{name: "unknown action blocked", code: 999, phase: decisionflow.PhaseReviewComplete, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decisionflow.ActionAllowedInPhase(tt.code, tt.phase)
			if got != tt.want {
				t.Fatalf("ActionAllowedInPhase(%d, %v) = %v, want %v", tt.code, tt.phase, got, tt.want)
			}
		})
	}
}

func TestValidateInteractiveDecisionRequest(t *testing.T) {
	tests := []struct {
		name       string
		code       int
		message    string
		phase      decisionflow.Phase
		wantStatus int
		wantErr    bool
	}{
		{name: "commit with message after review", code: decisionflow.DecisionCommit, message: "feat: ok", phase: decisionflow.PhaseReviewComplete, wantErr: false},
		{name: "commit empty message rejected", code: decisionflow.DecisionCommit, message: "   ", phase: decisionflow.PhaseReviewComplete, wantErr: true, wantStatus: http.StatusBadRequest},
		{name: "commit while reviewing rejected", code: decisionflow.DecisionCommit, message: "feat: no", phase: decisionflow.PhaseReviewRunning, wantErr: true, wantStatus: http.StatusConflict},
		{name: "skip while reviewing allowed", code: decisionflow.DecisionSkip, message: "", phase: decisionflow.PhaseReviewRunning, wantErr: false},
		{name: "skip after review rejected", code: decisionflow.DecisionSkip, message: "", phase: decisionflow.PhaseReviewComplete, wantErr: true, wantStatus: http.StatusConflict},
		{name: "abort while reviewing allowed", code: decisionflow.DecisionAbort, message: "", phase: decisionflow.PhaseReviewRunning, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := decisionflow.ValidateRequest(tt.code, tt.message, tt.phase)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantErr {
				reqErr, ok := err.(*decisionflow.RequestError)
				if !ok {
					t.Fatalf("expected *decisionflow.RequestError, got %T", err)
				}
				if reqErr.StatusCode() != tt.wantStatus {
					t.Fatalf("status = %d, want %d", reqErr.StatusCode(), tt.wantStatus)
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
