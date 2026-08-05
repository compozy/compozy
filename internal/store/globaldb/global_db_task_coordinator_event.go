package globaldb

import (
	"context"
	"fmt"

	taskpkg "github.com/compozy/compozy/internal/task"
)

func appendCoordinatorCompletionEventWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	eventID string,
	completion taskpkg.CoordinatorCompletion,
	result *taskpkg.CoordinatorCompletionResult,
) (taskpkg.Event, error) {
	if result.Settlement == nil {
		return taskpkg.Event{}, fmt.Errorf(
			"%w: coordinator completion settlement is required",
			taskpkg.ErrValidation,
		)
	}
	event, err := taskpkg.NewCoordinatorRunCompletedEvent(
		eventID,
		result.Run,
		result.Settlement.Task,
		completion.Actor,
		completion.Now,
	)
	if err != nil {
		return taskpkg.Event{}, err
	}
	if err := appendTaskEventWithExecutor(ctx, exec, EventRecordInsert{
		ID:        event.ID,
		TaskID:    event.TaskID,
		RunID:     event.RunID,
		EventType: event.EventType,
		Actor:     event.Actor,
		Origin:    event.Origin,
		Payload:   event.Payload,
		Timestamp: event.Timestamp,
	}); err != nil {
		return taskpkg.Event{}, err
	}
	return event, nil
}
