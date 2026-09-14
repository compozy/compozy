-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_mcp_auth_tokens" table
CREATE TABLE `new_mcp_auth_tokens` (`scope` text NOT NULL, `workspace_id` text NOT NULL DEFAULT '', `owner` text NOT NULL DEFAULT 'manual', `server_name` text NOT NULL, `definition_fingerprint` text NOT NULL DEFAULT '', `issuer` text NOT NULL DEFAULT '', `client_id` text NOT NULL, `scopes_json` text NOT NULL DEFAULT '[]', `access_token_ref` text NOT NULL, `refresh_token_ref` text NOT NULL DEFAULT '', `token_type` text NOT NULL DEFAULT 'Bearer', `expires_at` text NULL, `obtained_at` text NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`scope`, `workspace_id`, `owner`, `server_name`), CHECK (scope IN ('user', 'workspace', 'profile', 'workspace_profile')), CHECK (owner = 'manual' OR (owner LIKE 'extension:%' AND length(owner) > 10)), CHECK (trim(server_name) <> ''), CHECK (
				(scope = 'user' AND workspace_id = '') OR
				(scope IN ('workspace', 'profile', 'workspace_profile') AND trim(workspace_id) <> '')
			));
-- copy rows from old table "mcp_auth_tokens" to new temporary table "new_mcp_auth_tokens"
INSERT INTO `new_mcp_auth_tokens` (`scope`, `workspace_id`, `server_name`, `definition_fingerprint`, `issuer`, `client_id`, `scopes_json`, `access_token_ref`, `refresh_token_ref`, `token_type`, `expires_at`, `obtained_at`, `updated_at`) SELECT `scope`, `workspace_id`, `server_name`, `definition_fingerprint`, `issuer`, `client_id`, `scopes_json`, `access_token_ref`, `refresh_token_ref`, `token_type`, `expires_at`, `obtained_at`, `updated_at` FROM `mcp_auth_tokens`;
-- drop "mcp_auth_tokens" table after copying rows
DROP TABLE `mcp_auth_tokens`;
-- rename temporary table "new_mcp_auth_tokens" to "mcp_auth_tokens"
ALTER TABLE `new_mcp_auth_tokens` RENAME TO `mcp_auth_tokens`;
-- create index "idx_mcp_auth_tokens_updated_at" to table: "mcp_auth_tokens"
CREATE INDEX `idx_mcp_auth_tokens_updated_at` ON `mcp_auth_tokens` (`updated_at`);
-- create "new_mcp_oauth_registrations" table
CREATE TABLE `new_mcp_oauth_registrations` (`scope` text NOT NULL, `workspace_id` text NOT NULL DEFAULT '', `owner` text NOT NULL DEFAULT 'manual', `server_name` text NOT NULL, `definition_fingerprint` text NOT NULL, `resource_url` text NOT NULL, `issuer` text NOT NULL, `client_id` text NOT NULL, `token_endpoint_auth_method` text NOT NULL DEFAULT '', `client_secret_ref` text NOT NULL DEFAULT '', `registration_access_token_ref` text NOT NULL DEFAULT '', `registration_client_uri` text NOT NULL DEFAULT '', `client_id_issued_at` text NULL, `client_secret_expires_at` text NULL, `redirect_uri` text NOT NULL, `scopes_json` text NOT NULL DEFAULT '[]', `updated_at` text NOT NULL, PRIMARY KEY (`scope`, `workspace_id`, `owner`, `server_name`), CHECK (scope IN ('user', 'workspace', 'profile', 'workspace_profile')), CHECK (owner = 'manual' OR (owner LIKE 'extension:%' AND length(owner) > 10)), CHECK (trim(server_name) <> ''), CHECK (trim(definition_fingerprint) <> ''), CHECK (trim(resource_url) <> ''), CHECK (trim(issuer) <> ''), CHECK (trim(client_id) <> ''), CHECK (trim(redirect_uri) <> ''), CHECK (json_valid(scopes_json)), CHECK (trim(updated_at) <> ''), CHECK (
				(scope = 'user' AND workspace_id = '') OR
				(scope IN ('workspace', 'profile', 'workspace_profile') AND trim(workspace_id) <> '')
			), CHECK (
				(registration_access_token_ref = '' AND registration_client_uri = '') OR
				(trim(registration_access_token_ref) <> '' AND trim(registration_client_uri) <> '')
			));
-- copy rows from old table "mcp_oauth_registrations" to new temporary table "new_mcp_oauth_registrations"
INSERT INTO `new_mcp_oauth_registrations` (`scope`, `workspace_id`, `server_name`, `definition_fingerprint`, `resource_url`, `issuer`, `client_id`, `token_endpoint_auth_method`, `client_secret_ref`, `registration_access_token_ref`, `registration_client_uri`, `client_id_issued_at`, `client_secret_expires_at`, `redirect_uri`, `scopes_json`, `updated_at`) SELECT `scope`, `workspace_id`, `server_name`, `definition_fingerprint`, `resource_url`, `issuer`, `client_id`, `token_endpoint_auth_method`, `client_secret_ref`, `registration_access_token_ref`, `registration_client_uri`, `client_id_issued_at`, `client_secret_expires_at`, `redirect_uri`, `scopes_json`, `updated_at` FROM `mcp_oauth_registrations`;
-- drop "mcp_oauth_registrations" table after copying rows
DROP TABLE `mcp_oauth_registrations`;
-- rename temporary table "new_mcp_oauth_registrations" to "mcp_oauth_registrations"
ALTER TABLE `new_mcp_oauth_registrations` RENAME TO `mcp_oauth_registrations`;
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
