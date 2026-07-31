-- +goose Up
-- add column "model" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `model` text NOT NULL DEFAULT '';
-- add column "reasoning_effort" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `reasoning_effort` text NOT NULL DEFAULT '';
-- add column "speed" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `speed` text NOT NULL DEFAULT '';
-- add column "speed_resolution_json" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `speed_resolution_json` text NOT NULL DEFAULT '';
-- add column "runtime_status" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `runtime_status` text NOT NULL DEFAULT 'unbound';
-- add column "runtime_transition" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `runtime_transition` text NOT NULL DEFAULT '';
-- add column "runtime_failure" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `runtime_failure` text NOT NULL DEFAULT '';
