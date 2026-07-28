-- +goose Up
DELETE FROM vault_secrets
WHERE (
		ref LIKE 'vault:mcp/%/oauth/access-token'
		OR ref LIKE 'vault:mcp/%/oauth/refresh-token'
	)
	AND NOT EXISTS (
		SELECT 1
		FROM mcp_auth_tokens
		WHERE access_token_ref = vault_secrets.ref
			OR refresh_token_ref = vault_secrets.ref
	);
