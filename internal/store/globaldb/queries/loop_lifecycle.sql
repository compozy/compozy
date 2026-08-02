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
  AND wait_row.claim_state IN ('waiting', 'intervention_required')
ORDER BY wait_row.generation ASC, wait_row.node_id ASC, wait_row.item_index ASC;

-- name: GetLoopAdmissionClaim :one
SELECT * FROM loop_admission_claims
WHERE workspace_id = sqlc.arg(workspace_id)
  AND loop_name = sqlc.arg(loop_name)
  AND source_key = sqlc.arg(source_key)
  AND event_key = sqlc.arg(event_key);

-- name: DeleteExpiredLoopAdmissionClaims :execrows
DELETE FROM loop_admission_claims
WHERE expires_at <= sqlc.arg(expires_before);

-- name: ListLoopEffectOutbox :many
SELECT effect.* FROM loop_effect_outbox AS effect
JOIN loop_runs AS run ON run.id = effect.loop_run_id
WHERE run.workspace_id = sqlc.arg(workspace_id) AND effect.loop_run_id = sqlc.arg(loop_run_id)
ORDER BY effect.created_at ASC, effect.delivery_id ASC;

-- name: InsertLoopEffectOutbox :exec
INSERT INTO loop_effect_outbox (
  loop_run_id, delivery_id, source_event_id, trigger, generation,
  node_id, item_index, entry_index, entry_json, state, attempts, created_at
) VALUES (
  sqlc.arg(loop_run_id), sqlc.arg(delivery_id), sqlc.arg(source_event_id), sqlc.arg(trigger),
  sqlc.arg(generation), sqlc.arg(node_id), sqlc.arg(item_index), sqlc.arg(entry_index),
  sqlc.arg(entry_json), 'pending', 0, sqlc.arg(created_at)
);

-- name: ListPendingLoopEffects :many
SELECT effect.loop_run_id, run.workspace_id, effect.delivery_id,
       effect.source_event_id, effect.trigger, effect.generation, effect.node_id,
       effect.item_index, effect.entry_index, effect.entry_json, effect.state,
       effect.attempts, effect.created_at, effect.delivered_at
FROM loop_effect_outbox AS effect
JOIN loop_runs AS run ON run.id = effect.loop_run_id
WHERE effect.state = 'pending'
ORDER BY effect.created_at ASC, effect.loop_run_id ASC, effect.delivery_id ASC
LIMIT sqlc.arg(page_limit);

-- name: ClosePendingLoopEffect :execrows
UPDATE loop_effect_outbox
SET state = sqlc.arg(state), attempts = attempts + 1, delivered_at = sqlc.arg(delivered_at)
WHERE loop_run_id = sqlc.arg(loop_run_id)
  AND delivery_id = sqlc.arg(delivery_id)
  AND state = 'pending';
