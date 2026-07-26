package globaldb

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

func taskTriageFromGenerated(row sqlcgen.TaskTriageState) (taskpkg.TriageState, error) {
	record := taskpkg.TriageState{
		TaskID: row.TaskID,
		Actor: taskpkg.ActorIdentity{
			Kind: taskpkg.ActorKind(strings.TrimSpace(row.ActorKind)),
			Ref:  row.ActorID,
		},
		Read:      row.IsRead,
		Archived:  row.Archived,
		Dismissed: row.Dismissed,
	}
	if err := assignNullableTaskTimestamp(&record.LastSeenActivityAt, row.LastSeenActivityAt); err != nil {
		return taskpkg.TriageState{}, fmt.Errorf("store: parse task triage last_seen_activity_at: %w", err)
	}
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return taskpkg.TriageState{}, fmt.Errorf("store: parse task triage updated_at: %w", err)
	}
	record.UpdatedAt = updatedAt
	record = normalizeTaskTriageStateRecord(record)
	if err := record.Validate(); err != nil {
		return taskpkg.TriageState{}, err
	}
	return record, nil
}

func taskDependencyFromGenerated(row sqlcgen.TaskDependency) (taskpkg.Dependency, error) {
	createdAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return taskpkg.Dependency{}, err
	}
	record := taskpkg.Dependency{
		TaskID:          row.TaskID,
		DependsOnTaskID: row.DependsOnTaskID,
		Kind:            taskpkg.DependencyKind(strings.TrimSpace(row.Kind)),
		CreatedAt:       createdAt,
	}
	record = normalizeTaskDependencyRecord(record)
	if err := record.Validate(); err != nil {
		return taskpkg.Dependency{}, err
	}
	return record, nil
}

func taskEventFromGenerated(row sqlcgen.ListTaskEventsRow) (taskpkg.Event, error) {
	return taskEventFromGeneratedFields(
		row.ID, row.TaskID, row.RunID, row.EventType, row.ActorKind, row.ActorID,
		row.OriginKind, row.OriginRef, row.PayloadJson, row.Timestamp,
	)
}

func taskEventRecordFromGenerated(row sqlcgen.GetTaskEventRecordRow) (taskpkg.EventRecord, error) {
	return taskEventRecordFromGeneratedFields(
		row.EventSeq, row.ID, row.TaskID, row.RunID, row.EventType, row.ActorKind,
		row.ActorID, row.OriginKind, row.OriginRef, row.PayloadJson, row.Timestamp,
	)
}

func ascendingTaskEventRecordFromGenerated(
	row sqlcgen.ListTaskEventRecordsAscendingRow,
) (taskpkg.EventRecord, error) {
	return taskEventRecordFromGeneratedFields(
		row.EventSeq, row.ID, row.TaskID, row.RunID, row.EventType, row.ActorKind,
		row.ActorID, row.OriginKind, row.OriginRef, row.PayloadJson, row.Timestamp,
	)
}

func descendingTaskEventRecordFromGenerated(
	row sqlcgen.ListTaskEventRecordsDescendingRow,
) (taskpkg.EventRecord, error) {
	return taskEventRecordFromGeneratedFields(
		row.EventSeq, row.ID, row.TaskID, row.RunID, row.EventType, row.ActorKind,
		row.ActorID, row.OriginKind, row.OriginRef, row.PayloadJson, row.Timestamp,
	)
}

func taskEventRecordFromGeneratedFields(
	sequence int64,
	id string,
	taskID string,
	runID sql.NullString,
	eventType string,
	actorKind string,
	actorID string,
	originKind string,
	originRef string,
	payloadJSON sql.NullString,
	timestamp string,
) (taskpkg.EventRecord, error) {
	event, err := taskEventFromGeneratedFields(
		id, taskID, runID, eventType, actorKind, actorID, originKind, originRef, payloadJSON, timestamp,
	)
	if err != nil {
		return taskpkg.EventRecord{}, err
	}
	if sequence <= 0 {
		return taskpkg.EventRecord{}, fmt.Errorf(
			"%w: task_event_record.sequence must be positive",
			taskpkg.ErrValidation,
		)
	}
	return taskpkg.EventRecord{Sequence: sequence, Event: event}, nil
}

func taskEventFromGeneratedFields(
	id string,
	taskID string,
	runID sql.NullString,
	eventType string,
	actorKind string,
	actorID string,
	originKind string,
	originRef string,
	payloadJSON sql.NullString,
	timestamp string,
) (taskpkg.Event, error) {
	payload, err := decodeTaskJSON(payloadJSON, "task_event.payload_json")
	if err != nil {
		return taskpkg.Event{}, err
	}
	parsedTimestamp, err := store.ParseTimestamp(timestamp)
	if err != nil {
		return taskpkg.Event{}, err
	}
	record := normalizeTaskEventRecord(taskpkg.Event{
		ID:        id,
		TaskID:    taskID,
		RunID:     taskNullStringValue(runID),
		EventType: eventType,
		Actor: taskpkg.ActorIdentity{
			Kind: taskpkg.ActorKind(strings.TrimSpace(actorKind)),
			Ref:  actorID,
		},
		Origin: taskpkg.Origin{
			Kind: taskpkg.OriginKind(strings.TrimSpace(originKind)),
			Ref:  originRef,
		},
		Payload:   payload,
		Timestamp: parsedTimestamp,
	})
	if err := record.Validate(); err != nil {
		return taskpkg.Event{}, err
	}
	return record, nil
}

func taskRunIdempotencyFromGenerated(
	row sqlcgen.GetTaskRunIdempotencyRow,
) (taskpkg.RunIdempotency, error) {
	createdAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return taskpkg.RunIdempotency{}, err
	}
	record := normalizeTaskRunIdempotencyRecord(taskpkg.RunIdempotency{
		IdempotencyKey: row.IdempotencyKey,
		RunID:          row.RunID,
		Origin: taskpkg.Origin{
			Kind: taskpkg.OriginKind(strings.TrimSpace(row.OriginKind)),
			Ref:  row.OriginRef,
		},
		CreatedAt: createdAt,
	})
	if err := record.Validate(); err != nil {
		return taskpkg.RunIdempotency{}, err
	}
	return record, nil
}
