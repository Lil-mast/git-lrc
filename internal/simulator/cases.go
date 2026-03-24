package simulator

import "github.com/HexmosTech/git-lrc/internal/decisionflow"

type CaseExpectation struct {
	DecisionCode          int
	ExitCode              int
	Committed             bool
	Aborted               bool
	Skipped               bool
	Vouched               bool
	FinalMessage          string
	PushRequested         bool
	PushMarkerPersisted   bool
	CommitMessageOverride string
}

type SimCase struct {
	ID          string
	Description string
	Scenario    Scenario
	Expect      CaseExpectation
}

func ComprehensiveCases() []SimCase {
	return []SimCase{
		{
			ID:          "C01",
			Description: "git commit -m commit in precommit",
			Scenario:    Scenario{Name: "C01", Trigger: TriggerGitCommitWithMessage, Precommit: true, InitialMessage: "feat: cli", StartPhase: decisionflow.PhaseReviewComplete, Events: []Event{DecisionEvent{Source: DecisionSourceTerminal, Code: decisionflow.DecisionCommit}}},
			Expect:      CaseExpectation{DecisionCode: decisionflow.DecisionCommit, ExitCode: decisionflow.DecisionCommit, Committed: true, FinalMessage: "feat: cli", CommitMessageOverride: "feat: cli"},
		},
		{
			ID:          "C02",
			Description: "git commit -m skip",
			Scenario:    Scenario{Name: "C02", Trigger: TriggerGitCommitWithMessage, Precommit: true, InitialMessage: "feat: cli", StartPhase: decisionflow.PhaseReviewRunning, Events: []Event{DecisionEvent{Source: DecisionSourceTerminal, Code: decisionflow.DecisionSkip}}},
			Expect:      CaseExpectation{DecisionCode: decisionflow.DecisionSkip, ExitCode: decisionflow.DecisionSkip, Skipped: true},
		},
		{
			ID:          "C03",
			Description: "git commit -m vouch",
			Scenario:    Scenario{Name: "C03", Trigger: TriggerGitCommitWithMessage, Precommit: true, InitialMessage: "feat: cli", StartPhase: decisionflow.PhaseReviewRunning, Events: []Event{DecisionEvent{Source: DecisionSourceTerminal, Code: decisionflow.DecisionVouch}}},
			Expect:      CaseExpectation{DecisionCode: decisionflow.DecisionVouch, ExitCode: decisionflow.DecisionSkip, Vouched: true},
		},
		{
			ID:          "C04",
			Description: "git commit -m abort",
			Scenario:    Scenario{Name: "C04", Trigger: TriggerGitCommitWithMessage, Precommit: true, InitialMessage: "feat: cli", StartPhase: decisionflow.PhaseReviewRunning, Events: []Event{DecisionEvent{Source: DecisionSourceSignal, Code: decisionflow.DecisionAbort}}},
			Expect:      CaseExpectation{DecisionCode: decisionflow.DecisionAbort, ExitCode: decisionflow.DecisionAbort, Aborted: true},
		},
		{
			ID:          "C05",
			Description: "git commit editor-later commit",
			Scenario:    Scenario{Name: "C05", Trigger: TriggerGitCommitEditor, Precommit: true, EditorMessage: "feat: editor", StartPhase: decisionflow.PhaseReviewComplete, Events: []Event{DecisionEvent{Source: DecisionSourceTerminal, Code: decisionflow.DecisionCommit}}},
			Expect:      CaseExpectation{DecisionCode: decisionflow.DecisionCommit, ExitCode: decisionflow.DecisionCommit, Committed: true, FinalMessage: "feat: editor", CommitMessageOverride: "feat: editor"},
		},
		{
			ID:          "C06",
			Description: "git commit editor-later skip",
			Scenario:    Scenario{Name: "C06", Trigger: TriggerGitCommitEditor, Precommit: true, EditorMessage: "feat: editor", StartPhase: decisionflow.PhaseReviewRunning, Events: []Event{DecisionEvent{Source: DecisionSourceTerminal, Code: decisionflow.DecisionSkip}}},
			Expect:      CaseExpectation{DecisionCode: decisionflow.DecisionSkip, ExitCode: decisionflow.DecisionSkip, Skipped: true},
		},
		{
			ID:          "C07",
			Description: "git commit editor-later vouch",
			Scenario:    Scenario{Name: "C07", Trigger: TriggerGitCommitEditor, Precommit: true, EditorMessage: "feat: editor", StartPhase: decisionflow.PhaseReviewRunning, Events: []Event{DecisionEvent{Source: DecisionSourceTerminal, Code: decisionflow.DecisionVouch}}},
			Expect:      CaseExpectation{DecisionCode: decisionflow.DecisionVouch, ExitCode: decisionflow.DecisionSkip, Vouched: true},
		},
		{
			ID:          "C08",
			Description: "git commit editor-later abort",
			Scenario:    Scenario{Name: "C08", Trigger: TriggerGitCommitEditor, Precommit: true, EditorMessage: "feat: editor", StartPhase: decisionflow.PhaseReviewRunning, Events: []Event{DecisionEvent{Source: DecisionSourceSignal, Code: decisionflow.DecisionAbort}}},
			Expect:      CaseExpectation{DecisionCode: decisionflow.DecisionAbort, ExitCode: decisionflow.DecisionAbort, Aborted: true},
		},
		{
			ID:          "C09",
			Description: "git lrc skip",
			Scenario:    Scenario{Name: "C09", Trigger: TriggerGitLRC, StartPhase: decisionflow.PhaseReviewRunning, Events: []Event{DecisionEvent{Source: DecisionSourceTerminal, Code: decisionflow.DecisionSkip}}},
			Expect:      CaseExpectation{DecisionCode: decisionflow.DecisionSkip, ExitCode: 0, Skipped: true},
		},
		{
			ID:          "C10",
			Description: "git lrc vouch",
			Scenario:    Scenario{Name: "C10", Trigger: TriggerGitLRC, StartPhase: decisionflow.PhaseReviewRunning, Events: []Event{DecisionEvent{Source: DecisionSourceTerminal, Code: decisionflow.DecisionVouch}}},
			Expect:      CaseExpectation{DecisionCode: decisionflow.DecisionVouch, ExitCode: 0, Vouched: true},
		},
		{
			ID:          "C11",
			Description: "git lrc abort",
			Scenario:    Scenario{Name: "C11", Trigger: TriggerGitLRC, StartPhase: decisionflow.PhaseReviewRunning, Events: []Event{DecisionEvent{Source: DecisionSourceSignal, Code: decisionflow.DecisionAbort}}},
			Expect:      CaseExpectation{DecisionCode: decisionflow.DecisionAbort, ExitCode: decisionflow.DecisionAbort, Aborted: true},
		},
		{
			ID:          "C12",
			Description: "git lrc commit",
			Scenario:    Scenario{Name: "C12", Trigger: TriggerGitLRC, InitialMessage: "feat: cli", StartPhase: decisionflow.PhaseReviewComplete, Events: []Event{DecisionEvent{Source: DecisionSourceTerminal, Code: decisionflow.DecisionCommit}}},
			Expect:      CaseExpectation{DecisionCode: decisionflow.DecisionCommit, ExitCode: 0, Committed: true, FinalMessage: "feat: cli"},
		},
		{
			ID:          "C13",
			Description: "lrc skip",
			Scenario:    Scenario{Name: "C13", Trigger: TriggerLRC, StartPhase: decisionflow.PhaseReviewRunning, Events: []Event{DecisionEvent{Source: DecisionSourceTerminal, Code: decisionflow.DecisionSkip}}},
			Expect:      CaseExpectation{DecisionCode: decisionflow.DecisionSkip, ExitCode: 0, Skipped: true},
		},
		{
			ID:          "C14",
			Description: "lrc vouch",
			Scenario:    Scenario{Name: "C14", Trigger: TriggerLRC, StartPhase: decisionflow.PhaseReviewRunning, Events: []Event{DecisionEvent{Source: DecisionSourceTerminal, Code: decisionflow.DecisionVouch}}},
			Expect:      CaseExpectation{DecisionCode: decisionflow.DecisionVouch, ExitCode: 0, Vouched: true},
		},
		{
			ID:          "C15",
			Description: "lrc abort",
			Scenario:    Scenario{Name: "C15", Trigger: TriggerLRC, StartPhase: decisionflow.PhaseReviewRunning, Events: []Event{DecisionEvent{Source: DecisionSourceSignal, Code: decisionflow.DecisionAbort}}},
			Expect:      CaseExpectation{DecisionCode: decisionflow.DecisionAbort, ExitCode: decisionflow.DecisionAbort, Aborted: true},
		},
		{
			ID:          "C16",
			Description: "lrc commit",
			Scenario:    Scenario{Name: "C16", Trigger: TriggerLRC, InitialMessage: "feat: cli", StartPhase: decisionflow.PhaseReviewComplete, Events: []Event{DecisionEvent{Source: DecisionSourceTerminal, Code: decisionflow.DecisionCommit}}},
			Expect:      CaseExpectation{DecisionCode: decisionflow.DecisionCommit, ExitCode: 0, Committed: true, FinalMessage: "feat: cli"},
		},
		{
			ID:          "C17",
			Description: "serve web commit overrides cli",
			Scenario:    Scenario{Name: "C17", Trigger: TriggerLRC, InitialMessage: "feat: cli", StartPhase: decisionflow.PhaseReviewComplete, Events: []Event{DecisionEvent{Source: DecisionSourceWeb, Code: decisionflow.DecisionCommit, Message: "feat: web"}}},
			Expect:      CaseExpectation{DecisionCode: decisionflow.DecisionCommit, ExitCode: 0, Committed: true, FinalMessage: "feat: web"},
		},
		{
			ID:          "C18",
			Description: "serve web commit-push",
			Scenario:    Scenario{Name: "C18", Trigger: TriggerLRC, Precommit: true, InitialMessage: "feat: cli", StartPhase: decisionflow.PhaseReviewComplete, Events: []Event{DecisionEvent{Source: DecisionSourceWeb, Code: decisionflow.DecisionCommit, Message: "feat: web", Push: true}}},
			Expect:      CaseExpectation{DecisionCode: decisionflow.DecisionCommit, ExitCode: decisionflow.DecisionCommit, Committed: true, FinalMessage: "feat: web", PushRequested: true, PushMarkerPersisted: true, CommitMessageOverride: "feat: web"},
		},
		{
			ID:          "C19",
			Description: "serve web commit without cli message",
			Scenario:    Scenario{Name: "C19", Trigger: TriggerLRC, StartPhase: decisionflow.PhaseReviewComplete, Events: []Event{DecisionEvent{Source: DecisionSourceWeb, Code: decisionflow.DecisionCommit, Message: "feat: web-only"}}},
			Expect:      CaseExpectation{DecisionCode: decisionflow.DecisionCommit, ExitCode: 0, Committed: true, FinalMessage: "feat: web-only"},
		},
		{
			ID:          "C20",
			Description: "serve web abort",
			Scenario:    Scenario{Name: "C20", Trigger: TriggerLRC, StartPhase: decisionflow.PhaseReviewComplete, Events: []Event{DecisionEvent{Source: DecisionSourceWeb, Code: decisionflow.DecisionAbort}}},
			Expect:      CaseExpectation{DecisionCode: decisionflow.DecisionAbort, ExitCode: decisionflow.DecisionAbort, Aborted: true},
		},
		{
			ID:          "C21",
			Description: "terminal wins race",
			Scenario:    Scenario{Name: "C21", Trigger: TriggerLRC, StartPhase: decisionflow.PhaseReviewRunning, Events: []Event{DecisionEvent{Source: DecisionSourceTerminal, Code: decisionflow.DecisionSkip}, DecisionEvent{Source: DecisionSourceWeb, Code: decisionflow.DecisionCommit, Message: "feat: web"}}},
			Expect:      CaseExpectation{DecisionCode: decisionflow.DecisionSkip, ExitCode: 0, Skipped: true},
		},
		{
			ID:          "C22",
			Description: "web wins race",
			Scenario:    Scenario{Name: "C22", Trigger: TriggerLRC, StartPhase: decisionflow.PhaseReviewComplete, Events: []Event{DecisionEvent{Source: DecisionSourceWeb, Code: decisionflow.DecisionCommit, Message: "feat: web"}, DecisionEvent{Source: DecisionSourceTerminal, Code: decisionflow.DecisionAbort}}},
			Expect:      CaseExpectation{DecisionCode: decisionflow.DecisionCommit, ExitCode: 0, Committed: true, FinalMessage: "feat: web"},
		},
		{
			ID:          "C23",
			Description: "poll completes then grace-window decision",
			Scenario:    Scenario{Name: "C23", Trigger: TriggerLRC, InitialMessage: "feat: cli", StartPhase: decisionflow.PhaseReviewRunning, Events: []Event{PollCompletedEvent{}, DecisionEvent{Source: DecisionSourceTerminal, Code: decisionflow.DecisionCommit}}},
			Expect:      CaseExpectation{DecisionCode: decisionflow.DecisionCommit, ExitCode: 0, Committed: true, FinalMessage: "feat: cli"},
		},
		{
			ID:          "C24",
			Description: "invalid phase action rejected then valid action accepted",
			Scenario:    Scenario{Name: "C24", Trigger: TriggerLRC, InitialMessage: "feat: cli", StartPhase: decisionflow.PhaseReviewRunning, Events: []Event{DecisionEvent{Source: DecisionSourceWeb, Code: decisionflow.DecisionCommit, Message: "feat: web"}, PollCompletedEvent{}, DecisionEvent{Source: DecisionSourceWeb, Code: decisionflow.DecisionCommit, Message: "feat: web"}}},
			Expect:      CaseExpectation{DecisionCode: decisionflow.DecisionCommit, ExitCode: 0, Committed: true, FinalMessage: "feat: web"},
		},
	}
}

