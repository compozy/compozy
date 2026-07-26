package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

// CreateTaskEvent inserts one immutable task audit event.
func (g *TaskRepo) CreateTaskEvent(ctx context.Context, event taskpkg.Event) error {
	if err := g.checkReady(ctx, "create task event"); err != nil {
		return err
	}

	normalized, err := g.normalizeTaskEventForCreate(event)
	if err != nil {
		return err
	}

	return g.withTaskImmediateTransaction(ctx, "create task event", func(exec taskSQLExecutor) error {
		return appendTaskEventWithExecutor(ctx, exec, EventRecordInsert{
			ID:        normalized.ID,
			TaskID:    normalized.TaskID,
			RunID:     normalized.RunID,
			EventType: normalized.EventType,
			Actor:     normalized.Actor,
			Origin:    normalized.Origin,
			Payload:   normalized.Payload,
			Timestamp: normalized.Timestamp,
		})
	})
}

// ListTaskEvents returns persisted audit events that match the supplied filters.
func (g *TaskRepo) ListTaskEvents(ctx context.Context, query taskpkg.EventQuery) ([]taskpkg.Event, error) {
	if err := g.checkReady(ctx, "list task events"); err != nil {
		return nil, err
	}
	if err := query.Validate("task_event_query"); err != nil {
		return nil, err
	}

	normalized := normalizeTaskEventQuery(query)
	rows, err := g.queries.ListTaskEvents(ctx, sqlcgen.ListTaskEventsParams{
		TaskID:      normalized.TaskID,
		RunID:       normalized.RunID,
		EventType:   normalized.EventType,
		ResultLimit: taskQueryLimit(normalized.Limit),
	})
	if err != nil {
		return nil, fmt.Errorf("store: query task events: %w", err)
	}

	events := make([]taskpkg.Event, 0, len(rows))
	for _, row := range rows {
		event, mapErr := taskEventFromGenerated(row)
		if mapErr != nil {
			return nil, mapErr
		}
		events = append(events, event)
	}

	return events, nil
}

// TaskWakeEventExists reports whether the task ledger already contains a delivered or suppressed wake identity.
func (g *TaskRepo) TaskWakeEventExists(
	ctx context.Context,
	taskID string,
	wakeEventID string,
) (bool, error) {
	if err := g.checkReady(ctx, "lookup task wake event"); err != nil {
		return false, err
	}
	trimmedTaskID, err := requireTaskValue(taskID, "task wake event task id")
	if err != nil {
		return false, err
	}
	trimmedWakeEventID, err := requireTaskValue(wakeEventID, "task wake event id")
	if err != nil {
		return false, err
	}
	recorded, err := g.queries.TaskWakeEventExists(ctx, sqlcgen.TaskWakeEventExistsParams{
		TaskID:      trimmedTaskID,
		WakeEventID: sql.NullString{String: trimmedWakeEventID, Valid: true},
	})
	if err != nil {
		return false, fmt.Errorf(
			"store: lookup task %q wake event %q: %w",
			trimmedTaskID,
			trimmedWakeEventID,
			err,
		)
	}
	return recorded, nil
}

// GetTaskEventRecord returns one persisted task event plus its stable row sequence.
func (g *TaskRepo) GetTaskEventRecord(ctx context.Context, eventID string) (taskpkg.EventRecord, error) {
	if err := g.checkReady(ctx, "get task event record"); err != nil {
		return taskpkg.EventRecord{}, err
	}

	trimmedEventID, err := requireTaskValue(eventID, "task event id")
	if err != nil {
		return taskpkg.EventRecord{}, err
	}

	row, err := g.queries.GetTaskEventRecord(ctx, trimmedEventID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return taskpkg.EventRecord{}, taskpkg.ErrTaskEventNotFound
		}
		return taskpkg.EventRecord{}, fmt.Errorf("store: get task event record %q: %w", trimmedEventID, err)
	}
	return taskEventRecordFromGenerated(row)
}

// ListTaskEventRecords returns persisted task events ordered by stable sequence for live replay.
func (g *TaskRepo) ListTaskEventRecords(
	ctx context.Context,
	query taskpkg.EventRecordQuery,
) ([]taskpkg.EventRecord, error) {
	if err := g.checkReady(ctx, "list task event records"); err != nil {
		return nil, err
	}
	if err := query.Validate("task_event_record_query"); err != nil {
		return nil, err
	}

	if query.Descending {
		rows, err := g.queries.ListTaskEventRecordsDescending(ctx, sqlcgen.ListTaskEventRecordsDescendingParams{
			TaskID: strings.TrimSpace(query.TaskID), AfterSequence: query.AfterSequence,
			ResultLimit: taskQueryLimit(query.Limit),
		})
		if err != nil {
			return nil, fmt.Errorf("store: query task event records: %w", err)
		}
		records := make([]taskpkg.EventRecord, 0, len(rows))
		for _, row := range rows {
			record, mapErr := descendingTaskEventRecordFromGenerated(row)
			if mapErr != nil {
				return nil, mapErr
			}
			records = append(records, record)
		}
		return records, nil
	}

	rows, err := g.queries.ListTaskEventRecordsAscending(ctx, sqlcgen.ListTaskEventRecordsAscendingParams{
		TaskID: strings.TrimSpace(query.TaskID), AfterSequence: query.AfterSequence,
		ResultLimit: taskQueryLimit(query.Limit),
	})
	if err != nil {
		return nil, fmt.Errorf("store: query task event records: %w", err)
	}
	records := make([]taskpkg.EventRecord, 0, len(rows))
	for _, row := range rows {
		record, mapErr := ascendingTaskEventRecordFromGenerated(row)
		if mapErr != nil {
			return nil, mapErr
		}
		records = append(records, record)
	}
	return records, nil
}
