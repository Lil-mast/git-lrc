package simulator

import (
	"context"
	"testing"

	"github.com/HexmosTech/git-lrc/internal/decisionflow"
)

func TestHandleWebDecisionMappings(t *testing.T) {
	tests := []struct {
		name       string
		phase      decisionflow.Phase
		method     string
		path       string
		body       []byte
		wantStatus int
		wantCode   int
		wantPush   bool
		wantMsg    string
	}{
		{name: "commit", phase: decisionflow.PhaseReviewComplete, method: "POST", path: "/commit", body: []byte(`{"message":"feat: web"}`), wantStatus: HTTPStatusOK, wantCode: decisionflow.DecisionCommit, wantMsg: "feat: web"},
		{name: "commit-push", phase: decisionflow.PhaseReviewComplete, method: "POST", path: "/commit-push", body: []byte(`{"message":"feat: web"}`), wantStatus: HTTPStatusOK, wantCode: decisionflow.DecisionCommit, wantPush: true, wantMsg: "feat: web"},
		{name: "skip", phase: decisionflow.PhaseReviewRunning, method: "POST", path: "/skip", wantStatus: HTTPStatusOK, wantCode: decisionflow.DecisionSkip},
		{name: "vouch", phase: decisionflow.PhaseReviewRunning, method: "POST", path: "/vouch", wantStatus: HTTPStatusOK, wantCode: decisionflow.DecisionVouch},
		{name: "abort", phase: decisionflow.PhaseReviewRunning, method: "POST", path: "/abort", wantStatus: HTTPStatusOK, wantCode: decisionflow.DecisionAbort},
		{name: "method not allowed", phase: decisionflow.PhaseReviewComplete, method: "GET", path: "/commit", wantStatus: HTTPStatusMethodNotAllowed},
		{name: "unknown path", phase: decisionflow.PhaseReviewComplete, method: "POST", path: "/unknown", wantStatus: HTTPStatusNotFound},
		{name: "invalid phase commit while running", phase: decisionflow.PhaseReviewRunning, method: "POST", path: "/commit", body: []byte(`{"message":"feat: web"}`), wantStatus: HTTPStatusConflict},
		{name: "invalid empty web commit", phase: decisionflow.PhaseReviewComplete, method: "POST", path: "/commit", body: []byte(`{"message":"  "}`), wantStatus: HTTPStatusBadRequest},
		{name: "sanitized message", phase: decisionflow.PhaseReviewComplete, method: "POST", path: "/commit", body: []byte("{\"message\":\"fix:\\u0001ok\\n\"}"), wantStatus: HTTPStatusOK, wantCode: decisionflow.DecisionCommit, wantMsg: "fix:ok"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ev, status := HandleWebDecision(tc.phase, tc.method, tc.path, tc.body)
			if status != tc.wantStatus {
				t.Fatalf("status=%d want=%d", status, tc.wantStatus)
			}
			if status != HTTPStatusOK {
				return
			}
			if ev.Source != DecisionSourceWeb {
				t.Fatalf("source=%q want=%q", ev.Source, DecisionSourceWeb)
			}
			if ev.Code != tc.wantCode {
				t.Fatalf("code=%d want=%d", ev.Code, tc.wantCode)
			}
			if ev.Push != tc.wantPush {
				t.Fatalf("push=%v want=%v", ev.Push, tc.wantPush)
			}
			if ev.Message != tc.wantMsg {
				t.Fatalf("message=%q want=%q", ev.Message, tc.wantMsg)
			}
		})
	}
}

func TestEngineWithHTTPDecisionAutomation(t *testing.T) {
	webDecision, status := HandleWebDecision(
		decisionflow.PhaseReviewComplete,
		"POST",
		"/commit",
		[]byte(`{"message":"feat: from-http"}`),
	)
	if status != HTTPStatusOK {
		t.Fatalf("status=%d want=%d", status, HTTPStatusOK)
	}

	engine := Engine{}
	result, err := engine.Run(context.Background(), Scenario{
		Name:       "http-driven-automation",
		Trigger:    TriggerLRC,
		StartPhase: decisionflow.PhaseReviewComplete,
		Events: []Event{
			webDecision,
			DecisionEvent{Source: DecisionSourceTerminal, Code: decisionflow.DecisionAbort},
		},
	})
	if err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}

	if result.DecisionCode != decisionflow.DecisionCommit {
		t.Fatalf("decision=%d want=%d", result.DecisionCode, decisionflow.DecisionCommit)
	}
	if !result.Committed || result.Aborted {
		t.Fatalf("expected committed=true aborted=false got committed=%v aborted=%v", result.Committed, result.Aborted)
	}
	if result.FinalMessage != "feat: from-http" {
		t.Fatalf("finalMessage=%q want=%q", result.FinalMessage, "feat: from-http")
	}
}
