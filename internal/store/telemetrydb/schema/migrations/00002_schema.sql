-- +goose Up
-- drop index "idx_summaries_workspace_sequence" from table: "event_summaries"
DROP INDEX `idx_summaries_workspace_sequence`;
