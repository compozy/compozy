-- +goose Up
-- create "event_summaries" table
CREATE TABLE `event_summaries` (`seq` integer NULL PRIMARY KEY AUTOINCREMENT, `id` text NOT NULL, `profile_id` text NOT NULL, `session_id` text NOT NULL DEFAULT '', `workspace_id` text NOT NULL DEFAULT '', `worktree_id` text NOT NULL DEFAULT '', `type` text NOT NULL, `agent_name` text NOT NULL DEFAULT '', `content_json` text NOT NULL DEFAULT '', `task_id` text NOT NULL DEFAULT '', `run_id` text NOT NULL DEFAULT '', `workflow_id` text NOT NULL DEFAULT '', `claim_token_hash` text NOT NULL DEFAULT '', `lease_until` text NOT NULL DEFAULT '', `coordinator_session_id` text NOT NULL DEFAULT '', `scheduler_reason` text NOT NULL DEFAULT '', `hook_event` text NOT NULL DEFAULT '', `hook_name` text NOT NULL DEFAULT '', `actor_kind` text NOT NULL DEFAULT '', `actor_id` text NOT NULL DEFAULT '', `release_reason` text NOT NULL DEFAULT '', `parent_session_id` text NOT NULL DEFAULT '', `root_session_id` text NOT NULL DEFAULT '', `spawn_depth` integer NOT NULL DEFAULT 0, `summary` text NULL, `timestamp` text NOT NULL, `provider` text NOT NULL DEFAULT '', `outcome` text NOT NULL DEFAULT 'info');
-- create index "event_summaries_id" to table: "event_summaries"
CREATE UNIQUE INDEX `event_summaries_id` ON `event_summaries` (`id`);
-- create index "idx_summaries_actor" to table: "event_summaries"
CREATE INDEX `idx_summaries_actor` ON `event_summaries` (`actor_kind`, `actor_id`);
-- create index "idx_summaries_failures_timestamp" to table: "event_summaries"
CREATE INDEX `idx_summaries_failures_timestamp` ON `event_summaries` (`timestamp` DESC) WHERE outcome IN ('failure', 'warning');
-- create index "idx_summaries_provider_timestamp" to table: "event_summaries"
CREATE INDEX `idx_summaries_provider_timestamp` ON `event_summaries` (`provider`, `timestamp` DESC);
-- create index "idx_summaries_run" to table: "event_summaries"
CREATE INDEX `idx_summaries_run` ON `event_summaries` (`run_id`);
-- create index "idx_summaries_session" to table: "event_summaries"
CREATE INDEX `idx_summaries_session` ON `event_summaries` (`session_id`);
-- create index "idx_summaries_task" to table: "event_summaries"
CREATE INDEX `idx_summaries_task` ON `event_summaries` (`task_id`);
-- create index "idx_summaries_timestamp" to table: "event_summaries"
CREATE INDEX `idx_summaries_timestamp` ON `event_summaries` (`timestamp`);
-- create index "idx_summaries_type" to table: "event_summaries"
CREATE INDEX `idx_summaries_type` ON `event_summaries` (`type`);
-- create index "idx_summaries_workspace" to table: "event_summaries"
CREATE INDEX `idx_summaries_workspace` ON `event_summaries` (`workspace_id`);
-- create index "idx_summaries_worktree" to table: "event_summaries"
CREATE INDEX `idx_summaries_worktree` ON `event_summaries` (`worktree_id`);
-- create index "idx_summaries_profile_timestamp" to table: "event_summaries"
CREATE INDEX `idx_summaries_profile_timestamp` ON `event_summaries` (`profile_id`, `timestamp` DESC);
-- create index "idx_summaries_profile_sequence" to table: "event_summaries"
CREATE INDEX `idx_summaries_profile_sequence` ON `event_summaries` (`profile_id`, `seq`);
-- create index "idx_summaries_workspace_sequence" to table: "event_summaries"
CREATE INDEX `idx_summaries_workspace_sequence` ON `event_summaries` (`workspace_id`, `seq`);
