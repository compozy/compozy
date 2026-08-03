-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- drop the workspace cleanup trigger before rebuilding its target table
DROP TRIGGER `extension_env_bindings_workspace_delete`;
-- create "new_extension_env_bindings" table
CREATE TABLE `new_extension_env_bindings` (`extension_name` text NOT NULL, `workspace_id` text NOT NULL DEFAULT '', `env_name` text NOT NULL, `secret_ref` text NOT NULL, `kind` text NOT NULL, `created_at` text NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`extension_name`, `workspace_id`, `env_name`), CHECK (kind = 'extension_env'));
-- copy rows from old table "extension_env_bindings" to new temporary table "new_extension_env_bindings"
INSERT INTO `new_extension_env_bindings` (`extension_name`, `workspace_id`, `env_name`, `secret_ref`, `kind`, `created_at`, `updated_at`) SELECT `extension_name`, `workspace_id`, `env_name`, `secret_ref`, `kind`, `created_at`, `updated_at` FROM `extension_env_bindings`;
-- drop "extension_env_bindings" table after copying rows
DROP TABLE `extension_env_bindings`;
-- rename temporary table "new_extension_env_bindings" to "extension_env_bindings"
ALTER TABLE `new_extension_env_bindings` RENAME TO `extension_env_bindings`;
-- create index "idx_extension_env_bindings_secret_ref" to table: "extension_env_bindings"
CREATE INDEX `idx_extension_env_bindings_secret_ref` ON `extension_env_bindings` (`secret_ref`);
-- recreate the workspace cleanup trigger after rebuilding its target table
-- +goose StatementBegin
CREATE TRIGGER extension_env_bindings_workspace_delete
AFTER DELETE ON workspaces
BEGIN
	DELETE FROM extension_env_bindings WHERE workspace_id = OLD.id;
END;
-- +goose StatementEnd
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
