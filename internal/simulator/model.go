package simulator

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/HexmosTech/git-lrc/internal/decisionflow"
)

type scenarioModel struct {
	phase          decisionflow.Phase
	precommit      bool
	initialMessage string
	editorMessage  string
	resolved       bool
	result         Result
	timeline       []EventRecord
}

func newScenarioModel(s Scenario) scenarioModel {
	phase := s.StartPhase
	if phase != decisionflow.PhaseReviewComplete {
		phase = decisionflow.PhaseReviewRunning
	}
	return scenarioModel{
		phase:          phase,
		precommit:      s.Precommit,
		initialMessage: s.InitialMessage,
		editorMessage:  s.EditorMessage,
		result: Result{
			DecisionCode: -1,
			ExitCode:     -1,
		},
		timeline: make([]EventRecord, 0, 8),
	}
}

func (m scenarioModel) Init() tea.Cmd {
	return nil
}

func (m scenarioModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case PollCompletedEvent:
		if m.phase != decisionflow.PhaseReviewComplete {
			m.phase = decisionflow.PhaseReviewComplete
			m.record("phase", map[string]string{"value": "complete"})
		}
	case PollGraceExpiredEvent:
		m.record("grace-expired", nil)
	case DecisionEvent:
		m.applyDecision(v)
	}
	return m, nil
}

func (m scenarioModel) View() tea.View {
	state := "running"
	if m.phase == decisionflow.PhaseReviewComplete {
		state = "complete"
	}
	return tea.NewView(fmt.Sprintf("phase=%s resolved=%t", state, m.resolved))
}

func (m *scenarioModel) applyDecision(ev DecisionEvent) {
	m.record("decision-attempt", map[string]string{
		"source": string(ev.Source),
		"code":   fmt.Sprintf("%d", ev.Code),
	})

	if m.resolved {
		m.record("decision-ignored", map[string]string{"reason": "already-resolved"})
		return
	}

	if !actionAllowed(ev, m.phase) {
		m.record("decision-rejected", map[string]string{"reason": "invalid-phase"})
		return
	}

	if ev.Source == DecisionSourceWeb && ev.Code == decisionflow.DecisionCommit && ev.Message == "" {
		m.record("decision-rejected", map[string]string{"reason": "empty-web-message"})
		return
	}

	m.resolved = true
	m.result.DecisionCode = ev.Code
	m.result.PushRequested = ev.Push

	switch ev.Code {
	case decisionflow.DecisionCommit:
		m.result.FinalMessage = resolveFinalMessage(ev, m.initialMessage, m.editorMessage)
		m.result.Committed = true
		if m.precommit {
			m.result.ExitCode = decisionflow.DecisionCommit
			if m.result.FinalMessage != "" {
				m.result.CommitMessageOverride = m.result.FinalMessage
			}
			m.result.PushMarkerPersisted = ev.Push
		} else {
			m.result.ExitCode = 0
		}
	case decisionflow.DecisionAbort:
		m.result.FinalMessage = ""
		m.result.Aborted = true
		m.result.ExitCode = decisionflow.DecisionAbort
	case decisionflow.DecisionSkip:
		m.result.FinalMessage = ev.Message
		m.result.Skipped = true
		if m.precommit {
			m.result.ExitCode = decisionflow.DecisionSkip
		} else {
			m.result.ExitCode = 0
		}
		m.result.CommitMessageOverride = ""
		m.result.PushMarkerPersisted = false
	case decisionflow.DecisionVouch:
		m.result.FinalMessage = ev.Message
		m.result.Vouched = true
		if m.precommit {
			// precommit maps vouch to skip exit code in current runtime behavior.
			m.result.ExitCode = decisionflow.DecisionSkip
		} else {
			m.result.ExitCode = 0
		}
		m.result.CommitMessageOverride = ""
		m.result.PushMarkerPersisted = false
	}

	m.record("decision-accepted", map[string]string{
		"source": string(ev.Source),
		"code":   fmt.Sprintf("%d", ev.Code),
	})
}

func (m *scenarioModel) record(kind string, details map[string]string) {
	if details == nil {
		details = map[string]string{}
	}
	m.timeline = append(m.timeline, EventRecord{Type: kind, Details: details})
}

func actionAllowed(ev DecisionEvent, phase decisionflow.Phase) bool {
	// Terminal commit can rely on existing message fallback, while web commit requires explicit message.
	if ev.Source == DecisionSourceTerminal && ev.Code == decisionflow.DecisionCommit {
		return phase == decisionflow.PhaseReviewComplete
	}
	return decisionflow.ActionAllowedInPhase(ev.Code, phase)
}

func resolveFinalMessage(ev DecisionEvent, initialMessage, editorMessage string) string {
	if ev.Source == DecisionSourceWeb && ev.Message != "" {
		return ev.Message
	}
	if editorMessage != "" {
		return editorMessage
	}
	if ev.Message != "" {
		return ev.Message
	}
	return initialMessage
}
