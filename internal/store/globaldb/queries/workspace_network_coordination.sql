-- name: GetWorkspaceNetworkCoordination :one
SELECT workspace_id, enabled, revision, updated_at, updated_by
FROM workspace_network_coordination
WHERE workspace_id = sqlc.arg(workspace_id);

-- name: UpsertWorkspaceNetworkCoordination :one
INSERT INTO workspace_network_coordination (
  workspace_id,
  enabled,
  revision,
  updated_at,
  updated_by
) VALUES (
  sqlc.arg(workspace_id),
  sqlc.arg(enabled),
  1,
  sqlc.arg(updated_at),
  sqlc.arg(updated_by)
)
ON CONFLICT(workspace_id) DO UPDATE SET
  enabled = excluded.enabled,
  revision = workspace_network_coordination.revision + 1,
  updated_at = excluded.updated_at,
  updated_by = excluded.updated_by
RETURNING workspace_id, enabled, revision, updated_at, updated_by;
