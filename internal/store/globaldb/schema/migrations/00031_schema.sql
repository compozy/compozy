-- +goose Up
-- create "mcp_oauth_registrations" table
CREATE TABLE `mcp_oauth_registrations` (`scope` text NOT NULL, `workspace_id` text NOT NULL DEFAULT '', `server_name` text NOT NULL, `definition_fingerprint` text NOT NULL, `resource_url` text NOT NULL, `issuer` text NOT NULL, `client_id` text NOT NULL, `client_secret_ref` text NOT NULL DEFAULT '', `registration_access_token_ref` text NOT NULL DEFAULT '', `registration_client_uri` text NOT NULL DEFAULT '', `client_id_issued_at` text NULL, `client_secret_expires_at` text NULL, `redirect_uri` text NOT NULL, `scopes_json` text NOT NULL DEFAULT '[]', `updated_at` text NOT NULL, PRIMARY KEY (`scope`, `workspace_id`, `server_name`), CHECK (scope IN ('global', 'workspace')), CHECK (trim(server_name) <> ''), CHECK (trim(definition_fingerprint) <> ''), CHECK (trim(resource_url) <> ''), CHECK (trim(issuer) <> ''), CHECK (trim(client_id) <> ''), CHECK (trim(redirect_uri) <> ''), CHECK (json_valid(scopes_json)), CHECK (trim(updated_at) <> ''), CHECK (
				(scope = 'global' AND workspace_id = '') OR
				(scope = 'workspace' AND trim(workspace_id) <> '')
			), CHECK (
				(registration_access_token_ref = '' AND registration_client_uri = '') OR
				(trim(registration_access_token_ref) <> '' AND trim(registration_client_uri) <> '')
			));
