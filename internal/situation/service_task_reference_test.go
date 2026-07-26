package situation

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
)

func TestContextForSessionTopLevelTaskReferenceContract(t *testing.T) {
	t.Parallel()

	t.Run("Should redact top-level task reference text and preserve latest event sequence", func(t *testing.T) {
		t.Parallel()

		taskRecord := taskpkg.Task{
			ID:           "task-top-level",
			Identifier:   "ORCH-agh_claim_IDENTIFIER_SECRET",
			Scope:        taskpkg.ScopeWorkspace,
			WorkspaceID:  "ws-1",
			Title:        "do not leak agh_claim_TITLE_SECRET",
			Status:       taskpkg.TaskStatusInProgress,
			Priority:     taskpkg.PriorityHigh,
			MaxAttempts:  2,
			CurrentRunID: "run-1",
		}
		run := taskpkg.Run{
			ID:        "run-1",
			TaskID:    taskRecord.ID,
			Status:    taskpkg.TaskRunStatusRunning,
			Attempt:   1,
			SessionID: "sess-1",
			QueuedAt:  fixedTime(),
			StartedAt: fixedTime().Add(time.Minute),
		}
		event := taskpkg.Event{
			ID:        "evt-1",
			TaskID:    taskRecord.ID,
			RunID:     run.ID,
			EventType: "task.run.started",
			Payload:   jsonRaw(t, `{"message":"started"}`),
			Timestamp: fixedTime().Add(2 * time.Minute),
		}
		service := NewService(Deps{
			Now: fixedNow,
			TaskStore: taskStoreStub{
				tasks:  map[string]taskpkg.Task{taskRecord.ID: taskRecord},
				runs:   []taskpkg.Run{run},
				events: []taskpkg.Event{event},
			},
		})

		payload, err := service.ContextForSession(context.Background(), &session.Info{
			ID:          run.SessionID,
			AgentName:   "coder",
			Provider:    "codex",
			WorkspaceID: taskRecord.WorkspaceID,
			Workspace:   "/work/agh",
			Type:        session.SessionTypeUser,
			State:       session.StateActive,
			CreatedAt:   fixedTime(),
			UpdatedAt:   fixedTime(),
		})
		if err != nil {
			t.Fatalf("ContextForSession() error = %v", err)
		}
		if payload.Task.Task == nil {
			t.Fatal("Task.Task = nil, want task reference")
		}
		if got, want := payload.Task.Task.LatestEventSeq, int64(1); got != want {
			t.Fatalf("Task.Task.LatestEventSeq = %d, want %d", got, want)
		}
		if strings.Contains(payload.Task.Task.Title, "TITLE_SECRET") {
			t.Fatalf("Task.Task.Title = %q, want claim token redacted", payload.Task.Task.Title)
		}
		if strings.Contains(payload.Task.Task.Identifier, "IDENTIFIER_SECRET") {
			t.Fatalf("Task.Task.Identifier = %q, want claim token redacted", payload.Task.Task.Identifier)
		}

		rendered, err := RenderPrompt(&payload)
		if err != nil {
			t.Fatalf("RenderPrompt() error = %v", err)
		}
		for _, leaked := range []string{"TITLE_SECRET", "IDENTIFIER_SECRET"} {
			if strings.Contains(rendered, leaked) {
				t.Fatalf("RenderPrompt() leaked %q: %s", leaked, rendered)
			}
		}
		if !strings.Contains(rendered, "agh_claim_[REDACTED]") {
			t.Fatalf("RenderPrompt() = %s, want claim token redaction marker", rendered)
		}
	})
}
