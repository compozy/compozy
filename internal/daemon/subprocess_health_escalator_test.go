package daemon

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/subprocess"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
)

func TestSubprocessHealthEscalator(t *testing.T) {
	t.Parallel()

	t.Run("Should surface without mutation when the gate is disabled", func(t *testing.T) {
		t.Parallel()

		runs := &subprocessHealthRunSourceStub{}
		actor := &subprocessHealthEscalationActorStub{runs: runs}
		escalator, err := newSubprocessHealthEscalator(runs, actor, 0, discardLogger())
		if err != nil {
			t.Fatalf("newSubprocessHealthEscalator() error = %v", err)
		}

		escalator.OnSubprocessHealth(testutil.Context(t), unhealthySubprocessObservation(9))

		if runs.listCalls != 0 || actor.markCalls != 0 {
			t.Fatalf("disabled calls = list:%d mark:%d, want 0/0", runs.listCalls, actor.markCalls)
		}
	})

	t.Run("Should escalate one exact nonterminal binding after the threshold", func(t *testing.T) {
		t.Parallel()

		runs := &subprocessHealthRunSourceStub{runs: []taskpkg.Run{{
			ID:        "run-health",
			TaskID:    "task-health",
			SessionID: "sess-health",
			Status:    taskpkg.TaskRunStatusRunning,
		}}}
		actor := &subprocessHealthEscalationActorStub{runs: runs}
		escalator, err := newSubprocessHealthEscalator(runs, actor, 3, discardLogger())
		if err != nil {
			t.Fatalf("newSubprocessHealthEscalator() error = %v", err)
		}

		escalator.OnSubprocessHealth(testutil.Context(t), unhealthySubprocessObservation(2))
		escalator.OnSubprocessHealth(testutil.Context(t), unhealthySubprocessObservation(3))
		escalator.OnSubprocessHealth(testutil.Context(t), unhealthySubprocessObservation(4))

		if got, want := runs.lastQuery.SessionID, "sess-health"; got != want {
			t.Fatalf("ListTaskRuns().SessionID = %q, want %q", got, want)
		}
		if got, want := actor.markCalls, 1; got != want {
			t.Fatalf("MarkRunNeedsAttention() calls = %d, want %d", got, want)
		}
		if actor.runID != "run-health" || strings.Contains(actor.diagnostic, "super-secret") ||
			!strings.Contains(actor.diagnostic, "probe failed") {
			t.Fatalf(
				"escalation = run:%q diagnostic:%q, want redacted correlated failure",
				actor.runID,
				actor.diagnostic,
			)
		}
	})

	t.Run("Should leave a terminal binding unchanged", func(t *testing.T) {
		t.Parallel()

		runs := &subprocessHealthRunSourceStub{runs: []taskpkg.Run{{
			ID:        "run-terminal",
			TaskID:    "task-terminal",
			SessionID: "sess-health",
			Status:    taskpkg.TaskRunStatusCompleted,
		}}}
		actor := &subprocessHealthEscalationActorStub{runs: runs}
		escalator, err := newSubprocessHealthEscalator(runs, actor, 3, discardLogger())
		if err != nil {
			t.Fatalf("newSubprocessHealthEscalator() error = %v", err)
		}

		escalator.OnSubprocessHealth(testutil.Context(t), unhealthySubprocessObservation(3))

		if actor.markCalls != 0 {
			t.Fatalf("MarkRunNeedsAttention() calls = %d, want 0", actor.markCalls)
		}
	})
}

type subprocessHealthRunSourceStub struct {
	mu        sync.Mutex
	runs      []taskpkg.Run
	listCalls int
	lastQuery taskpkg.RunQuery
}

func (s *subprocessHealthRunSourceStub) ListTaskRuns(
	_ context.Context,
	query taskpkg.RunQuery,
) ([]taskpkg.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listCalls++
	s.lastQuery = query
	return append([]taskpkg.Run(nil), s.runs...), nil
}

type subprocessHealthEscalationActorStub struct {
	mu         sync.Mutex
	runs       *subprocessHealthRunSourceStub
	markCalls  int
	runID      string
	diagnostic string
}

func (a *subprocessHealthEscalationActorStub) MarkRunNeedsAttention(
	_ context.Context,
	runID string,
	diagnostic string,
) (taskpkg.Run, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.markCalls++
	a.runID = runID
	a.diagnostic = diagnostic

	a.runs.mu.Lock()
	defer a.runs.mu.Unlock()
	for idx := range a.runs.runs {
		if a.runs.runs[idx].ID != runID {
			continue
		}
		a.runs.runs[idx].Status = taskpkg.TaskRunStatusNeedsAttention
		return a.runs.runs[idx], nil
	}
	return taskpkg.Run{}, taskpkg.ErrTaskRunNotFound
}

func unhealthySubprocessObservation(failures int) session.SubprocessHealthSnapshot {
	return session.SubprocessHealthSnapshot{
		SessionID:   "sess-health",
		WorkspaceID: "ws-health",
		AgentName:   "coder",
		Health: subprocess.HealthState{
			Healthy:             false,
			Message:             "api_key=super-secret probe failed",
			LastCheckedAt:       time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC),
			ConsecutiveFailures: failures,
			LastError:           "context deadline exceeded",
		},
	}
}
