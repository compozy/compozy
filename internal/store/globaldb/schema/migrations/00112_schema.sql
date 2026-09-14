-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_extension_env_bindings" table
CREATE TABLE `new_extension_env_bindings` (`extension_name` text NOT NULL, `profile_id` text NOT NULL DEFAULT '', `workspace_id` text NOT NULL DEFAULT '', `env_name` text NOT NULL, `secret_ref` text NOT NULL, `input_id` text NOT NULL DEFAULT '', `active` integer NOT NULL DEFAULT 1, `mcp_server` text NOT NULL DEFAULT '', `header_name` text NOT NULL DEFAULT '', `kind` text NOT NULL, `created_at` text NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`extension_name`, `profile_id`, `workspace_id`, `env_name`), CHECK (active IN (0, 1)), CHECK (kind = 'extension_env'), CHECK ((mcp_server = '' AND header_name = '') OR (mcp_server <> '' AND header_name <> '')));
-- copy rows from old table "extension_env_bindings" to new temporary table "new_extension_env_bindings"
INSERT INTO `new_extension_env_bindings` (`extension_name`, `profile_id`, `workspace_id`, `env_name`, `secret_ref`, `mcp_server`, `header_name`, `kind`, `created_at`, `updated_at`) SELECT `extension_name`, `profile_id`, `workspace_id`, `env_name`, `secret_ref`, `mcp_server`, `header_name`, `kind`, `created_at`, `updated_at` FROM `extension_env_bindings`;
-- drop trigger "extension_env_bindings_profile_delete" before applying its declarative change
DROP TRIGGER IF EXISTS `extension_env_bindings_profile_delete`;
-- drop trigger "extension_env_bindings_profile_insert" before applying its declarative change
DROP TRIGGER IF EXISTS `extension_env_bindings_profile_insert`;
-- drop trigger "extension_env_bindings_workspace_delete" before applying its declarative change
DROP TRIGGER IF EXISTS `extension_env_bindings_workspace_delete`;
-- drop "extension_env_bindings" table after copying rows
DROP TABLE `extension_env_bindings`;
-- rename temporary table "new_extension_env_bindings" to "extension_env_bindings"
ALTER TABLE `new_extension_env_bindings` RENAME TO `extension_env_bindings`;
-- create index "idx_extension_env_bindings_secret_ref" to table: "extension_env_bindings"
CREATE INDEX `idx_extension_env_bindings_secret_ref` ON `extension_env_bindings` (`secret_ref`);
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
-- apply declarative trigger "extension_env_bindings_profile_delete" on table "profiles"
-- +goose StatementBegin
CREATE TRIGGER extension_env_bindings_profile_delete
AFTER DELETE ON profiles
BEGIN
	DELETE FROM extension_env_bindings WHERE profile_id = OLD.id;
END;
-- +goose StatementEnd
-- apply declarative trigger "extension_env_bindings_profile_insert" on table "extension_env_bindings"
-- +goose StatementBegin
CREATE TRIGGER extension_env_bindings_profile_insert
BEFORE INSERT ON extension_env_bindings
WHEN NEW.profile_id <> '' AND NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id)
BEGIN
	SELECT RAISE(ABORT, 'profile_not_found');
END;
-- +goose StatementEnd
-- apply declarative trigger "extension_env_bindings_workspace_delete" on table "workspaces"
-- +goose StatementBegin
CREATE TRIGGER extension_env_bindings_workspace_delete
AFTER DELETE ON workspaces
BEGIN
	DELETE FROM extension_env_bindings WHERE workspace_id = OLD.id;
END;
-- +goose StatementEnd
