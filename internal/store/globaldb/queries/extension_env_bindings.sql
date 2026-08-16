-- name: UpsertExtensionEnvBinding :exec
INSERT INTO extension_env_bindings (
  extension_name, workspace_id, env_name, secret_ref, kind, created_at, updated_at
) VALUES (
  sqlc.arg(extension_name), sqlc.arg(workspace_id), sqlc.arg(env_name), sqlc.arg(secret_ref),
  sqlc.arg(kind), sqlc.arg(created_at), sqlc.arg(updated_at)
)
ON CONFLICT(extension_name, workspace_id, env_name) DO UPDATE SET
  secret_ref = excluded.secret_ref,
  kind = excluded.kind,
  updated_at = excluded.updated_at;

-- name: ListExtensionEnvBindings :many
SELECT extension_name, workspace_id, env_name, secret_ref, mcp_server, header_name, kind, created_at, updated_at
FROM extension_env_bindings
WHERE extension_name = sqlc.arg(extension_name)
  AND workspace_id = sqlc.arg(workspace_id)
ORDER BY env_name ASC;

-- name: CountExtensionEnvBindingsBySecretRef :one
SELECT COUNT(*)
FROM extension_env_bindings
WHERE secret_ref = sqlc.arg(secret_ref);

-- name: DeleteExtensionEnvBinding :execrows
DELETE FROM extension_env_bindings
WHERE extension_name = sqlc.arg(extension_name)
  AND workspace_id = sqlc.arg(workspace_id)
  AND env_name = sqlc.arg(env_name);

-- name: DeleteExtensionEnvBindings :execrows
DELETE FROM extension_env_bindings
WHERE extension_name = sqlc.arg(extension_name)
  AND workspace_id = sqlc.arg(workspace_id);

-- name: ListExtensionEnvSecretRefsByWorkspace :many
SELECT DISTINCT secret_ref
FROM extension_env_bindings
WHERE workspace_id = sqlc.arg(workspace_id)
ORDER BY secret_ref ASC;

-- name: DeleteExtensionEnvBindingsByWorkspace :execrows
DELETE FROM extension_env_bindings
WHERE workspace_id = sqlc.arg(workspace_id);
