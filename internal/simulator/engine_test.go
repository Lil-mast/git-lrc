package simulator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/HexmosTech/git-lrc/internal/decisionflow"
)

func TestEngineRejectsRealNetworkBackend(t *testing.T) {
	engine := Engine{Backend: RealNetworkGuard{}}
	_, err := engine.Run(context.Background(), Scenario{Name: "guard", Trigger: TriggerLRC})
	if !errors.Is(err, ErrRealNetworkNotAllowed) {
		t.Fatalf("expected real network guard error, got %v", err)
	}
}

func TestComprehensiveCases(t *testing.T) {
	engine := Engine{}
	for _, tc := range ComprehensiveCases() {
		t.Run(fmt.Sprintf("%s_%s", tc.ID, tc.Description), func(t *testing.T) {
			scenario := tc.Scenario
			if scenario.Precommit {
				gitDir := t.TempDir()
				scenario.GitDir = gitDir
			}
			result, err := engine.Run(context.Background(), scenario)
			if err != nil {
				t.Fatalf("unexpected run error: %v", err)
			}

			assertExpected(t, result, tc.Expect)
			assertPrecommitArtifacts(t, scenario, result, tc.Expect)
		})
	}
}

func TestMessageCases(t *testing.T) {
	engine := Engine{}
	for _, tc := range MessageCases() {
		t.Run(fmt.Sprintf("%s_%s", tc.ID, tc.Description), func(t *testing.T) {
			scenario := tc.Scenario
			if scenario.Precommit {
				scenario.GitDir = t.TempDir()
			}
			result, err := engine.Run(context.Background(), scenario)
			if err != nil {
				t.Fatalf("unexpected run error: %v", err)
			}

			assertExpected(t, result, tc.Expect)
			assertPrecommitArtifacts(t, scenario, result, tc.Expect)
		})
	}
}

func TestTimelineContainsExpectedSignals(t *testing.T) {
	engine := Engine{}
	result, err := engine.Run(context.Background(), Scenario{
		Name:       "timeline",
		Trigger:    TriggerLRC,
		StartPhase: decisionflow.PhaseReviewRunning,
		Events: []Event{
			DecisionEvent{Source: DecisionSourceWeb, Code: decisionflow.DecisionCommit, Message: "bad"},
			PollCompletedEvent{},
			DecisionEvent{Source: DecisionSourceTerminal, Code: decisionflow.DecisionCommit},
		},
	})
	if err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}

	hasRejected := false
	hasPhaseComplete := false
	hasAccepted := false
	for _, rec := range result.Timeline {
		switch rec.Type {
		case "decision-rejected":
			hasRejected = true
		case "phase":
			if rec.Details["value"] == "complete" {
				hasPhaseComplete = true
			}
		case "decision-accepted":
			hasAccepted = true
		}
	}

	if !hasRejected || !hasPhaseComplete || !hasAccepted {
		t.Fatalf("timeline missing expected events: rejected=%v phaseComplete=%v accepted=%v", hasRejected, hasPhaseComplete, hasAccepted)
	}
}

func assertExpected(t *testing.T, got Result, want CaseExpectation) {
	t.Helper()
	if got.DecisionCode != want.DecisionCode {
		t.Fatalf("decision=%d want=%d", got.DecisionCode, want.DecisionCode)
	}
	if got.ExitCode != want.ExitCode {
		t.Fatalf("exitCode=%d want=%d", got.ExitCode, want.ExitCode)
	}
	if got.Committed != want.Committed {
		t.Fatalf("committed=%v want=%v", got.Committed, want.Committed)
	}
	if got.Aborted != want.Aborted {
		t.Fatalf("aborted=%v want=%v", got.Aborted, want.Aborted)
	}
	if got.Skipped != want.Skipped {
		t.Fatalf("skipped=%v want=%v", got.Skipped, want.Skipped)
	}
	if got.Vouched != want.Vouched {
		t.Fatalf("vouched=%v want=%v", got.Vouched, want.Vouched)
	}
	if got.FinalMessage != want.FinalMessage {
		t.Fatalf("finalMessage=%q want=%q", got.FinalMessage, want.FinalMessage)
	}
	if got.PushRequested != want.PushRequested {
		t.Fatalf("pushRequested=%v want=%v", got.PushRequested, want.PushRequested)
	}
	if got.PushMarkerPersisted != want.PushMarkerPersisted {
		t.Fatalf("pushMarkerPersisted=%v want=%v", got.PushMarkerPersisted, want.PushMarkerPersisted)
	}
	if got.CommitMessageOverride != want.CommitMessageOverride {
		t.Fatalf("commitMessageOverride=%q want=%q", got.CommitMessageOverride, want.CommitMessageOverride)
	}
}

func assertPrecommitArtifacts(t *testing.T, scenario Scenario, got Result, want CaseExpectation) {
	t.Helper()
	if !scenario.Precommit {
		return
	}

	if scenario.GitDir == "" {
		t.Fatalf("precommit scenario missing GitDir")
	}

	commitPath := filepath.Join(scenario.GitDir, simCommitMessageFile)
	pushPath := filepath.Join(scenario.GitDir, simPushRequestFile)

	if got.CommitMessagePath != commitPath {
		t.Fatalf("commit message path=%q want=%q", got.CommitMessagePath, commitPath)
	}
	if got.PushMarkerPath != pushPath {
		t.Fatalf("push marker path=%q want=%q", got.PushMarkerPath, pushPath)
	}

	if want.CommitMessageOverride != "" {
		b, err := os.ReadFile(commitPath)
		if err != nil {
			t.Fatalf("expected commit message override file, read error: %v", err)
		}
		if string(b) != want.CommitMessageOverride+"\n" {
			t.Fatalf("commit message file=%q want=%q", string(b), want.CommitMessageOverride+"\n")
		}
	} else {
		if _, err := os.Stat(commitPath); err == nil {
			t.Fatalf("expected commit message override file to be absent")
		}
	}

	if want.PushMarkerPersisted {
		b, err := os.ReadFile(pushPath)
		if err != nil {
			t.Fatalf("expected push marker file, read error: %v", err)
		}
		if string(b) != "push" {
			t.Fatalf("push marker file=%q want=%q", string(b), "push")
		}
	} else {
		if _, err := os.Stat(pushPath); err == nil {
			t.Fatalf("expected push marker file to be absent")
		}
	}
}
