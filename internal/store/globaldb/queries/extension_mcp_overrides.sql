-- name: ListExtensionMCPOverrides :many
SELECT * FROM extension_mcp_overrides
WHERE profile = sqlc.arg(profile) AND workspace_id = sqlc.arg(workspace_id)
ORDER BY extension, server;

-- name: InsertExtensionMCPAllocation :exec
INSERT INTO extension_mcp_overrides (
 extension, profile, workspace_id, server, runtime_name, updated_at
) VALUES (
 sqlc.arg(extension), sqlc.arg(profile), sqlc.arg(workspace_id), sqlc.arg(server),
 sqlc.arg(runtime_name), sqlc.arg(updated_at)
);

-- name: UpdateExtensionMCPOverride :execrows
UPDATE extension_mcp_overrides
SET env_json = sqlc.arg(env_json), headers_json = sqlc.arg(headers_json),
 url = sqlc.arg(url), updated_at = sqlc.arg(updated_at)
WHERE extension = sqlc.arg(extension) AND profile = sqlc.arg(profile)
 AND workspace_id = sqlc.arg(workspace_id) AND server = sqlc.arg(server);

-- name: DeleteExtensionMCPOverrides :exec
DELETE FROM extension_mcp_overrides
WHERE extension = sqlc.arg(extension) AND profile = sqlc.arg(profile)
 AND workspace_id = sqlc.arg(workspace_id);

-- name: ListAllExtensionMCPOverrides :many
SELECT * FROM extension_mcp_overrides ORDER BY profile, workspace_id, extension, server;

-- name: DeleteExtensionMCPWorkspace :exec
DELETE FROM extension_mcp_overrides
WHERE extension = sqlc.arg(extension) AND workspace_id = sqlc.arg(workspace_id);

-- name: DeleteExtensionMCPAllocation :exec
DELETE FROM extension_mcp_overrides
WHERE extension = sqlc.arg(extension) AND profile = sqlc.arg(profile)
 AND workspace_id = sqlc.arg(workspace_id) AND server = sqlc.arg(server);
