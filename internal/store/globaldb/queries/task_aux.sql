-- name: GetTaskTriageState :one
SELECT task_id, actor_kind, actor_id, is_read, archived, dismissed, last_seen_activity_at, updated_at
FROM task_triage_state
WHERE task_id = sqlc.arg(task_id)
  AND actor_kind = sqlc.arg(actor_kind)
  AND actor_id = sqlc.arg(actor_id);

-- name: UpsertTaskTriageState :exec
INSERT INTO task_triage_state (
  task_id, actor_kind, actor_id, is_read, archived, dismissed, last_seen_activity_at, updated_at
) VALUES (
  sqlc.arg(task_id), sqlc.arg(actor_kind), sqlc.arg(actor_id), sqlc.arg(is_read),
  sqlc.arg(archived), sqlc.arg(dismissed), sqlc.narg(last_seen_activity_at), sqlc.arg(updated_at)
)
ON CONFLICT(task_id, actor_kind, actor_id) DO UPDATE SET
  is_read = excluded.is_read,
  archived = excluded.archived,
  dismissed = excluded.dismissed,
  last_seen_activity_at = excluded.last_seen_activity_at,
  updated_at = excluded.updated_at;

-- name: ListTaskTriageStates :many
SELECT task_id, actor_kind, actor_id, is_read, archived, dismissed, last_seen_activity_at, updated_at
FROM task_triage_state
WHERE actor_kind = sqlc.arg(actor_kind) AND actor_id = sqlc.arg(actor_id)
ORDER BY updated_at DESC, task_id ASC;

-- name: InsertTaskDependency :exec
INSERT INTO task_dependencies (task_id, depends_on_task_id, kind, created_at)
VALUES (sqlc.arg(task_id), sqlc.arg(depends_on_task_id), sqlc.arg(kind), sqlc.arg(created_at));

-- name: DeleteTaskDependency :execrows
DELETE FROM task_dependencies
WHERE task_id = sqlc.arg(task_id) AND depends_on_task_id = sqlc.arg(depends_on_task_id);

-- name: ListTaskDependencies :many
SELECT task_id, depends_on_task_id, kind, created_at
FROM task_dependencies
WHERE task_id = sqlc.arg(task_id)
ORDER BY created_at ASC, depends_on_task_id ASC;

-- name: ListTaskDependents :many
SELECT task_id, depends_on_task_id, kind, created_at
FROM task_dependencies
WHERE depends_on_task_id = sqlc.arg(depends_on_task_id)
ORDER BY created_at ASC, task_id ASC;

-- name: CountTaskDependencies :one
SELECT COUNT(1) FROM task_dependencies WHERE task_id = sqlc.arg(task_id);

-- name: TaskDependencyExists :one
SELECT EXISTS(
  SELECT 1 FROM task_dependencies
  WHERE task_id = sqlc.arg(task_id)
    AND depends_on_task_id = sqlc.arg(depends_on_task_id)
    AND kind = sqlc.arg(kind)
);

-- name: InsertTaskEvent :exec
INSERT INTO task_events (
  event_seq, id, task_id, run_id, event_type, actor_kind, actor_id,
  origin_kind, origin_ref, payload_json, timestamp
) VALUES (
  sqlc.arg(event_seq), sqlc.arg(id), sqlc.arg(task_id), sqlc.narg(run_id),
  sqlc.arg(event_type), sqlc.arg(actor_kind), sqlc.arg(actor_id),
  sqlc.arg(origin_kind), sqlc.arg(origin_ref), sqlc.narg(payload_json), sqlc.arg(timestamp)
);

-- name: GetTaskEventRecord :one
SELECT event_seq, id, task_id, run_id, event_type, actor_kind, actor_id,
       origin_kind, origin_ref, payload_json, timestamp
FROM task_events
WHERE id = sqlc.arg(id);

-- name: ListTaskEvents :many
SELECT id, task_id, run_id, event_type, actor_kind, actor_id,
       origin_kind, origin_ref, payload_json, timestamp
