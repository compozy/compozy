-- +goose Up
-- add column "selected_provider" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `selected_provider` text NOT NULL DEFAULT '';
-- add column "selected_model" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `selected_model` text NOT NULL DEFAULT '';
-- add column "selected_reasoning_effort" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `selected_reasoning_effort` text NOT NULL DEFAULT '';
-- add column "selected_speed" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `selected_speed` text NOT NULL DEFAULT '';
-- add column "runtime_selection_revision" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `runtime_selection_revision` integer NOT NULL DEFAULT 0;
