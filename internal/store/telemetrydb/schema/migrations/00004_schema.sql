-- +goose Up
-- drop index "idx_summaries_failures_timestamp" from table: "event_summaries"
DROP INDEX `idx_summaries_failures_timestamp`;
-- create index "idx_summaries_failures_timestamp" to table: "event_summaries"
CREATE INDEX `idx_summaries_failures_timestamp` ON `event_summaries` (`timestamp` DESC, `seq` DESC) WHERE outcome IN ('failure', 'warning');
-- drop index "idx_summaries_profile_timestamp" from table: "event_summaries"
DROP INDEX `idx_summaries_profile_timestamp`;
-- create index "idx_summaries_profile_timestamp" to table: "event_summaries"
CREATE INDEX `idx_summaries_profile_timestamp` ON `event_summaries` (`profile_id`, `timestamp` DESC, `seq` DESC);
