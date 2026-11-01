-- name: ListInstanceCollectorChain :many
SELECT id FROM sessions WHERE instance_id = sqlc.arg(instance_id);

-- name: ListInstanceCollectorTasks :many
SELECT t.id, '' AS run_id, t.profile_id, COALESCE(t.workspace_id, '') AS workspace_id,
       t.owner_ref, t.status
FROM tasks t JOIN sessions s ON s.id = t.owner_ref
WHERE s.instance_id = sqlc.arg(instance_id) AND t.owner_kind = 'agent_session'
  AND t.status = 'blocked' AND t.profile_id = sqlc.arg(profile_id)
  AND COALESCE(t.workspace_id, '') IN (SELECT value FROM json_each(sqlc.arg(workspaces)))
UNION ALL
SELECT t.id, r.id AS run_id, t.profile_id, COALESCE(t.workspace_id, '') AS workspace_id,
       t.owner_ref, r.status
FROM tasks t JOIN sessions s ON s.id = t.owner_ref JOIN task_runs r ON r.task_id = t.id
WHERE s.instance_id = sqlc.arg(instance_id) AND t.owner_kind = 'agent_session'
  AND r.status = 'queued' AND t.profile_id = sqlc.arg(profile_id)
  AND COALESCE(t.workspace_id, '') IN (SELECT value FROM json_each(sqlc.arg(workspaces)));

-- name: ListInstanceCollectorSessions :many
SELECT s.id, s.profile_id, s.workspace_id, COALESCE(h.health, 'unknown') AS health
FROM sessions s LEFT JOIN session_health h ON h.session_id = s.id
WHERE s.profile_id = sqlc.arg(profile_id)
  AND s.workspace_id IN (SELECT value FROM json_each(sqlc.arg(workspaces)))
  AND COALESCE(s.instance_id, '') <> sqlc.arg(instance_id)
  AND s.session_type <> 'triage'
ORDER BY s.id;

-- name: ListInstanceCollectorAutomations :many
SELECT id, profile_id, workspace_id, status, ended_at
FROM automation_runs
WHERE profile_id = sqlc.arg(profile_id) AND status = 'failed'
  AND workspace_id IN (SELECT value FROM json_each(sqlc.arg(workspaces)))
  AND ended_at IS NOT NULL
ORDER BY ended_at, id;
