CREATE TABLE mcp_auth_tokens (
			scope          TEXT NOT NULL CHECK (scope IN ('global', 'workspace')),
			workspace_id   TEXT NOT NULL DEFAULT '',
			server_name    TEXT NOT NULL CHECK (trim(server_name) <> ''),
			definition_fingerprint TEXT NOT NULL DEFAULT '',
			issuer         TEXT NOT NULL DEFAULT '',
			client_id      TEXT NOT NULL,
			scopes_json    TEXT NOT NULL DEFAULT '[]',
			access_token_ref   TEXT NOT NULL,
			refresh_token_ref  TEXT NOT NULL DEFAULT '',
			token_type     TEXT NOT NULL DEFAULT 'Bearer',
			expires_at     TEXT,
			obtained_at    TEXT NOT NULL,
			updated_at     TEXT NOT NULL,
			PRIMARY KEY (scope, workspace_id, server_name),
			CHECK (
				(scope = 'global' AND workspace_id = '') OR
				(scope = 'workspace' AND trim(workspace_id) <> '')
			)
		);

CREATE INDEX idx_mcp_auth_tokens_updated_at
			ON mcp_auth_tokens(updated_at);