func MessageCases() []SimCase {
	return []SimCase{
		{
			ID:          "M01",
			Description: "CLI present, web empty",
			Scenario:    Scenario{Name: "M01", Trigger: TriggerLRC, InitialMessage: "cli", StartPhase: decisionflow.PhaseReviewComplete, Events: []Event{DecisionEvent{Source: DecisionSourceTerminal, Code: decisionflow.DecisionCommit}}},
			Expect:      CaseExpectation{DecisionCode: decisionflow.DecisionCommit, ExitCode: 0, Committed: true, FinalMessage: "cli"},
		},
		{
			ID:          "M02",
			Description: "Web overrides CLI",
			Scenario:    Scenario{Name: "M02", Trigger: TriggerLRC, InitialMessage: "cli", StartPhase: decisionflow.PhaseReviewComplete, Events: []Event{DecisionEvent{Source: DecisionSourceWeb, Code: decisionflow.DecisionCommit, Message: "web"}}},
			Expect:      CaseExpectation{DecisionCode: decisionflow.DecisionCommit, ExitCode: 0, Committed: true, FinalMessage: "web"},
		},
		{
			ID:          "M03",
			Description: "Web used without CLI",
			Scenario:    Scenario{Name: "M03", Trigger: TriggerLRC, StartPhase: decisionflow.PhaseReviewComplete, Events: []Event{DecisionEvent{Source: DecisionSourceWeb, Code: decisionflow.DecisionCommit, Message: "web"}}},
			Expect:      CaseExpectation{DecisionCode: decisionflow.DecisionCommit, ExitCode: 0, Committed: true, FinalMessage: "web"},
		},
		{
			ID:          "M04",
			Description: "CLI+Web+Editor -> Web stays final for web commit",
			Scenario:    Scenario{Name: "M04", Trigger: TriggerGitCommitEditor, Precommit: true, InitialMessage: "cli", EditorMessage: "editor", StartPhase: decisionflow.PhaseReviewComplete, Events: []Event{DecisionEvent{Source: DecisionSourceWeb, Code: decisionflow.DecisionCommit, Message: "web"}}},
			Expect:      CaseExpectation{DecisionCode: decisionflow.DecisionCommit, ExitCode: decisionflow.DecisionCommit, Committed: true, FinalMessage: "web", CommitMessageOverride: "web"},
		},
		{
			ID:          "M05",
			Description: "Editor used when CLI+Web absent",
			Scenario:    Scenario{Name: "M05", Trigger: TriggerGitCommitEditor, Precommit: true, EditorMessage: "editor", StartPhase: decisionflow.PhaseReviewComplete, Events: []Event{DecisionEvent{Source: DecisionSourceTerminal, Code: decisionflow.DecisionCommit}}},
			Expect:      CaseExpectation{DecisionCode: decisionflow.DecisionCommit, ExitCode: decisionflow.DecisionCommit, Committed: true, FinalMessage: "editor", CommitMessageOverride: "editor"},
		},
		{
			ID:          "M06",
			Description: "Editor overrides CLI in terminal commit path",
			Scenario:    Scenario{Name: "M06", Trigger: TriggerGitCommitEditor, Precommit: true, InitialMessage: "cli", EditorMessage: "editor", StartPhase: decisionflow.PhaseReviewComplete, Events: []Event{DecisionEvent{Source: DecisionSourceTerminal, Code: decisionflow.DecisionCommit}}},
			Expect:      CaseExpectation{DecisionCode: decisionflow.DecisionCommit, ExitCode: decisionflow.DecisionCommit, Committed: true, FinalMessage: "editor", CommitMessageOverride: "editor"},
		},
	}
}
