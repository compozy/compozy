-- +goose Up
-- add column "human_ingress" to table: "session_prompt_admissions"
ALTER TABLE `session_prompt_admissions` ADD COLUMN `human_ingress` integer NOT NULL DEFAULT 0;
-- add column "human_ingress" to table: "session_input_queue"
ALTER TABLE `session_input_queue` ADD COLUMN `human_ingress` integer NOT NULL DEFAULT 0;
