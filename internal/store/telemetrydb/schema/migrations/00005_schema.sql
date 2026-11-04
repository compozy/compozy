-- +goose Up
-- create index "idx_summaries_activity" to table: "event_summaries"
CREATE INDEX `idx_summaries_activity` ON `event_summaries` (`profile_id`, `agent_name`, `workspace_id`, `timestamp`);
