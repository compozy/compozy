package globaldb

import (
	"context"
	"encoding/json"
	"fmt"

	eventspkg "github.com/compozy/compozy/internal/events"
	storepkg "github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

type terminalRunHistoryPayload struct {
	Status     taskpkg.RunStatus `json:"status"`
	TaskStatus taskpkg.Status    `json:"task_status"`
	Result     json.RawMessage   `json:"result,omitempty"`
	Historical bool              `json:"historical"`
}

// ImportTerminalRunHistory atomically imports one finished bootstrap record,
// its task projection, and its canonical audit event.
func (g *TaskRepo) ImportTerminalRunHistory(
	ctx context.Context,
	command *taskpkg.TerminalRunHistoryImport,
) error {
	if err := g.checkReady(ctx, "import terminal task run history"); err != nil {
		return err
	}
	if command == nil {
		return fmt.Errorf("%w: terminal run history import command is required", taskpkg.ErrValidation)
	}
	run, actor := command.Run(), command.Actor()
	runStatus := run.Status.Normalize()
	taskStatus, ok := taskpkg.StatusForTerminalRun(runStatus)
	if !ok {
		return fmt.Errorf(
			"%w: history import requires a terminal run status, got %q",
			taskpkg.ErrInvalidStatusTransition,
			run.Status,
		)
	}
	normalized, err := g.normalizeTaskRunForCreate(run)
	if err != nil {
		return err
	}
	event, err := g.terminalRunHistoryEvent(normalized, actor, runStatus, taskStatus)
	if err != nil {
		return err
	}
	return g.withTaskImmediateTransaction(ctx, "import terminal task run history", func(exec taskSQLExecutor) error {
		return importTerminalRunHistory(ctx, g, exec, normalized, event, taskStatus)
	})
}

func (g *TaskRepo) terminalRunHistoryEvent(
	run taskpkg.Run,
	actor taskpkg.ActorContext,
	runStatus taskpkg.RunStatus,
	taskStatus taskpkg.Status,
) (taskpkg.Event, error) {
	payload, err := json.Marshal(terminalRunHistoryPayload{
		Status:     runStatus,
		TaskStatus: taskStatus,
		Result:     run.Result,
		Historical: true,
	})
	if err != nil {
		return taskpkg.Event{}, fmt.Errorf("store: marshal terminal run history payload: %w", err)
	}
	eventID, err := storepkg.NewID("evt")
	if err != nil {
		return taskpkg.Event{}, fmt.Errorf("store: generate terminal run history event id: %w", err)
	}
	return g.normalizeTaskEventForCreate(taskpkg.Event{
		ID:        eventID,
		TaskID:    run.TaskID,
		RunID:     run.ID,
		EventType: terminalRunHistoryEventType(runStatus),
		Actor:     actor.Actor,
		Origin:    actor.Origin,
		Payload:   payload,
		Timestamp: run.EndedAt,
	})
}

func terminalRunHistoryEventType(status taskpkg.RunStatus) string {
	switch status {
	case taskpkg.TaskRunStatusFailed:
		return eventspkg.TaskRunFailed
	case taskpkg.TaskRunStatusCanceled:
		return eventspkg.TaskRunCanceled
	default:
		return eventspkg.TaskRunCompleted
	}
}

func importTerminalRunHistory(
	ctx context.Context,
	repo *TaskRepo,
	exec taskSQLExecutor,
	run taskpkg.Run,
	event taskpkg.Event,
	taskStatus taskpkg.Status,
) error {
	taskRecord, err := repo.getTaskWithExecutor(ctx, exec, run.TaskID)
	if err != nil {
		return err
	}
	if taskRecord.Status.Normalize() != taskStatus {
		return fmt.Errorf(
			"%w: %s run history requires task %q in status %q",
			taskpkg.ErrInvalidStatusTransition,
			run.Status.Normalize(),
			taskRecord.ID,
			taskStatus,
		)
	}
	bound, err := bindTaskRunWorkspace(run, taskRecord)
	if err != nil {
		return err
	}
	if err := insertTaskRunWithExecutor(ctx, exec, bound); err != nil {
		return err
	}
	if err := replaceTaskRunCapabilitiesWithExecutor(ctx, exec, bound); err != nil {
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
}
