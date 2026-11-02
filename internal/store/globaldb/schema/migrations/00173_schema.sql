-- +goose Up
-- add column "usage_total_tokens" to table: "session_subagents"
ALTER TABLE `session_subagents` ADD COLUMN `usage_total_tokens` integer NULL;
-- add column "usage_tool_uses" to table: "session_subagents"
ALTER TABLE `session_subagents` ADD COLUMN `usage_tool_uses` integer NULL;
