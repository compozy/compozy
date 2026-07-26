package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

// EventRecordInsert captures the normalized task-event fields written by an
// owning transaction.
type EventRecordInsert struct {
	ID        string
	TaskID    string
	RunID     string
	EventType string
	Actor     taskpkg.ActorIdentity
	Origin    taskpkg.Origin
	Payload   json.RawMessage
	Timestamp time.Time
}

func appendTaskEventWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	insert EventRecordInsert,
) error {
	_, err := appendTaskEventRecordWithExecutor(ctx, exec, insert)
	return err
}

func appendTaskEventRecordWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	insert EventRecordInsert,
) (taskpkg.EventRecord, error) {
	event := normalizeTaskEventRecord(taskpkg.Event{
		ID:        insert.ID,
		TaskID:    insert.TaskID,
		RunID:     insert.RunID,
		EventType: insert.EventType,
		Actor:     insert.Actor,
		Origin:    insert.Origin,
		Payload:   insert.Payload,
		Timestamp: insert.Timestamp,
	})
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	if err := event.Validate(); err != nil {
		return taskpkg.EventRecord{}, err
	}

	if err := ensureTaskEventTaskExists(ctx, exec, event.TaskID); err != nil {
		return taskpkg.EventRecord{}, err
	}
	if strings.TrimSpace(event.RunID) != "" {
		runTaskID, err := taskEventRunTaskID(ctx, exec, event.RunID)
		if err != nil {
			return taskpkg.EventRecord{}, err
		}
		if runTaskID != event.TaskID {
			return taskpkg.EventRecord{}, fmt.Errorf(
				"%w: task_event.run_id %q does not belong to task %q",
				taskpkg.ErrValidation,
				event.RunID,
				event.TaskID,
			)
		}
	}

	nextSequence, err := nextTaskEventSequenceWithExecutor(ctx, exec)
	if err != nil {
		return taskpkg.EventRecord{}, err
	}
	if err := sqlcgen.New(exec).InsertTaskEvent(ctx, sqlcgen.InsertTaskEventParams{
		EventSeq:    nextSequence,
		ID:          event.ID,
		TaskID:      event.TaskID,
		RunID:       sql.NullString{String: event.RunID, Valid: strings.TrimSpace(event.RunID) != ""},
		EventType:   event.EventType,
		ActorKind:   string(event.Actor.Kind),
		ActorID:     event.Actor.Ref,
		OriginKind:  string(event.Origin.Kind),
		OriginRef:   event.Origin.Ref,
		PayloadJson: sql.NullString{String: string(event.Payload), Valid: len(event.Payload) > 0},
		Timestamp:   store.FormatTimestamp(event.Timestamp),
	}); err != nil {
		return taskpkg.EventRecord{}, fmt.Errorf("store: create task event %q: %w", event.ID, err)
	}
	if err := persistNetworkTaskStatusProjectionWithExecutor(ctx, exec, event); err != nil {
		return taskpkg.EventRecord{}, err
	}
	collectTaskEvent(exec, taskpkg.EventRecord{Sequence: nextSequence, Event: event})

	return taskpkg.EventRecord{Sequence: nextSequence, Event: event}, nil
}

func appendTaskEventPayloadWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
	runID string,
	eventType string,
	actor taskpkg.ActorContext,
	timestamp time.Time,
	payload any,
) error {
	_, err := appendTaskEventPayloadRecordWithExecutor(
		ctx,
		exec,
		taskID,
		runID,
		eventType,
		actor,
		timestamp,
		payload,
	)
	return err
}

func appendTaskEventPayloadRecordWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
	runID string,
	eventType string,
	actor taskpkg.ActorContext,
	timestamp time.Time,
	payload any,
) (taskpkg.EventRecord, error) {
	if err := actor.Validate(); err != nil {
		return taskpkg.EventRecord{}, err
	}
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return taskpkg.EventRecord{}, fmt.Errorf("store: marshal task event %q payload: %w", eventType, err)
	}
	return appendTaskEventRecordWithExecutor(ctx, exec, EventRecordInsert{
		ID:        store.NewID("evt"),
		TaskID:    taskID,
		RunID:     runID,
		EventType: eventType,
		Actor:     actor.Actor,
		Origin:    actor.Origin,
		Payload:   rawPayload,
		Timestamp: timestamp,
	})
}

func ensureTaskEventTaskExists(ctx context.Context, exec taskSQLExecutor, taskID string) error {
	trimmedID, err := requireTaskValue(taskID, "task id")
	if err != nil {
		return err
	}

	if _, err := sqlcgen.New(exec).GetTaskID(ctx, trimmedID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return taskpkg.ErrTaskNotFound
		}
		return fmt.Errorf("store: lookup task %q: %w", trimmedID, err)
	}

	return nil
}

func taskEventRunTaskID(ctx context.Context, exec taskSQLExecutor, runID string) (string, error) {
	trimmedID, err := requireTaskValue(runID, "task run id")
	if err != nil {
		return "", err
	}

	taskID, err := sqlcgen.New(exec).GetTaskRunTaskID(ctx, trimmedID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", taskpkg.ErrTaskRunNotFound
		}
		return "", fmt.Errorf("store: lookup task run %q: %w", trimmedID, err)
	}
	return taskNullStringValue(taskID), nil
}
