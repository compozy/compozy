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

CREATE TABLE mcp_oauth_registrations (
			scope                         TEXT NOT NULL CHECK (scope IN ('global', 'workspace')),
			workspace_id                  TEXT NOT NULL DEFAULT '',
			server_name                   TEXT NOT NULL CHECK (trim(server_name) <> ''),
			definition_fingerprint        TEXT NOT NULL CHECK (trim(definition_fingerprint) <> ''),
			resource_url                  TEXT NOT NULL CHECK (trim(resource_url) <> ''),
			issuer                        TEXT NOT NULL CHECK (trim(issuer) <> ''),
			client_id                     TEXT NOT NULL CHECK (trim(client_id) <> ''),
			token_endpoint_auth_method    TEXT NOT NULL DEFAULT '',
			client_secret_ref             TEXT NOT NULL DEFAULT '',
			registration_access_token_ref TEXT NOT NULL DEFAULT '',
			registration_client_uri       TEXT NOT NULL DEFAULT '',
			client_id_issued_at           TEXT,
			client_secret_expires_at      TEXT,
			redirect_uri                  TEXT NOT NULL CHECK (trim(redirect_uri) <> ''),
			scopes_json                   TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(scopes_json)),
			updated_at                    TEXT NOT NULL CHECK (trim(updated_at) <> ''),
			PRIMARY KEY (scope, workspace_id, server_name),
			CHECK (
				(scope = 'global' AND workspace_id = '') OR
				(scope = 'workspace' AND trim(workspace_id) <> '')
			),
			CHECK (
				(registration_access_token_ref = '' AND registration_client_uri = '') OR
				(trim(registration_access_token_ref) <> '' AND trim(registration_client_uri) <> '')
			)
		);
