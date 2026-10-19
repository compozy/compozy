-- +goose Up
-- add column "mentions_json" to table: "session_prompt_admissions"
ALTER TABLE `session_prompt_admissions` ADD COLUMN `mentions_json` text NOT NULL DEFAULT '[]' CHECK (json_valid(mentions_json));
-- add column "mentions_json" to table: "session_input_queue"
ALTER TABLE `session_input_queue` ADD COLUMN `mentions_json` text NOT NULL DEFAULT '[]' CHECK (json_valid(mentions_json));
