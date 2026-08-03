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

-- name: ListWaitingLoopNodeInventory :many
SELECT run.id AS loop_run_id, run.loop_name, wait_row.generation, wait_row.node_id,
  wait_row.item_index, wait_row.created_at AS state_at
FROM loop_node_waits AS wait_row
JOIN loop_runs AS run ON run.id = wait_row.loop_run_id
WHERE run.workspace_id = sqlc.arg(filter_workspace_id)
  AND wait_row.claim_state IN ('waiting', 'intervention_required')
  AND (CAST(sqlc.arg(filter_loop_name) AS TEXT) = '' OR run.loop_name = CAST(sqlc.arg(filter_loop_name) AS TEXT))
  AND (CAST(sqlc.arg(filter_run_id) AS TEXT) = '' OR run.id = CAST(sqlc.arg(filter_run_id) AS TEXT))
  AND (
    CAST(sqlc.arg(has_cursor) AS INTEGER) = 0
    OR wait_row.created_at > sqlc.arg(cursor_at)
    OR (wait_row.created_at = sqlc.arg(cursor_at) AND run.id > sqlc.arg(cursor_run_id))
    OR (wait_row.created_at = sqlc.arg(cursor_at) AND run.id = sqlc.arg(cursor_run_id)
      AND wait_row.generation > sqlc.arg(cursor_generation))
    OR (wait_row.created_at = sqlc.arg(cursor_at) AND run.id = sqlc.arg(cursor_run_id)
      AND wait_row.generation = sqlc.arg(cursor_generation) AND wait_row.node_id > sqlc.arg(cursor_node_id))
    OR (wait_row.created_at = sqlc.arg(cursor_at) AND run.id = sqlc.arg(cursor_run_id)
      AND wait_row.generation = sqlc.arg(cursor_generation) AND wait_row.node_id = sqlc.arg(cursor_node_id)
      AND wait_row.item_index > sqlc.arg(cursor_item_index))
  )
ORDER BY wait_row.created_at ASC, run.id ASC, wait_row.generation ASC,
  wait_row.node_id ASC, wait_row.item_index ASC
LIMIT sqlc.arg(page_limit);

-- name: ListQuarantinedLoopNodeInventory :many
SELECT run.id AS loop_run_id, run.loop_name, run.generation, control.node_id,
  0 AS item_index, control.quarantined_at AS state_at
FROM loop_node_controls AS control
JOIN loop_runs AS run ON run.id = control.loop_run_id
WHERE run.workspace_id = sqlc.arg(filter_workspace_id) AND control.quarantined = 1
  AND control.quarantined_at IS NOT NULL
  AND (CAST(sqlc.arg(filter_loop_name) AS TEXT) = '' OR run.loop_name = CAST(sqlc.arg(filter_loop_name) AS TEXT))
  AND (CAST(sqlc.arg(filter_run_id) AS TEXT) = '' OR run.id = CAST(sqlc.arg(filter_run_id) AS TEXT))
  AND (
    CAST(sqlc.arg(has_cursor) AS INTEGER) = 0
    OR control.quarantined_at > sqlc.arg(cursor_at)
    OR (control.quarantined_at = sqlc.arg(cursor_at)
      AND run.id > sqlc.arg(cursor_run_id))
    OR (control.quarantined_at = sqlc.arg(cursor_at)
      AND run.id = sqlc.arg(cursor_run_id) AND run.generation > sqlc.arg(cursor_generation))
    OR (control.quarantined_at = sqlc.arg(cursor_at)
      AND run.id = sqlc.arg(cursor_run_id) AND run.generation = sqlc.arg(cursor_generation)
      AND control.node_id > sqlc.arg(cursor_node_id))
  )
ORDER BY state_at ASC, run.id ASC, run.generation ASC, control.node_id ASC
LIMIT sqlc.arg(page_limit);

-- name: ListAttentionLoopNodeInventory :many
SELECT run.id AS loop_run_id, run.loop_name, run.generation, control.node_id,
  0 AS item_index, control.updated_at AS state_at
FROM loop_node_controls AS control
JOIN loop_runs AS run ON run.id = control.loop_run_id
WHERE run.workspace_id = sqlc.arg(filter_workspace_id) AND control.attention_flag != ''
  AND (CAST(sqlc.arg(filter_loop_name) AS TEXT) = '' OR run.loop_name = CAST(sqlc.arg(filter_loop_name) AS TEXT))
  AND (CAST(sqlc.arg(filter_run_id) AS TEXT) = '' OR run.id = CAST(sqlc.arg(filter_run_id) AS TEXT))
  AND (
    CAST(sqlc.arg(has_cursor) AS INTEGER) = 0
    OR control.updated_at > sqlc.arg(cursor_at)
    OR (control.updated_at = sqlc.arg(cursor_at) AND run.id > sqlc.arg(cursor_run_id))
    OR (control.updated_at = sqlc.arg(cursor_at) AND run.id = sqlc.arg(cursor_run_id)
      AND run.generation > sqlc.arg(cursor_generation))
    OR (control.updated_at = sqlc.arg(cursor_at) AND run.id = sqlc.arg(cursor_run_id)
      AND run.generation = sqlc.arg(cursor_generation) AND control.node_id > sqlc.arg(cursor_node_id))
  )
ORDER BY control.updated_at ASC, run.id ASC, run.generation ASC, control.node_id ASC
LIMIT sqlc.arg(page_limit);

-- name: ListRetryingLoopNodeInventory :many
SELECT run.id AS loop_run_id, run.loop_name, output.generation, output.node_id,
  output.item_index, output.next_attempt_at AS state_at
FROM loop_generation_outputs AS output
JOIN loop_runs AS run ON run.id = output.loop_run_id
WHERE run.workspace_id = sqlc.arg(filter_workspace_id)
  AND output.status = 'retrying' AND output.next_attempt_at IS NOT NULL
  AND (CAST(sqlc.arg(filter_loop_name) AS TEXT) = '' OR run.loop_name = CAST(sqlc.arg(filter_loop_name) AS TEXT))
  AND (CAST(sqlc.arg(filter_run_id) AS TEXT) = '' OR run.id = CAST(sqlc.arg(filter_run_id) AS TEXT))
  AND (
    CAST(sqlc.arg(has_cursor) AS INTEGER) = 0
    OR output.next_attempt_at > sqlc.arg(cursor_at)
    OR (output.next_attempt_at = sqlc.arg(cursor_at) AND run.id > sqlc.arg(cursor_run_id))
    OR (output.next_attempt_at = sqlc.arg(cursor_at) AND run.id = sqlc.arg(cursor_run_id)
      AND output.generation > sqlc.arg(cursor_generation))
    OR (output.next_attempt_at = sqlc.arg(cursor_at) AND run.id = sqlc.arg(cursor_run_id)
      AND output.generation = sqlc.arg(cursor_generation) AND output.node_id > sqlc.arg(cursor_node_id))
    OR (output.next_attempt_at = sqlc.arg(cursor_at) AND run.id = sqlc.arg(cursor_run_id)
      AND output.generation = sqlc.arg(cursor_generation) AND output.node_id = sqlc.arg(cursor_node_id)
      AND output.item_index > sqlc.arg(cursor_item_index))
  )
ORDER BY output.next_attempt_at ASC, run.id ASC, output.generation ASC,
  output.node_id ASC, output.item_index ASC
LIMIT sqlc.arg(page_limit);

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
