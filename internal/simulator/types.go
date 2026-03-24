package simulator

import "github.com/HexmosTech/git-lrc/internal/decisionflow"

// DecisionSource identifies where a decision event originated.
type DecisionSource string

const (
	DecisionSourceTerminal DecisionSource = "terminal"
	DecisionSourceWeb      DecisionSource = "web"
	DecisionSourceSignal   DecisionSource = "signal"
)

// TriggerMode identifies how the workflow was initiated.
type TriggerMode string

const (
	TriggerGitCommitWithMessage TriggerMode = "git-commit-m"
	TriggerGitCommitEditor      TriggerMode = "git-commit-editor"
	TriggerGitLRC               TriggerMode = "git-lrc"
	TriggerLRC                  TriggerMode = "lrc"
)

// EventRecord captures deterministic, assertable timeline details.
type EventRecord struct {
	Type    string
	Details map[string]string
}

// Scenario defines a simulated workflow run.
type Scenario struct {
	Name           string
	Trigger        TriggerMode
	Precommit      bool
	GitDir         string
	InitialMessage string
	EditorMessage  string
	StartPhase     decisionflow.Phase
	Events         []Event
}

// Result captures the final simulated outcome.
type Result struct {
	DecisionCode          int
	ExitCode              int
	FinalMessage          string
	PushRequested         bool
	Committed             bool
	Aborted               bool
	Skipped               bool
	Vouched               bool
	CommitMessageOverride string
	PushMarkerPersisted   bool
	CommitMessagePath     string
	PushMarkerPath        string
	Timeline              []EventRecord
}

// Event is an input to the Bubble Tea simulator model.
type Event interface{ isEvent() }

type PollCompletedEvent struct{}

func (PollCompletedEvent) isEvent() {}

type PollGraceExpiredEvent struct{}

func (PollGraceExpiredEvent) isEvent() {}

type DecisionEvent struct {
	Source  DecisionSource
	Code    int
	Message string
	Push    bool
}

func (DecisionEvent) isEvent() {}
