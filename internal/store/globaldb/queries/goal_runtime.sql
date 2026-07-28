-- name: ClaimGoalSessionOutbox :many
SELECT id, event_id, workspace_id, origin_session_id, loop_run_id, bound_session_id,
       cause, created_at, delivered_at
FROM loop_goal_session_outbox
WHERE delivered_at IS NULL
ORDER BY id ASC
LIMIT sqlc.arg(claim_limit);

-- name: AcknowledgeGoalSessionOutbox :exec
UPDATE loop_goal_session_outbox
SET delivered_at = CAST(sqlc.arg(delivered_at) AS TEXT)
WHERE event_id = sqlc.arg(event_id) AND delivered_at IS NULL;

-- name: EnqueueGoalSessionOutbox :exec
INSERT INTO loop_goal_session_outbox (
    event_id, workspace_id, origin_session_id, loop_run_id,
    bound_session_id, cause, created_at
) VALUES (
    sqlc.arg(event_id), sqlc.arg(workspace_id), sqlc.arg(origin_session_id),
    sqlc.arg(loop_run_id), sqlc.narg(bound_session_id), sqlc.arg(cause), CAST(sqlc.arg(created_at) AS TEXT)
)
ON CONFLICT(event_id) DO NOTHING;

-- name: GetGoalRunOrigin :one
SELECT origin_kind, origin_session_id
FROM loop_runs
WHERE id = sqlc.arg(id) AND workspace_id = sqlc.arg(workspace_id);

-- name: ValidateGoalSessionWorkspace :one
SELECT 1
FROM sessions
WHERE id = sqlc.arg(id) AND workspace_id = sqlc.arg(workspace_id);

-- name: GetGoalSessionOutboxByEventID :one
SELECT id, event_id, workspace_id, origin_session_id, loop_run_id, bound_session_id,
       cause, created_at, delivered_at
FROM loop_goal_session_outbox
WHERE event_id = sqlc.arg(event_id);

-- name: ReadGoalSessionProjection :one
SELECT id, status, definition_digest, origin_session_id, goal_context_nudge_ratio, goal_cleared_at
FROM loop_runs
WHERE workspace_id = sqlc.arg(workspace_id)
  AND origin_kind = 'session'
  AND origin_session_id = sqlc.arg(origin_session_id)
ORDER BY created_at DESC, rowid DESC
LIMIT 1;

-- name: ReadLatestGoalSessionCheckpoint :one
SELECT *
FROM loop_goal_checkpoints
WHERE loop_run_id = sqlc.arg(loop_run_id)
ORDER BY generation DESC, updated_at DESC, node_id ASC, item_index ASC
LIMIT 1;

-- name: ReadLatestGoalSessionVerdict :one
SELECT *
FROM loop_goal_turns
WHERE loop_run_id = sqlc.arg(loop_run_id) AND verdict_outcome IS NOT NULL
ORDER BY seq DESC
LIMIT 1;

-- name: ClaimGoalSessionCleanup :many
SELECT cleanup.id, cleanup.cleanup_id, cleanup.workspace_id, cleanup.loop_run_id,
       cleanup.handle, cleanup.binding_epoch, cleanup.session_id, cleanup.cause,
       cleanup.created_at, cleanup.completed_at
FROM loop_goal_session_cleanup AS cleanup
JOIN loop_session_bindings AS binding
  ON binding.loop_run_id = cleanup.loop_run_id
 AND binding.handle = cleanup.handle
 AND binding.binding_epoch = cleanup.binding_epoch
WHERE cleanup.completed_at IS NULL
  AND NOT (binding.state = 'failed' AND binding.failure_code = sqlc.arg(unsettled_failure_code))
ORDER BY cleanup.id ASC
LIMIT sqlc.arg(claim_limit);

-- name: ReconcileGoalSessionCleanup :exec
UPDATE loop_session_bindings
SET failure_code = sqlc.arg(settled_failure_code)
WHERE state = 'failed' AND failure_code = sqlc.arg(unsettled_failure_code)
  AND EXISTS (
      SELECT 1 FROM loop_goal_session_cleanup AS cleanup
      WHERE cleanup.loop_run_id = loop_session_bindings.loop_run_id
        AND cleanup.handle = loop_session_bindings.handle
        AND cleanup.binding_epoch = loop_session_bindings.binding_epoch
        AND cleanup.completed_at IS NULL
  );

-- name: AcknowledgeGoalSessionCleanup :execrows
UPDATE loop_goal_session_cleanup
SET completed_at = CAST(sqlc.arg(completed_at) AS TEXT)
WHERE cleanup_id = sqlc.arg(cleanup_id) AND completed_at IS NULL;

-- name: EnqueueGoalSessionCleanup :exec
INSERT INTO loop_goal_session_cleanup (
    cleanup_id, workspace_id, loop_run_id, handle, binding_epoch, session_id, cause, created_at
) VALUES (
    sqlc.arg(cleanup_id), sqlc.arg(workspace_id), sqlc.arg(loop_run_id), sqlc.arg(handle),
    sqlc.arg(binding_epoch), sqlc.arg(session_id), sqlc.arg(cause), CAST(sqlc.arg(created_at) AS TEXT)
)
ON CONFLICT(cleanup_id) DO NOTHING;

-- name: GetGoalSessionCleanup :one
SELECT id, cleanup_id, workspace_id, loop_run_id, handle, binding_epoch,
       session_id, cause, created_at, completed_at
FROM loop_goal_session_cleanup
WHERE cleanup_id = sqlc.arg(cleanup_id);
