package simulator

import (
	"context"
	"errors"

	tea "charm.land/bubbletea/v2"
)

var ErrRealNetworkNotAllowed = errors.New("simulator forbids real network review calls")

// ReviewBackend simulates review submission/polling for deterministic scenarios.
type ReviewBackend interface {
	Submit(ctx context.Context, trigger TriggerMode) (string, error)
}

// FakeBackend is the default backend for simulator tests.
type FakeBackend struct {
	ReviewID    string
	SubmitCalls int
}

func (f *FakeBackend) Submit(_ context.Context, _ TriggerMode) (string, error) {
	f.SubmitCalls++
	if f.ReviewID == "" {
		f.ReviewID = "sim-review-1"
	}
	return f.ReviewID, nil
}

// RealNetworkGuard fails fast if a scenario tries to use a non-fake backend.
type RealNetworkGuard struct{}

func (RealNetworkGuard) Submit(_ context.Context, _ TriggerMode) (string, error) {
	return "", ErrRealNetworkNotAllowed
}

type Engine struct {
	Backend ReviewBackend
}

func (e Engine) Run(ctx context.Context, s Scenario) (Result, error) {
	backend := e.Backend
	if backend == nil {
		backend = &FakeBackend{}
	}

	if _, err := backend.Submit(ctx, s.Trigger); err != nil {
		return Result{}, err
	}

	m := newScenarioModel(s)
	for _, ev := range s.Events {
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		updated, _ := m.Update(eventToMsg(ev))
		m = updated.(scenarioModel)
	}

	result := m.result
	result.Timeline = append(result.Timeline, m.timeline...)
	if s.Precommit {
		if err := applyPrecommitArtifacts(s.GitDir, &result); err != nil {
			return Result{}, err
		}
	}
	return result, nil
}

func eventToMsg(ev Event) tea.Msg {
	return ev
}
