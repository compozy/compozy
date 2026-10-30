-- +goose Up
-- create "session_process_observations" table
CREATE TABLE `session_process_observations` (`pid` integer NOT NULL, `started_at` text NOT NULL, `session_id` text NOT NULL, `workspace_id` text NOT NULL, `profile_id` text NOT NULL, `session_name` text NOT NULL, `pgid` integer NOT NULL, `first_seen` text NOT NULL, `group_observed_at` text NOT NULL, `attribution` text NOT NULL, PRIMARY KEY (`pid`, `started_at`), CHECK (pid > 1), CHECK (attribution IN ('env', 'lineage')));
-- create index "idx_session_process_observations_owner" to table: "session_process_observations"
CREATE INDEX `idx_session_process_observations_owner` ON `session_process_observations` (`profile_id`, `workspace_id`, `session_id`);
