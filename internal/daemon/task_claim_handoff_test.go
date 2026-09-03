package daemon

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/session"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func TestTaskClaimHandoffRuntime(t *testing.T) {
	t.Run("Should prompt the bound session and stop the bootstrap after its turn", func(t *testing.T) {
		t.Parallel()

		sessions := &taskClaimHandoffSessionsStub{
			promptEvents: make(chan acp.AgentEvent),
		}
		runtime, err := newTaskClaimHandoffRuntime(t.Context(), sessions, nil)
		if err != nil {
			t.Fatalf("newTaskClaimHandoffRuntime() error = %v", err)
		}
		t.Cleanup(func() {
			close(sessions.promptEvents)
			if err := runtime.shutdown(context.Background()); err != nil {
				t.Fatalf("shutdown() error = %v", err)
			}
		})

		run := taskpkg.Run{ID: "run-1", TaskID: "task-1", SessionID: "sess-bound"}
		if err := runtime.Activate(t.Context(), "sess-bootstrap", run); err != nil {
			t.Fatalf("Activate() error = %v", err)
		}
		if !runtime.HasPending("sess-bootstrap") {
			t.Fatal("HasPending(bootstrap) = false, want true before turn end")
		}
		prompt := sessions.promptCall()
		if prompt.sessionID != "sess-bound" ||
			prompt.opts.Metadata.TaskID != "task-1" ||
			prompt.opts.Metadata.TaskRunID != "run-1" ||
			prompt.opts.Metadata.Reason != taskClaimHandoffReason ||
			prompt.opts.TurnID != taskClaimHandoffTurnID("sess-bound", "run-1") ||
			!strings.Contains(prompt.opts.Message, "Do not claim this run again") {
			t.Fatalf("PromptSynthetic() call = %#v, want bound run handoff", prompt)
		}

		runtime.onTurnEnd(t.Context(), "sess-bootstrap")
		if runtime.HasPending("sess-bootstrap") {
			t.Fatal("HasPending(bootstrap) = true, want consumed handoff")
		}
		stop := sessions.stopCall()
		if stop.sessionID != "sess-bootstrap" ||
			stop.cause != session.CauseCompleted ||
			!strings.Contains(stop.detail, `run "run-1" handed off to session "sess-bound"`) {
			t.Fatalf("StopWithCause() call = %#v, want completed bootstrap handoff", stop)
		}
	})

	t.Run("Should keep a failed bootstrap stop pending for the next turn end", func(t *testing.T) {
		t.Parallel()

		events := make(chan acp.AgentEvent)
		close(events)
		sessions := &taskClaimHandoffSessionsStub{
			promptEvents: events,
			stopErr:      errors.New("stop failed"),
		}
		runtime, err := newTaskClaimHandoffRuntime(t.Context(), sessions, nil)
		if err != nil {
			t.Fatalf("newTaskClaimHandoffRuntime() error = %v", err)
		}
		t.Cleanup(func() {
			if err := runtime.shutdown(context.Background()); err != nil {
				t.Fatalf("shutdown() error = %v", err)
			}
		})

		if err := runtime.Activate(t.Context(), "sess-bootstrap", taskpkg.Run{
			ID: "run-1", TaskID: "task-1", SessionID: "sess-bound",
		}); err != nil {
			t.Fatalf("Activate() error = %v", err)
		}
		runtime.onTurnEnd(t.Context(), "sess-bootstrap")
		if !runtime.HasPending("sess-bootstrap") {
			t.Fatal("HasPending(bootstrap) = false, want retryable failed stop")
		}
		sessions.setStopError(nil)
		runtime.onTurnEnd(t.Context(), "sess-bootstrap")
		if runtime.HasPending("sess-bootstrap") {
			t.Fatal("HasPending(bootstrap) = true, want successful retry consumed")
		}
		if got := sessions.stopCount(); got != 2 {
			t.Fatalf("StopWithCause() calls = %d, want 2", got)
		}
	})

	t.Run("Should reserve the bootstrap before prompting and release it when prompting fails", func(t *testing.T) {
		t.Parallel()
		sessions := &taskClaimHandoffSessionsStub{promptErr: errors.New("prompt failed")}
		runtime, err := newTaskClaimHandoffRuntime(t.Context(), sessions, nil)
		if err != nil {
			t.Fatalf("newTaskClaimHandoffRuntime() error = %v", err)
		}
		t.Cleanup(func() {
			if err := runtime.shutdown(context.Background()); err != nil {
				t.Fatalf("shutdown() error = %v", err)
			}
		})

		run := taskpkg.Run{ID: "run-1", TaskID: "task-1", SessionID: "sess-bound"}
		if err := runtime.Activate(t.Context(), "sess-bootstrap", run); err == nil {
			t.Fatal("Activate(prompt failure) error = nil")
		}
		if runtime.HasPending("sess-bootstrap") {
			t.Fatal("HasPending(prompt failure) = true, want released reservation")
		}
	})
}

type taskClaimHandoffPromptCall struct {
	sessionID string
	opts      session.SyntheticPromptOpts
}

type taskClaimHandoffStopCall struct {
	sessionID string
	cause     session.StopCause
	detail    string
}

type taskClaimHandoffSessionsStub struct {
	mu           sync.Mutex
	promptEvents chan acp.AgentEvent
	prompts      []taskClaimHandoffPromptCall
	stops        []taskClaimHandoffStopCall
	stopErr      error
	promptErr    error
}

func (s *taskClaimHandoffSessionsStub) PromptSynthetic(
	_ context.Context,
	id string,
	opts session.SyntheticPromptOpts,
) (<-chan acp.AgentEvent, error) {
	s.mu.Lock()
	s.prompts = append(s.prompts, taskClaimHandoffPromptCall{sessionID: id, opts: opts})
	events := s.promptEvents
	err := s.promptErr
	s.mu.Unlock()
	return events, err
}

func (s *taskClaimHandoffSessionsStub) StopWithCause(
	_ context.Context,
	id string,
	cause session.StopCause,
	detail string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stops = append(s.stops, taskClaimHandoffStopCall{sessionID: id, cause: cause, detail: detail})
	return s.stopErr
}

func (s *taskClaimHandoffSessionsStub) promptCall() taskClaimHandoffPromptCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.prompts) == 0 {
		return taskClaimHandoffPromptCall{}
	}
	return s.prompts[len(s.prompts)-1]
}

func (s *taskClaimHandoffSessionsStub) stopCall() taskClaimHandoffStopCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.stops) == 0 {
		return taskClaimHandoffStopCall{}
	}
	return s.stops[len(s.stops)-1]
}

func (s *taskClaimHandoffSessionsStub) setStopError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopErr = err
}

func (s *taskClaimHandoffSessionsStub) stopCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.stops)
}
