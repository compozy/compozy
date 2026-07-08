package acpshared

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
)

func TestHandleSessionTimeoutClassifiesStalls(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		timeoutErr  error
		wantStalled bool
	}{
		{
			name:        "Should tag an idle-window expiry as a stall",
			timeoutErr:  NewActivityTimeoutError(3 * time.Minute),
			wantStalled: true,
		},
		{
			name:        "Should not tag an init timeout as a stall",
			timeoutErr:  NewInitTimeoutError(30 * time.Second),
			wantStalled: false,
		},
		{
			name:        "Should not tag a plain deadline as a stall",
			timeoutErr:  context.DeadlineExceeded,
			wantStalled: false,
		},
		{
			name:        "Should tag a wrapped activity timeout as a stall",
			timeoutErr:  errors.Join(errors.New("outer"), NewActivityTimeoutError(time.Minute)),
			wantStalled: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			jb := &job{SafeName: "task_01", CodeFiles: []string{"task_01.md"}}
			result := HandleSessionTimeout(tt.timeoutErr, jb, 0, false, time.Minute)
			if result.Stalled != tt.wantStalled {
				t.Fatalf("Stalled = %t, want %t", result.Stalled, tt.wantStalled)
			}
			if !result.Retryable {
				t.Fatal("a session timeout must stay retryable")
			}
			if result.Status != attemptStatusTimeout {
				t.Fatalf("status = %q, want %q", result.Status, attemptStatusTimeout)
			}
		})
	}
}

func TestSessionUpdateHandlerTracksLastToolCall(t *testing.T) {
	t.Parallel()

	t.Run("Should report the most recent tool call transition", func(t *testing.T) {
		t.Parallel()
		handler := newHandlerForToolCallTracking(t)

		handler.applySessionUpdate(model.SessionUpdate{
			Kind:       model.UpdateKindToolCallStarted,
			ToolCallID: "tool-1",
		})
		handler.applySessionUpdate(model.SessionUpdate{
			Kind:       model.UpdateKindToolCallUpdated,
			ToolCallID: "tool-2",
		})

		if got := handler.LastToolCall(); got != "tool-2" {
			t.Fatalf("LastToolCall = %q, want tool-2", got)
		}
	})

	t.Run("Should ignore non tool-call updates", func(t *testing.T) {
		t.Parallel()
		handler := newHandlerForToolCallTracking(t)

		handler.applySessionUpdate(model.SessionUpdate{
			Kind:       model.UpdateKindToolCallStarted,
			ToolCallID: "tool-1",
		})
		handler.applySessionUpdate(model.SessionUpdate{Kind: model.UpdateKindAgentMessageChunk})
		handler.applySessionUpdate(model.SessionUpdate{Kind: model.UpdateKindPlanUpdated})

		if got := handler.LastToolCall(); got != "tool-1" {
			t.Fatalf("LastToolCall = %q, want tool-1", got)
		}
	})

	t.Run("Should be empty before any tool call and safe on a nil handler", func(t *testing.T) {
		t.Parallel()
		if got := newHandlerForToolCallTracking(t).LastToolCall(); got != "" {
			t.Fatalf("LastToolCall = %q, want empty", got)
		}
		var nilHandler *SessionUpdateHandler
		if got := nilHandler.LastToolCall(); got != "" {
			t.Fatalf("nil handler LastToolCall = %q, want empty", got)
		}
	})
}

func newHandlerForToolCallTracking(t *testing.T) *SessionUpdateHandler {
	t.Helper()
	var jobUsage model.Usage
	var aggregate model.Usage
	return newSessionUpdateHandler(SessionUpdateHandlerConfig{
		Context:        context.Background(),
		Index:          0,
		AgentID:        model.IDECodex,
		SessionID:      "sess-tool-call",
		JobUsage:       &jobUsage,
		AggregateUsage: &aggregate,
		AggregateMu:    &sync.Mutex{},
	})
}
