-- +goose Up
-- add column "runtime_provider" to table: "session_input_queue"
ALTER TABLE `session_input_queue` ADD COLUMN `runtime_provider` text NOT NULL DEFAULT '';
-- add column "runtime_model" to table: "session_input_queue"
ALTER TABLE `session_input_queue` ADD COLUMN `runtime_model` text NOT NULL DEFAULT '';
-- add column "runtime_reasoning_effort" to table: "session_input_queue"
ALTER TABLE `session_input_queue` ADD COLUMN `runtime_reasoning_effort` text NOT NULL DEFAULT '';
-- add column "runtime_speed" to table: "session_input_queue"
ALTER TABLE `session_input_queue` ADD COLUMN `runtime_speed` text NOT NULL DEFAULT '';
