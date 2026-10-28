-- name: SetWorkspaceArchived :execrows
UPDATE workspaces SET archived_at = sqlc.narg(archived_at), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);

-- name: ListWorkspaceArchiveBlockers :many
SELECT 'session' AS kind, s.id, s.name FROM sessions s
WHERE s.workspace_id = sqlc.arg(workspace_id) AND state <> 'stopped'
UNION ALL
SELECT 'loop' AS kind, r.id, r.loop_name AS name FROM loop_runs r
WHERE r.workspace_id = sqlc.arg(workspace_id)
  AND status IN ('queued', 'running', 'watching', 'needs-approval', 'paused')
UNION ALL
SELECT 'job' AS kind, j.id, j.name FROM automation_jobs j
LEFT JOIN automation_job_overlays o ON o.job_id = j.id
WHERE (j.workspace_id = sqlc.arg(workspace_id) OR j.loop_workspace_id = sqlc.arg(workspace_id))
  AND COALESCE(o.enabled_override, j.enabled) = 1
UNION ALL
SELECT 'trigger' AS kind, t.id, t.name FROM automation_triggers t
LEFT JOIN automation_trigger_overlays o ON o.trigger_id = t.id
WHERE (t.workspace_id = sqlc.arg(workspace_id) OR t.loop_workspace_id = sqlc.arg(workspace_id))
  AND COALESCE(o.enabled_override, t.enabled) = 1
ORDER BY kind, id;
