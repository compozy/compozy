-- name: ClearTaskNeedsAttention :execrows
UPDATE tasks
SET needs_attention_reason = NULL,
    needs_attention_at = NULL,
    needs_attention_by_kind = NULL,
    needs_attention_by_ref = NULL,
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND needs_attention_at IS NOT NULL;

-- name: UpdateTaskStatusChokepoint :execrows
UPDATE tasks
SET status = sqlc.arg(to_status)
WHERE id = sqlc.arg(id) AND status = sqlc.arg(from_status);

-- name: GetTaskStatus :one
SELECT status FROM tasks WHERE id = sqlc.arg(id);

-- name: SetTaskCurrentRunProjection :execrows
UPDATE tasks
SET current_run_id = sqlc.arg(run_id)
WHERE id = sqlc.arg(task_id)
  AND (current_run_id IS NULL OR current_run_id = '' OR current_run_id = sqlc.arg(run_id));

-- name: ClearTaskCurrentRunProjection :execrows
UPDATE tasks
SET current_run_id = NULL
WHERE id = sqlc.arg(task_id) AND current_run_id = sqlc.arg(run_id);

-- name: GetTaskCurrentRunProjection :one
SELECT current_run_id FROM tasks WHERE id = sqlc.arg(id);

-- name: PauseTask :execrows
UPDATE tasks
SET paused = 1,
    paused_by = sqlc.arg(paused_by),
    paused_at = sqlc.arg(paused_at),
    paused_reason = sqlc.arg(paused_reason),
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);

-- name: ResumeTask :execrows
UPDATE tasks
SET paused = 0,
    paused_by = '',
    paused_at = NULL,
    paused_reason = '',
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);

-- name: CountPausedTasks :one
SELECT COUNT(1) FROM tasks WHERE paused = 1;

-- name: CountActiveTaskRunClaims :one
SELECT COUNT(1)
FROM task_runs
WHERE status IN (sqlc.arg(claimed_status), sqlc.arg(starting_status), sqlc.arg(running_status));

-- name: CountAllQueuedTaskRuns :one
SELECT COUNT(1)
FROM task_runs
WHERE status = sqlc.arg(queued_status);

-- name: CountNeedsAttentionTaskRuns :one
SELECT COUNT(1)
FROM task_runs
WHERE status = sqlc.arg(needs_attention_status);

-- name: GetSchedulerPause :one
SELECT paused, paused_by, paused_at, reason, updated_at
FROM scheduler_pause
WHERE id = 1;

-- name: SetSchedulerPaused :exec
INSERT INTO scheduler_pause (id, paused, paused_by, paused_at, reason, updated_at)
VALUES (
    1, 1, sqlc.arg(paused_by), sqlc.arg(paused_at),
    sqlc.arg(reason), sqlc.arg(updated_at)
)
ON CONFLICT(id) DO UPDATE SET
    paused = 1,
    paused_by = excluded.paused_by,
    paused_at = excluded.paused_at,
    reason = excluded.reason,
    updated_at = excluded.updated_at;

-- name: SetSchedulerResumed :exec
INSERT INTO scheduler_pause (id, paused, paused_by, paused_at, reason, updated_at)
VALUES (1, 0, '', NULL, '', sqlc.arg(updated_at))
ON CONFLICT(id) DO UPDATE SET
    paused = 0,
    paused_by = '',
    paused_at = NULL,
    reason = '',
    updated_at = excluded.updated_at;
