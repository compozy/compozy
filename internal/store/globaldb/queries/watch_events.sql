-- name: ListParkedWatchEventSubscriptions :many
SELECT lr.workspace_id, lr.id, lr.loop_name, lr.generation, lr.inputs_json,
       lr.definition_digest, lds.definition_json, lgo.node_id,
       COALESCE(lgo.output_ref, '') AS output_ref
FROM loop_runs lr
JOIN loop_definition_snapshots lds
  ON lds.workspace_id = lr.workspace_id
 AND lds.definition_digest = lr.definition_digest
JOIN loop_generation_outputs lgo
  ON lgo.loop_run_id = lr.id
 AND lgo.generation = lr.generation
WHERE lr.status = sqlc.arg(status)
  AND json_extract(COALESCE(lgo.output_ref, '{}'), '$.kind') = sqlc.arg(output_kind)
ORDER BY lr.workspace_id ASC, lr.id ASC, lgo.node_id ASC;

-- name: ListParkedWatchEventSubscriptionsForLoopRun :many
SELECT lr.workspace_id, lr.id, lr.loop_name, lr.generation, lr.inputs_json,
       lr.definition_digest, lds.definition_json, lgo.node_id,
       COALESCE(lgo.output_ref, '') AS output_ref
FROM loop_runs lr
JOIN loop_definition_snapshots lds
  ON lds.workspace_id = lr.workspace_id
 AND lds.definition_digest = lr.definition_digest
JOIN loop_generation_outputs lgo
  ON lgo.loop_run_id = lr.id
 AND lgo.generation = lr.generation
WHERE lr.status = sqlc.arg(status)
  AND json_extract(COALESCE(lgo.output_ref, '{}'), '$.kind') = sqlc.arg(output_kind)
  AND lr.id = sqlc.arg(loop_run_id)
ORDER BY lr.workspace_id ASC, lr.id ASC, lgo.node_id ASC;

-- name: ListParkedWatchEventSubscriptionsPage :many
SELECT lr.workspace_id, lr.id, lr.loop_name, lr.generation, lr.inputs_json,
       lr.definition_digest, lds.definition_json, lgo.node_id,
       COALESCE(lgo.output_ref, '') AS output_ref
FROM loop_runs lr
JOIN loop_definition_snapshots lds
  ON lds.workspace_id = lr.workspace_id
 AND lds.definition_digest = lr.definition_digest
JOIN loop_generation_outputs lgo
  ON lgo.loop_run_id = lr.id
 AND lgo.generation = lr.generation
WHERE lr.status = sqlc.arg(status)
  AND json_extract(COALESCE(lgo.output_ref, '{}'), '$.kind') = sqlc.arg(output_kind)
  AND (
    lr.workspace_id > sqlc.arg(cursor_workspace_id)
    OR (lr.workspace_id = sqlc.arg(cursor_workspace_id) AND lr.id > sqlc.arg(cursor_loop_run_id))
    OR (lr.workspace_id = sqlc.arg(cursor_workspace_id)
        AND lr.id = sqlc.arg(cursor_loop_run_id)
        AND lgo.node_id > sqlc.arg(cursor_node_id))
  )
ORDER BY lr.workspace_id ASC, lr.id ASC, lgo.node_id ASC
LIMIT sqlc.arg(row_limit);

-- name: GetNetworkWatchEventsCursor :one
SELECT COALESCE(MAX(sequence), 0)
FROM network_timeline_log
WHERE workspace_id = sqlc.arg(workspace_id);
