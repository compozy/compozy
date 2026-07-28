-- name: GetAppMetadata :one
SELECT value FROM app_metadata WHERE key = sqlc.arg(key);

-- name: UpsertAppMetadata :exec
INSERT INTO app_metadata (key, value, updated_at)
VALUES (sqlc.arg(key), sqlc.arg(value), sqlc.arg(updated_at))
ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at;

-- name: InsertAppMetadataIfAbsent :exec
INSERT INTO app_metadata (key, value, updated_at)
VALUES (sqlc.arg(key), sqlc.arg(value), sqlc.arg(updated_at))
ON CONFLICT(key) DO NOTHING;

-- name: DeleteAppMetadata :exec
DELETE FROM app_metadata WHERE key = sqlc.arg(key);

-- name: ListBundleActivationResourceSpecs :many
SELECT spec_json FROM resource_records WHERE kind = sqlc.arg(kind);

-- name: InsertPermissionLog :exec
INSERT INTO permission_log (
  id, session_id, agent_name, action, resource, decision, policy_used, timestamp
) VALUES (
  sqlc.arg(id), sqlc.arg(session_id), sqlc.arg(agent_name), sqlc.arg(action),
  sqlc.arg(resource), sqlc.arg(decision), sqlc.arg(policy_used), sqlc.arg(timestamp)
);

-- name: InsertWorkspace :exec
INSERT INTO workspaces (
  id, root_dir, add_dirs, name, default_agent, sandbox_ref, created_at, updated_at
) VALUES (
  sqlc.arg(id), sqlc.arg(root_dir), sqlc.arg(add_dirs), sqlc.arg(name),
  sqlc.narg(default_agent), sqlc.arg(sandbox_ref), sqlc.arg(created_at), sqlc.arg(updated_at)
);

-- name: UpdateWorkspace :execrows
UPDATE workspaces SET
  root_dir = sqlc.arg(root_dir), add_dirs = sqlc.arg(add_dirs), name = sqlc.arg(name),
  default_agent = sqlc.narg(default_agent), sandbox_ref = sqlc.arg(sandbox_ref),
  updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);

-- name: ListActiveSessionIDsByWorkspace :many
SELECT id FROM sessions WHERE workspace_id = sqlc.arg(workspace_id) AND state = 'active';

-- name: DeleteSessionsByWorkspace :exec
DELETE FROM sessions WHERE workspace_id = sqlc.arg(workspace_id);

-- name: DeleteSession :execrows
DELETE FROM sessions WHERE id = sqlc.arg(id);

-- name: DeleteWorkspace :execrows
DELETE FROM workspaces WHERE id = sqlc.arg(id);

-- name: GetWorkspace :one
SELECT id, root_dir, add_dirs, name, default_agent, sandbox_ref, created_at, updated_at
FROM workspaces WHERE id = sqlc.arg(id);

-- name: GetWorkspaceByPath :one
SELECT id, root_dir, add_dirs, name, default_agent, sandbox_ref, created_at, updated_at
FROM workspaces WHERE root_dir = sqlc.arg(root_dir);

-- name: GetWorkspaceByName :one
SELECT id, root_dir, add_dirs, name, default_agent, sandbox_ref, created_at, updated_at
FROM workspaces WHERE name = sqlc.arg(name);

-- name: ListWorkspaces :many
SELECT id, root_dir, add_dirs, name, default_agent, sandbox_ref, created_at, updated_at
FROM workspaces ORDER BY name ASC, id ASC;
