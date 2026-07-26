package globaldb

import (
	"strings"

	taskpkg "github.com/compozy/agh/internal/task"
)

func normalizeTaskTriageLookup(
	taskID string,
	actor taskpkg.ActorIdentity,
) (string, taskpkg.ActorIdentity, error) {
	trimmedTaskID, err := requireTaskValue(taskID, "task triage task id")
	if err != nil {
		return "", taskpkg.ActorIdentity{}, err
	}

	normalizedActor, err := normalizeTaskTriageActor(actor)
	if err != nil {
		return "", taskpkg.ActorIdentity{}, err
	}

	return trimmedTaskID, normalizedActor, nil
}

func normalizeTaskTriageActor(actor taskpkg.ActorIdentity) (taskpkg.ActorIdentity, error) {
	normalizedActor := taskpkg.ActorIdentity{
		Kind: actor.Kind.Normalize(),
		Ref:  strings.TrimSpace(actor.Ref),
	}
	if err := normalizedActor.Validate("task_triage_state.actor"); err != nil {
		return taskpkg.ActorIdentity{}, err
	}
	return normalizedActor, nil
}

func normalizeTaskDependencyRecord(record taskpkg.Dependency) taskpkg.Dependency {
	normalized := record
	normalized.TaskID = strings.TrimSpace(normalized.TaskID)
	normalized.DependsOnTaskID = strings.TrimSpace(normalized.DependsOnTaskID)
	normalized.Kind = normalized.Kind.Normalize()
	if !normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = normalized.CreatedAt.UTC()
	}
	return normalized
}

func normalizeTaskTriageStateRecord(record taskpkg.TriageState) taskpkg.TriageState {
	normalized := record
	normalized.TaskID = strings.TrimSpace(normalized.TaskID)
	normalized.Actor.Kind = normalized.Actor.Kind.Normalize()
	normalized.Actor.Ref = strings.TrimSpace(normalized.Actor.Ref)
	if !normalized.LastSeenActivityAt.IsZero() {
		normalized.LastSeenActivityAt = normalized.LastSeenActivityAt.UTC()
	}
	if !normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = normalized.UpdatedAt.UTC()
	}
	return normalized
}

func normalizeTaskEventRecord(record taskpkg.Event) taskpkg.Event {
	normalized := record
	normalized.ID = strings.TrimSpace(normalized.ID)
	normalized.TaskID = strings.TrimSpace(normalized.TaskID)
	normalized.RunID = strings.TrimSpace(normalized.RunID)
	normalized.EventType = strings.TrimSpace(normalized.EventType)
	normalized.Actor.Kind = normalized.Actor.Kind.Normalize()
	normalized.Actor.Ref = strings.TrimSpace(normalized.Actor.Ref)
	normalized.Origin.Kind = normalized.Origin.Kind.Normalize()
	normalized.Origin.Ref = strings.TrimSpace(normalized.Origin.Ref)
	normalized.Payload = normalizeTaskJSON(normalized.Payload)
	if !normalized.Timestamp.IsZero() {
		normalized.Timestamp = normalized.Timestamp.UTC()
	}
	return normalized
}

func normalizeTaskEventQuery(query taskpkg.EventQuery) taskpkg.EventQuery {
	normalized := query
	normalized.TaskID = strings.TrimSpace(normalized.TaskID)
	normalized.RunID = strings.TrimSpace(normalized.RunID)
	normalized.EventType = strings.TrimSpace(normalized.EventType)
	return normalized
}

func taskQueryLimit(limit int) int64 {
	if limit <= 0 {
		return -1
	}
	return int64(limit)
}

func normalizeTaskRunIdempotencyLookup(key string, origin taskpkg.Origin) (string, taskpkg.Origin, error) {
	trimmedKey, err := requireTaskValue(key, "task run idempotency key")
	if err != nil {
		return "", taskpkg.Origin{}, err
	}

	normalizedOrigin := taskpkg.Origin{
		Kind: origin.Kind.Normalize(),
		Ref:  strings.TrimSpace(origin.Ref),
	}
	if err := normalizedOrigin.Validate("task_run_idempotency.origin"); err != nil {
		return "", taskpkg.Origin{}, err
	}

	return trimmedKey, normalizedOrigin, nil
}

func normalizeTaskRunIdempotencyRecord(record taskpkg.RunIdempotency) taskpkg.RunIdempotency {
	normalized := record
	normalized.IdempotencyKey = strings.TrimSpace(normalized.IdempotencyKey)
	normalized.RunID = strings.TrimSpace(normalized.RunID)
	normalized.Origin.Kind = normalized.Origin.Kind.Normalize()
	normalized.Origin.Ref = strings.TrimSpace(normalized.Origin.Ref)
	if !normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = normalized.CreatedAt.UTC()
	}
	return normalized
}

func taskOriginsEqual(left taskpkg.Origin, right taskpkg.Origin) bool {
	return left.Kind.Normalize() == right.Kind.Normalize() &&
		strings.TrimSpace(left.Ref) == strings.TrimSpace(right.Ref)
}
