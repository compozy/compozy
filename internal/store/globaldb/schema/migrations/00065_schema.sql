-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- add column "format" to table: "extensions"
ALTER TABLE `extensions` ADD COLUMN `format` text NOT NULL DEFAULT 'compozy';
-- add column "ingest_diagnostics_json" to table: "extensions"
ALTER TABLE `extensions` ADD COLUMN `ingest_diagnostics_json` text NOT NULL DEFAULT '[]';
-- add column "format" to table: "extension_dev_links"
ALTER TABLE `extension_dev_links` ADD COLUMN `format` text NOT NULL DEFAULT 'compozy';
-- add column "ingest_diagnostics_json" to table: "extension_dev_links"
ALTER TABLE `extension_dev_links` ADD COLUMN `ingest_diagnostics_json` text NOT NULL DEFAULT '[]';
-- create "new_extension_env_bindings" table
CREATE TABLE `new_extension_env_bindings` (`extension_name` text NOT NULL, `workspace_id` text NOT NULL DEFAULT '', `env_name` text NOT NULL, `secret_ref` text NOT NULL, `mcp_server` text NOT NULL DEFAULT '', `header_name` text NOT NULL DEFAULT '', `kind` text NOT NULL, `created_at` text NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`extension_name`, `workspace_id`, `env_name`), CHECK (kind = 'extension_env'), CHECK ((mcp_server = '' AND header_name = '') OR (mcp_server <> '' AND header_name <> '')));
-- copy rows from old table "extension_env_bindings" to new temporary table "new_extension_env_bindings"
INSERT INTO `new_extension_env_bindings` (`extension_name`, `workspace_id`, `env_name`, `secret_ref`, `kind`, `created_at`, `updated_at`) SELECT `extension_name`, `workspace_id`, `env_name`, `secret_ref`, `kind`, `created_at`, `updated_at` FROM `extension_env_bindings`;
-- drop trigger "extension_env_bindings_workspace_delete" before rebuilding its table
DROP TRIGGER `extension_env_bindings_workspace_delete`;
-- drop "extension_env_bindings" table after copying rows
DROP TABLE `extension_env_bindings`;
-- rename temporary table "new_extension_env_bindings" to "extension_env_bindings"
ALTER TABLE `new_extension_env_bindings` RENAME TO `extension_env_bindings`;
-- create index "idx_extension_env_bindings_secret_ref" to table: "extension_env_bindings"
CREATE INDEX `idx_extension_env_bindings_secret_ref` ON `extension_env_bindings` (`secret_ref`);
-- recreate trigger "extension_env_bindings_workspace_delete" after rebuilding table "extension_env_bindings"
-- +goose StatementBegin
CREATE TRIGGER extension_env_bindings_workspace_delete
AFTER DELETE ON workspaces
BEGIN
	DELETE FROM extension_env_bindings WHERE workspace_id = OLD.id;
END;
-- +goose StatementEnd
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
