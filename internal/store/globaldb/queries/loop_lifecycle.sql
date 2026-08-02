-- name: ListLoopNodeControls :many
SELECT control.* FROM loop_node_controls AS control
JOIN loop_runs AS run ON run.id = control.loop_run_id
WHERE run.workspace_id = sqlc.arg(workspace_id) AND control.loop_run_id = sqlc.arg(loop_run_id)
ORDER BY control.node_id ASC;

-- name: ListLoopNodeAttempts :many
SELECT attempt.* FROM loop_node_attempts AS attempt
JOIN loop_runs AS run ON run.id = attempt.loop_run_id
WHERE run.workspace_id = sqlc.arg(workspace_id) AND attempt.loop_run_id = sqlc.arg(loop_run_id)
ORDER BY attempt.generation ASC, attempt.node_id ASC, attempt.item_index ASC, attempt.attempt ASC;

-- name: ListLoopNodeWaits :many
SELECT wait_row.* FROM loop_node_waits AS wait_row
JOIN loop_runs AS run ON run.id = wait_row.loop_run_id
WHERE run.workspace_id = sqlc.arg(workspace_id) AND wait_row.loop_run_id = sqlc.arg(loop_run_id)
ORDER BY wait_row.generation ASC, wait_row.node_id ASC, wait_row.item_index ASC;

-- name: GetLoopAdmissionClaim :one
SELECT * FROM loop_admission_claims
WHERE workspace_id = sqlc.arg(workspace_id)
  AND loop_name = sqlc.arg(loop_name)
  AND source_key = sqlc.arg(source_key)
  AND event_key = sqlc.arg(event_key);

-- name: ListLoopEffectOutbox :many
SELECT effect.* FROM loop_effect_outbox AS effect
JOIN loop_runs AS run ON run.id = effect.loop_run_id
WHERE run.workspace_id = sqlc.arg(workspace_id) AND effect.loop_run_id = sqlc.arg(loop_run_id)
ORDER BY effect.created_at ASC, effect.delivery_id ASC;
