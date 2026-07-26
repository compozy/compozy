//go:build !integration

package daemon

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
)

type taskWakeBridgePromptCall struct {
	SessionID string
	Opts      session.SyntheticPromptOpts
}

type taskWakeBridgeSessions struct {
	calls []taskWakeBridgePromptCall
	err   error
}

func (s *taskWakeBridgeSessions) PromptSynthetic(
	ctx context.Context,
	sessionID string,
	opts session.SyntheticPromptOpts,
) (<-chan acp.AgentEvent, error) {
	if ctx == nil {
		return nil, errors.New("prompt context is required")
	}
	s.calls = append(s.calls, taskWakeBridgePromptCall{
		SessionID: strings.TrimSpace(sessionID),
		Opts:      opts,
	})
	if s.err != nil {
		return nil, s.err
	}
	events := make(chan acp.AgentEvent)
	close(events)
	return events, nil
}

func TestTaskWakeBridgeWakeCreatorPromptsSyntheticQueueMode(t *testing.T) {
	t.Parallel()

	t.Run("Should prompt the creator with queued synthetic metadata", func(t *testing.T) {
		t.Parallel()

		sessions := &taskWakeBridgeSessions{}
		bridge, err := newTaskWakeBridge(context.Background(), sessions, nil)
		if err != nil {
			t.Fatalf("newTaskWakeBridge() error = %v", err)
		}
		event := taskpkg.WakeEvent{
			WakeEventID: "wake-terminal-task-1-run-1-completed",
			WorkspaceID: "ws-1",
			TaskID:      "task-1",
			RunID:       "run-1",
			Reason:      taskpkg.WakeReasonTerminal,
			Summary:     "Task completed",
		}

		if err := bridge.WakeCreator(context.Background(), "sess-creator", event); err != nil {
			t.Fatalf("WakeCreator() error = %v", err)
		}

		if got, want := len(sessions.calls), 1; got != want {
			t.Fatalf("len(PromptSynthetic calls) = %d, want %d", got, want)
		}
		call := sessions.calls[0]
		if got, want := call.SessionID, "sess-creator"; got != want {
			t.Fatalf("SessionID = %q, want %q", got, want)
		}
		if call.Opts.SkipIfBusy {
			t.Fatal("SkipIfBusy = true, want false for queued delivery")
		}
		if call.Opts.InterruptIfAgentWaiting {
			t.Fatal("InterruptIfAgentWaiting = true, want false")
		}
		meta := call.Opts.Metadata
		if got, want := meta.TaskID, event.TaskID; got != want {
			t.Fatalf("Metadata.TaskID = %q, want %q", got, want)
		}
		if got, want := meta.TaskRunID, event.RunID; got != want {
			t.Fatalf("Metadata.TaskRunID = %q, want %q", got, want)
		}
		if got, want := meta.WakeEventID, event.WakeEventID; got != want {
			t.Fatalf("Metadata.WakeEventID = %q, want %q", got, want)
		}
		if got, want := meta.Reason, string(event.Reason); got != want {
			t.Fatalf("Metadata.Reason = %q, want %q", got, want)
		}
		if got, want := meta.Summary, event.Summary; got != want {
			t.Fatalf("Metadata.Summary = %q, want %q", got, want)
		}
		if !strings.Contains(call.Opts.Message, event.TaskID) ||
			!strings.Contains(call.Opts.Message, event.RunID) ||
			!strings.Contains(call.Opts.Message, event.WakeEventID) {
			t.Fatalf("Message = %q, want task/run/wake ids", call.Opts.Message)
		}
	})
}

func TestTaskWakeBridgeWakeCreatorMapsDeadSessions(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		err  error
	}{
		{name: "missing", err: session.ErrSessionNotFound},
		{name: "inactive", err: session.ErrSessionNotActive},
	} {
		t.Run("Should map "+tc.name+" session to ErrSessionNotLive", func(t *testing.T) {
			t.Parallel()

			sessions := &taskWakeBridgeSessions{err: tc.err}
			bridge, err := newTaskWakeBridge(context.Background(), sessions, nil)
			if err != nil {
				t.Fatalf("newTaskWakeBridge() error = %v", err)
			}

			wakeErr := bridge.WakeCreator(context.Background(), "sess-dead", taskpkg.WakeEvent{
				WakeEventID: "wake-terminal-task-1-run-1-completed",
				TaskID:      "task-1",
				RunID:       "run-1",
				Reason:      taskpkg.WakeReasonTerminal,
				Summary:     "Task completed",
			})
			if !errors.Is(wakeErr, taskpkg.ErrSessionNotLive) {
				t.Fatalf("WakeCreator() error = %v, want %v", wakeErr, taskpkg.ErrSessionNotLive)
			}
			if got, want := len(sessions.calls), 1; got != want {
				t.Fatalf("len(PromptSynthetic calls) = %d, want %d", got, want)
			}
		})
	}
}

func TestTaskWakeBridgeWakeCreatorRedactsRawClaimTokens(t *testing.T) {
	t.Parallel()

	t.Run("Should redact raw claim tokens from wake prompts", func(t *testing.T) {
		t.Parallel()

		rawToken := "agh_claim_secret123"
		sessions := &taskWakeBridgeSessions{}
		bridge, err := newTaskWakeBridge(context.Background(), sessions, nil)
		if err != nil {
			t.Fatalf("newTaskWakeBridge() error = %v", err)
		}

		if err := bridge.WakeCreator(context.Background(), "sess-creator", taskpkg.WakeEvent{
			WakeEventID: "wake-terminal-task-1-run-1-failed",
			TaskID:      "task-1",
			RunID:       "run-1",
			Reason:      taskpkg.WakeReasonTerminal,
			Summary:     "Task failed with " + rawToken,
		}); err != nil {
			t.Fatalf("WakeCreator() error = %v", err)
		}

		if got, want := len(sessions.calls), 1; got != want {
			t.Fatalf("len(PromptSynthetic calls) = %d, want %d", got, want)
		}
		call := sessions.calls[0]
		if strings.Contains(call.Opts.Message, rawToken) {
			t.Fatalf("Message contains raw claim token: %q", call.Opts.Message)
		}
		if strings.Contains(call.Opts.Metadata.Summary, rawToken) {
			t.Fatalf("Metadata.Summary contains raw claim token: %q", call.Opts.Metadata.Summary)
		}
	})
}
