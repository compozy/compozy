-- +goose Up
-- create "task_network_coordination" table
CREATE TABLE `task_network_coordination` (`task_id` text NULL, `workspace_id` text NOT NULL, `enabled` integer NOT NULL, `revision` integer NOT NULL, `updated_at` text NOT NULL, `updated_by` text NOT NULL, PRIMARY KEY (`task_id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`task_id`) REFERENCES `tasks` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (enabled IN (0, 1)), CHECK (revision >= 1), CHECK (length(trim(updated_by)) > 0));
-- create index "idx_task_network_coordination_workspace" to table: "task_network_coordination"
CREATE INDEX `idx_task_network_coordination_workspace` ON `task_network_coordination` (`workspace_id`, `task_id`);
