-- name: GetRunStarvation :one
SELECT run_id, wake_count, first_starved_at, last_wake_at, escalation_tier,
       spawn_requested_at, starved_event_at, updated_at
FROM task_run_starvation
WHERE run_id = sqlc.arg(run_id);

-- name: ListRunStarvation :many
SELECT run_id, wake_count, first_starved_at, last_wake_at, escalation_tier,
       spawn_requested_at, starved_event_at, updated_at
FROM task_run_starvation;

-- name: UpsertRunStarvation :exec
INSERT INTO task_run_starvation (
  run_id, wake_count, first_starved_at, last_wake_at, escalation_tier,
  spawn_requested_at, starved_event_at, updated_at
) VALUES (
  sqlc.arg(run_id), sqlc.arg(wake_count), sqlc.arg(first_starved_at), sqlc.narg(last_wake_at),
  sqlc.arg(escalation_tier), sqlc.narg(spawn_requested_at), sqlc.narg(starved_event_at),
  sqlc.arg(updated_at)
)
ON CONFLICT(run_id) DO UPDATE SET
  wake_count = excluded.wake_count,
  first_starved_at = excluded.first_starved_at,
  last_wake_at = excluded.last_wake_at,
  escalation_tier = excluded.escalation_tier,
  spawn_requested_at = excluded.spawn_requested_at,
  starved_event_at = excluded.starved_event_at,
  updated_at = excluded.updated_at;

-- name: DeleteRunStarvation :exec
DELETE FROM task_run_starvation WHERE run_id = sqlc.arg(run_id);
