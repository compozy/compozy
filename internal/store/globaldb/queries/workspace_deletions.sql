-- name: InsertWorkspaceDeletionIntent :execrows
INSERT INTO workspace_deletion_intents (
  workspace_id, root_dir, add_dirs, name, default_agent, sandbox_ref,
  created_at, updated_at, requested_at
)
SELECT
  id, root_dir, add_dirs, name, default_agent, sandbox_ref,
  created_at, updated_at, sqlc.arg(requested_at)
FROM workspaces
WHERE id = sqlc.arg(workspace_id);

-- name: GetWorkspaceDeletionIntent :one
SELECT
  workspace_id, root_dir, add_dirs, name, default_agent, sandbox_ref,
  created_at, updated_at, requested_at
FROM workspace_deletion_intents
WHERE workspace_id = sqlc.arg(workspace_id);

-- name: ListWorkspaceDeletionIntents :many
SELECT
  workspace_id, root_dir, add_dirs, name, default_agent, sandbox_ref,
  created_at, updated_at, requested_at
FROM workspace_deletion_intents
ORDER BY requested_at, workspace_id;

-- name: DeleteWorkspaceDeletionIntent :execrows
DELETE FROM workspace_deletion_intents
WHERE workspace_id = sqlc.arg(workspace_id);
