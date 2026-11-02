-- name: SetWorkspaceArchived :execrows
UPDATE workspaces SET archived_at = sqlc.narg(archived_at), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);

-- name: ListWorkspaceArchiveBlockers :many
SELECT 'session' AS kind, s.id, s.name FROM sessions s
WHERE s.workspace_id = sqlc.arg(workspace_id) AND state <> 'stopped'
UNION ALL
SELECT 'instance' AS kind, i.id, i.agent_name AS name FROM agent_instances i
WHERE i.workspace_id = sqlc.arg(workspace_id) AND i.archived_at IS NULL
UNION ALL
SELECT 'loop' AS kind, r.id, r.loop_name AS name FROM loop_runs r
WHERE r.workspace_id = sqlc.arg(workspace_id)
  AND status IN ('queued', 'running', 'watching', 'needs-approval', 'paused')
UNION ALL
SELECT 'job' AS kind, j.id, j.name FROM automation_jobs j
LEFT JOIN automation_job_overlays o ON o.job_id = j.id
WHERE (j.workspace_id = sqlc.arg(workspace_id) OR j.loop_workspace_id = sqlc.arg(workspace_id))
  AND CASE WHEN j.source IN ('config', 'package') THEN COALESCE(o.enabled_override, j.enabled) ELSE j.enabled END = 1
  AND NOT EXISTS (SELECT 1 FROM resource_records r WHERE r.kind = 'automation.job' AND r.id = j.id)
UNION ALL
SELECT 'trigger' AS kind, t.id, t.name FROM automation_triggers t
LEFT JOIN automation_trigger_overlays o ON o.trigger_id = t.id
WHERE (t.workspace_id = sqlc.arg(workspace_id) OR t.loop_workspace_id = sqlc.arg(workspace_id))
  AND CASE WHEN t.source IN ('config', 'package') THEN COALESCE(o.enabled_override, t.enabled) ELSE t.enabled END = 1
  AND NOT EXISTS (SELECT 1 FROM resource_records r WHERE r.kind = 'automation.trigger' AND r.id = t.id)
UNION ALL
SELECT 'job' AS kind, r.id, CAST(json_extract(r.spec_json, '$.name') AS TEXT) AS name
FROM resource_records r LEFT JOIN automation_job_overlays o ON o.job_id = r.id
WHERE r.kind = 'automation.job'
  AND (json_extract(r.spec_json, '$.workspace_id') = sqlc.arg(workspace_id)
    OR json_extract(r.spec_json, '$.loop_target.workspace_id') = sqlc.arg(workspace_id))
  AND CASE WHEN json_extract(r.spec_json, '$.source') IN ('config', 'package')
    THEN COALESCE(o.enabled_override, json_extract(r.spec_json, '$.enabled'))
    ELSE json_extract(r.spec_json, '$.enabled') END = 1
UNION ALL
SELECT 'trigger' AS kind, r.id, CAST(json_extract(r.spec_json, '$.name') AS TEXT) AS name
FROM resource_records r LEFT JOIN automation_trigger_overlays o ON o.trigger_id = r.id
WHERE r.kind = 'automation.trigger'
  AND (json_extract(r.spec_json, '$.workspace_id') = sqlc.arg(workspace_id)
    OR json_extract(r.spec_json, '$.loop_target.workspace_id') = sqlc.arg(workspace_id))
  AND CASE WHEN json_extract(r.spec_json, '$.source') IN ('config', 'package')
    THEN COALESCE(o.enabled_override, json_extract(r.spec_json, '$.enabled'))
    ELSE json_extract(r.spec_json, '$.enabled') END = 1
ORDER BY kind, id;
