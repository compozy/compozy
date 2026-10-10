-- +goose Up
-- add column "origin_json" to table: "session_prompt_admissions"
ALTER TABLE `session_prompt_admissions` ADD COLUMN `origin_json` text NULL;
-- add column "origin_json" to table: "session_input_queue"
ALTER TABLE `session_input_queue` ADD COLUMN `origin_json` text NULL;
