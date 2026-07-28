-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_automation_suggestions" table
CREATE TABLE `new_automation_suggestions` (`id` text NULL, `workspace_id` text NOT NULL, `source` text NOT NULL, `dedup_key` text NOT NULL, `status` text NOT NULL, `payload` text NOT NULL, `created_at` text NOT NULL, `resolved_at` text NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (source IN ('catalog', 'usage', 'integration')), CHECK (status IN ('pending', 'accepted', 'dismissed')), CHECK (json_valid(payload) AND json_type(payload) = 'object'), CHECK (
			(status = 'pending' AND resolved_at IS NULL) OR
			(status IN ('accepted', 'dismissed') AND resolved_at IS NOT NULL)
		));
-- copy rows from old table "automation_suggestions" to new temporary table "new_automation_suggestions"
INSERT INTO `new_automation_suggestions` (`id`, `workspace_id`, `source`, `dedup_key`, `status`, `payload`, `created_at`, `resolved_at`) SELECT `id`, `workspace_id`, `source`, `dedup_key`, `status`, `payload`, `created_at`, `resolved_at` FROM `automation_suggestions`;
-- drop "automation_suggestions" table after copying rows
DROP TABLE `automation_suggestions`;
-- rename temporary table "new_automation_suggestions" to "automation_suggestions"
ALTER TABLE `new_automation_suggestions` RENAME TO `automation_suggestions`;
-- create index "automation_suggestions_workspace_id_dedup_key" to table: "automation_suggestions"
CREATE UNIQUE INDEX `automation_suggestions_workspace_id_dedup_key` ON `automation_suggestions` (`workspace_id`, `dedup_key`);
-- create index "idx_automation_suggestions_workspace_status" to table: "automation_suggestions"
CREATE INDEX `idx_automation_suggestions_workspace_status` ON `automation_suggestions` (`workspace_id`, `status`, `created_at`, `id`);
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
