-- +goose Up
-- add column "usage_run_id" to table: "session_subagents"
ALTER TABLE `session_subagents` ADD COLUMN `usage_run_id` text NOT NULL DEFAULT '';