FROM task_events
WHERE (CAST(sqlc.arg(task_id) AS TEXT) = '' OR task_id = CAST(sqlc.arg(task_id) AS TEXT))
  AND (CAST(sqlc.arg(run_id) AS TEXT) = '' OR run_id = CAST(sqlc.arg(run_id) AS TEXT))
  AND (CAST(sqlc.arg(event_type) AS TEXT) = '' OR event_type = CAST(sqlc.arg(event_type) AS TEXT))
ORDER BY timestamp DESC, id DESC
LIMIT sqlc.arg(result_limit);

-- name: TaskWakeEventExists :one
SELECT EXISTS(
  SELECT 1
  FROM task_events
  WHERE task_id = sqlc.arg(task_id)
    AND event_type IN ('task.wake.delivered', 'task.wake.suppressed')
    AND json_extract(payload_json, '$.wake_event_id') = sqlc.arg(wake_event_id)
);

-- name: ListTaskEventRecordsAscending :many
SELECT event_seq, id, task_id, run_id, event_type, actor_kind, actor_id,
       origin_kind, origin_ref, payload_json, timestamp
FROM task_events
WHERE (CAST(sqlc.arg(task_id) AS TEXT) = '' OR task_id = CAST(sqlc.arg(task_id) AS TEXT))
  AND (CAST(sqlc.arg(after_sequence) AS INTEGER) <= 0 OR event_seq > CAST(sqlc.arg(after_sequence) AS INTEGER))
ORDER BY event_seq ASC
LIMIT sqlc.arg(result_limit);

-- name: ListTaskEventRecordsDescending :many
SELECT event_seq, id, task_id, run_id, event_type, actor_kind, actor_id,
       origin_kind, origin_ref, payload_json, timestamp
FROM task_events
WHERE (CAST(sqlc.arg(task_id) AS TEXT) = '' OR task_id = CAST(sqlc.arg(task_id) AS TEXT))
  AND (CAST(sqlc.arg(after_sequence) AS INTEGER) <= 0 OR event_seq > CAST(sqlc.arg(after_sequence) AS INTEGER))
ORDER BY event_seq DESC
LIMIT sqlc.arg(result_limit);

-- name: MaxTaskEventSequence :one
SELECT CAST(COALESCE(MAX(event_seq), 0) AS INTEGER) FROM task_events;

-- name: GetTaskRunTaskID :one
SELECT task_id FROM task_runs WHERE id = sqlc.arg(id);

-- name: InsertTaskRunIdempotency :execrows
INSERT INTO task_run_idempotency (
  idempotency_key, origin_kind, origin_ref, run_id, created_at
) VALUES (
  sqlc.arg(idempotency_key), sqlc.arg(origin_kind), sqlc.arg(origin_ref),
  sqlc.arg(run_id), sqlc.arg(created_at)
)
ON CONFLICT(idempotency_key, origin_kind, origin_ref) DO NOTHING;

-- name: GetTaskRunIdempotency :one
SELECT idempotency_key, run_id, origin_kind, origin_ref, created_at
FROM task_run_idempotency
WHERE idempotency_key = sqlc.arg(idempotency_key)
  AND origin_kind = sqlc.arg(origin_kind)
  AND origin_ref = sqlc.arg(origin_ref);

-- name: GetTaskRunIDByIdempotency :one
SELECT run_id
FROM task_run_idempotency
WHERE idempotency_key = sqlc.arg(idempotency_key)
  AND origin_kind = sqlc.arg(origin_kind)
  AND origin_ref = sqlc.arg(origin_ref);

-- name: MaxTaskRunAttempt :one
SELECT CAST(COALESCE(MAX(attempt), 0) AS INTEGER)
FROM task_runs
WHERE task_id = sqlc.arg(task_id);

-- name: GetOpenTaskRunID :one
SELECT id
FROM task_runs
WHERE task_id = sqlc.arg(task_id)
  AND status NOT IN (sqlc.arg(completed_status), sqlc.arg(failed_status), sqlc.arg(canceled_status))
  AND (CAST(sqlc.arg(designation_group_id) AS TEXT) = ''
       OR COALESCE(designation_group_id, '') <> CAST(sqlc.arg(designation_group_id) AS TEXT))
ORDER BY queued_at DESC, id DESC
LIMIT 1;
