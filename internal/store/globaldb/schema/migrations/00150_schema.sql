-- +goose Up
-- add column "grant_ceiling" to table: "automation_jobs"
ALTER TABLE `automation_jobs` ADD COLUMN `grant_ceiling` text NOT NULL DEFAULT '';
-- add column "grant_ceiling" to table: "automation_triggers"
ALTER TABLE `automation_triggers` ADD COLUMN `grant_ceiling` text NOT NULL DEFAULT '';
-- add column "grant_ceiling" to table: "loop_runs"
ALTER TABLE `loop_runs` ADD COLUMN `grant_ceiling` text NOT NULL DEFAULT '';
-- add column "grant_ceiling" to table: "tasks"
ALTER TABLE `tasks` ADD COLUMN `grant_ceiling` text NOT NULL DEFAULT '';
-- add column "effective_workspace_access" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `effective_workspace_access` text NOT NULL DEFAULT '';
