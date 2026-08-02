-- +goose Up
-- add column "network_requirement_digest" to table: "extensions"
ALTER TABLE `extensions` ADD COLUMN `network_requirement_digest` text NOT NULL DEFAULT '';
-- add column "network_confirmed_by" to table: "extensions"
ALTER TABLE `extensions` ADD COLUMN `network_confirmed_by` text NULL;
-- add column "network_confirmed_at" to table: "extensions"
ALTER TABLE `extensions` ADD COLUMN `network_confirmed_at` text NULL;
-- add column "network_requirement_digest" to table: "extension_dev_links"
ALTER TABLE `extension_dev_links` ADD COLUMN `network_requirement_digest` text NOT NULL DEFAULT '';
-- add column "network_confirmed_by" to table: "extension_dev_links"
ALTER TABLE `extension_dev_links` ADD COLUMN `network_confirmed_by` text NULL;
-- add column "network_confirmed_at" to table: "extension_dev_links"
ALTER TABLE `extension_dev_links` ADD COLUMN `network_confirmed_at` text NULL;
-- create "extension_env_bindings" table
CREATE TABLE `extension_env_bindings` (`extension_name` text NOT NULL, `workspace_id` text NOT NULL DEFAULT '', `env_name` text NOT NULL, `secret_ref` text NOT NULL, `kind` text NOT NULL, `created_at` text NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`extension_name`, `workspace_id`, `env_name`));
-- create index "idx_extension_env_bindings_secret_ref" to table: "extension_env_bindings"
CREATE INDEX `idx_extension_env_bindings_secret_ref` ON `extension_env_bindings` (`secret_ref`);
