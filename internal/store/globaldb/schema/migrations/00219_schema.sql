-- +goose Up
-- add column "loop_input_command" to table: "automation_jobs"
ALTER TABLE `automation_jobs` ADD COLUMN `loop_input_command` text NULL;
-- add column "loop_input_command" to table: "automation_triggers"
ALTER TABLE `automation_triggers` ADD COLUMN `loop_input_command` text NULL;
