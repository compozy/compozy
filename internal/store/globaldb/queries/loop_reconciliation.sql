-- name: ListLoopReconciliationCandidates :many
SELECT candidate.loop_run_id FROM (
    SELECT loop_run_id FROM task_runs
    WHERE status IN ('queued','claimed','starting','running','needs_attention')
      AND loop_run_id IS NOT NULL AND trim(loop_run_id) <> ''
    UNION
    SELECT tr.loop_run_id FROM tasks t INDEXED BY idx_tasks_unsettled CROSS JOIN task_runs tr ON tr.task_id = t.id
    WHERE t.status NOT IN ('completed','failed','canceled')
      AND tr.loop_run_id IS NOT NULL AND trim(tr.loop_run_id) <> ''
) candidate LEFT JOIN loop_runs lr ON lr.id = candidate.loop_run_id
WHERE lr.id IS NULL OR lr.status IN ('done','no-op','blocked','failed','exhausted','stalled','canceled')
ORDER BY candidate.loop_run_id;

-- name: GetLoopReconciliationStatus :one
SELECT status FROM loop_runs WHERE id = ?;

-- name: ListLoopProvenance :many
WITH coordinators AS (
    SELECT task_id, workspace_id, loop_run_id,
        ROW_NUMBER() OVER (PARTITION BY task_id, workspace_id ORDER BY queued_at DESC, id DESC) AS precedence
    FROM task_runs
    WHERE run_kind = 'coordinator' AND loop_run_id IS NOT NULL AND trim(loop_run_id) <> ''
)
SELECT t.id, t.workspace_id, t.metadata_json, tr.loop_run_id, lr.loop_name
FROM tasks t
JOIN coordinators tr ON tr.task_id = t.id AND tr.workspace_id = t.workspace_id AND tr.precedence = 1
LEFT JOIN loop_runs lr ON lr.id = tr.loop_run_id AND lr.workspace_id = t.workspace_id
;

-- name: UpdateLoopProvenance :execrows
UPDATE tasks SET metadata_json = sqlc.arg(metadata)
WHERE id = sqlc.arg(id) AND metadata_json IS sqlc.narg(previous_metadata);
