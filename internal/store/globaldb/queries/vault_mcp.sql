-- name: UpsertVaultSecret :exec
INSERT INTO vault_secrets (ref, kind, encrypted_value, created_at, updated_at)
VALUES (sqlc.arg(ref), sqlc.arg(kind), sqlc.arg(encrypted_value), sqlc.arg(created_at), sqlc.arg(updated_at))
ON CONFLICT(ref) DO UPDATE SET
  kind = excluded.kind, encrypted_value = excluded.encrypted_value, updated_at = excluded.updated_at;

-- name: GetVaultSecret :one
SELECT ref, kind, encrypted_value, created_at, updated_at
FROM vault_secrets WHERE ref = sqlc.arg(ref);

-- name: DeleteVaultSecret :execrows
DELETE FROM vault_secrets WHERE ref = sqlc.arg(ref);

-- name: UpsertMCPAuthToken :exec
INSERT INTO mcp_auth_tokens (
  scope, workspace_id, server_name, definition_fingerprint, issuer, client_id, scopes_json,
  access_token_ref, refresh_token_ref,
  token_type, expires_at, obtained_at, updated_at
) VALUES (
  sqlc.arg(scope), sqlc.arg(workspace_id), sqlc.arg(server_name), sqlc.arg(definition_fingerprint),
  sqlc.arg(issuer), sqlc.arg(client_id), sqlc.arg(scopes_json),
  sqlc.arg(access_token_ref), sqlc.arg(refresh_token_ref), sqlc.arg(token_type),
  sqlc.narg(expires_at), sqlc.arg(obtained_at), sqlc.arg(updated_at)
)
ON CONFLICT(scope, workspace_id, server_name) DO UPDATE SET
  definition_fingerprint = excluded.definition_fingerprint, issuer = excluded.issuer,
  client_id = excluded.client_id, scopes_json = excluded.scopes_json,
  access_token_ref = excluded.access_token_ref, refresh_token_ref = excluded.refresh_token_ref,
  token_type = excluded.token_type, expires_at = excluded.expires_at,
  obtained_at = excluded.obtained_at, updated_at = excluded.updated_at;

-- name: GetMCPAuthToken :one
SELECT scope, workspace_id, server_name, definition_fingerprint, issuer, client_id, scopes_json,
       access_token_ref, refresh_token_ref,
       token_type, expires_at, obtained_at, updated_at
FROM mcp_auth_tokens
WHERE scope = sqlc.arg(scope)
  AND workspace_id = sqlc.arg(workspace_id)
  AND server_name = sqlc.arg(server_name);

-- name: ListMCPAuthTokens :many
SELECT scope, workspace_id, server_name, definition_fingerprint, issuer, client_id, scopes_json,
       access_token_ref, refresh_token_ref,
       token_type, expires_at, obtained_at, updated_at
FROM mcp_auth_tokens ORDER BY scope ASC, workspace_id ASC, server_name ASC;

-- name: GetMCPAuthTokenRefs :one
SELECT access_token_ref, refresh_token_ref
FROM mcp_auth_tokens
WHERE scope = sqlc.arg(scope)
  AND workspace_id = sqlc.arg(workspace_id)
  AND server_name = sqlc.arg(server_name);

-- name: DeleteMCPAuthToken :execrows
DELETE FROM mcp_auth_tokens
WHERE scope = sqlc.arg(scope)
  AND workspace_id = sqlc.arg(workspace_id)
  AND server_name = sqlc.arg(server_name);

-- name: ListMCPAuthTokenRefsByWorkspace :many
SELECT access_token_ref, refresh_token_ref
FROM mcp_auth_tokens
WHERE scope = 'workspace'
  AND workspace_id = sqlc.arg(workspace_id)
ORDER BY server_name ASC;

-- name: DeleteMCPAuthTokensByWorkspace :execrows
DELETE FROM mcp_auth_tokens
WHERE scope = 'workspace'
  AND workspace_id = sqlc.arg(workspace_id);
