-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_task_network_coordination" table
CREATE TABLE `new_task_network_coordination` (`task_id` text NOT NULL, `workspace_id` text NOT NULL, `enabled` integer NOT NULL, `revision` integer NOT NULL, `updated_at` text NOT NULL, `updated_by` text NOT NULL, PRIMARY KEY (`task_id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`task_id`) REFERENCES `tasks` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (enabled IN (0, 1)), CHECK (revision >= 1), CHECK (length(trim(updated_by)) > 0));
-- copy rows from old table "task_network_coordination" to new temporary table "new_task_network_coordination"
INSERT INTO `new_task_network_coordination` (`task_id`, `workspace_id`, `enabled`, `revision`, `updated_at`, `updated_by`) SELECT `task_id`, `workspace_id`, `enabled`, `revision`, `updated_at`, `updated_by` FROM `task_network_coordination`;
-- drop "task_network_coordination" table after copying rows
DROP TABLE `task_network_coordination`;
-- rename temporary table "new_task_network_coordination" to "task_network_coordination"
ALTER TABLE `new_task_network_coordination` RENAME TO `task_network_coordination`;
-- create index "idx_task_network_coordination_workspace" to table: "task_network_coordination"
CREATE INDEX `idx_task_network_coordination_workspace` ON `task_network_coordination` (`workspace_id`, `task_id`);
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
