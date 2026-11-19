-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_extension_mcp_overrides" table
CREATE TABLE `new_extension_mcp_overrides` (`extension` text NOT NULL, `profile` text NOT NULL DEFAULT '', `workspace_id` text NOT NULL DEFAULT '', `server` text NOT NULL, `runtime_name` text NOT NULL, `env_json` text NOT NULL DEFAULT '{}', `headers_json` text NOT NULL DEFAULT '{}', `url` text NOT NULL DEFAULT '', `updated_at` text NOT NULL, `disabled` boolean NOT NULL DEFAULT FALSE, PRIMARY KEY (`extension`, `profile`, `workspace_id`, `server`), CHECK (trim(extension) <> ''), CHECK (trim(server) <> ''), CHECK (trim(runtime_name) <> ''), CHECK (json_valid(env_json)), CHECK (json_valid(headers_json)), CHECK (disabled IN (0, 1)));
-- copy rows from old table "extension_mcp_overrides" to new temporary table "new_extension_mcp_overrides"
INSERT INTO `new_extension_mcp_overrides` (`extension`, `profile`, `workspace_id`, `server`, `runtime_name`, `env_json`, `headers_json`, `url`, `updated_at`) SELECT `extension`, `profile`, `workspace_id`, `server`, `runtime_name`, `env_json`, `headers_json`, `url`, `updated_at` FROM `extension_mcp_overrides`;
-- drop trigger "extension_mcp_overrides_profile_delete" before applying its declarative change
DROP TRIGGER IF EXISTS `extension_mcp_overrides_profile_delete`;
-- drop trigger "extension_mcp_overrides_workspace_delete" before applying its declarative change
DROP TRIGGER IF EXISTS `extension_mcp_overrides_workspace_delete`;
-- drop "extension_mcp_overrides" table after copying rows
DROP TABLE `extension_mcp_overrides`;
-- rename temporary table "new_extension_mcp_overrides" to "extension_mcp_overrides"
ALTER TABLE `new_extension_mcp_overrides` RENAME TO `extension_mcp_overrides`;
-- create index "extension_mcp_overrides_profile_workspace_id_runtime_name" to table: "extension_mcp_overrides"
CREATE UNIQUE INDEX `extension_mcp_overrides_profile_workspace_id_runtime_name` ON `extension_mcp_overrides` (`profile`, `workspace_id`, `runtime_name`);
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
-- apply declarative trigger "extension_mcp_overrides_profile_delete" on table "profiles"
-- +goose StatementBegin
CREATE TRIGGER extension_mcp_overrides_profile_delete
AFTER DELETE ON profiles
BEGIN
 DELETE FROM extension_mcp_overrides WHERE profile = OLD.id;
END;
-- +goose StatementEnd
-- apply declarative trigger "extension_mcp_overrides_workspace_delete" on table "workspaces"
-- +goose StatementBegin
CREATE TRIGGER extension_mcp_overrides_workspace_delete
AFTER DELETE ON workspaces
BEGIN
 DELETE FROM extension_mcp_overrides WHERE workspace_id = OLD.id;
END;
-- +goose StatementEnd
