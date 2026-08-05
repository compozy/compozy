package globaldb

import (
	"context"
	"encoding/json"
	"fmt"

	eventspkg "github.com/compozy/compozy/internal/events"
	storepkg "github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

type completedRunHistoryPayload struct {
	Status     taskpkg.RunStatus `json:"status"`
	TaskStatus taskpkg.Status    `json:"task_status"`
	Result     json.RawMessage   `json:"result,omitempty"`
	Historical bool              `json:"historical"`
}

// ImportCompletedRunHistory atomically imports one completed bootstrap record,
// its task projection, and its canonical audit event.
func (g *TaskRepo) ImportCompletedRunHistory(
	ctx context.Context,
	command *taskpkg.CompletedRunHistoryImport,
) error {
	if err := g.checkReady(ctx, "import completed task run history"); err != nil {
		return err
	}

	run, actor := command.Run(), command.Actor()
	if run.Status.Normalize() != taskpkg.TaskRunStatusCompleted {
		return fmt.Errorf(
			"%w: completed history import requires completed status",
			taskpkg.ErrInvalidStatusTransition,
		)
	}
	normalized, err := g.normalizeTaskRunForCreate(run)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(completedRunHistoryPayload{
		Status:     taskpkg.TaskRunStatusCompleted,
		TaskStatus: taskpkg.TaskStatusCompleted,
		Result:     normalized.Result,
		Historical: true,
	})
	if err != nil {
		return fmt.Errorf("store: marshal completed run history payload: %w", err)
	}
	eventID, err := storepkg.NewID("evt")
	if err != nil {
		return fmt.Errorf("store: generate completed run history event id: %w", err)
	}
	event, err := g.normalizeTaskEventForCreate(taskpkg.Event{
		ID:        eventID,
		TaskID:    normalized.TaskID,
		RunID:     normalized.ID,
		EventType: eventspkg.TaskRunCompleted,
		Actor:     actor.Actor,
		Origin:    actor.Origin,
		Payload:   payload,
		Timestamp: normalized.EndedAt,
	})
	if err != nil {
		return err
	}

	return g.withTaskImmediateTransaction(ctx, "import completed task run history", func(exec taskSQLExecutor) error {
		taskRecord, err := g.getTaskWithExecutor(ctx, exec, normalized.TaskID)
		if err != nil {
			return err
		}
		if taskRecord.Status.Normalize() != taskpkg.TaskStatusCompleted {
			return fmt.Errorf(
				"%w: completed run history requires completed task %q",
				taskpkg.ErrInvalidStatusTransition,
				taskRecord.ID,
			)
		}
		normalized, err = bindTaskRunWorkspace(normalized, taskRecord)
		if err != nil {
			return err
		}
		if err := insertTaskRunWithExecutor(ctx, exec, normalized); err != nil {
			return err
		}
		if err := replaceTaskRunCapabilitiesWithExecutor(ctx, exec, normalized); err != nil {
			return err
		}
		return appendTaskEventWithExecutor(ctx, exec, EventRecordInsert{
			ID:        event.ID,
			TaskID:    event.TaskID,
			RunID:     event.RunID,
			EventType: event.EventType,
			Actor:     event.Actor,
			Origin:    event.Origin,
			Payload:   event.Payload,
			Timestamp: event.Timestamp,
		})
	})
}
