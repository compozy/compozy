-- +goose Up
-- create index "idx_task_blocks_task_created" to table: "task_blocks"
CREATE INDEX `idx_task_blocks_task_created` ON `task_blocks` (`task_id`, `created_at`, `id`);
-- create index "idx_task_designation_rollups_task" to table: "task_designation_rollups"
CREATE INDEX `idx_task_designation_rollups_task` ON `task_designation_rollups` (`task_id`);
-- create index "idx_perm_timestamp" to table: "permission_log"
CREATE INDEX `idx_perm_timestamp` ON `permission_log` (`timestamp`);
-- create index "idx_token_stats_updated" to table: "token_stats"
CREATE INDEX `idx_token_stats_updated` ON `token_stats` (`updated_at`);
-- create index "idx_agent_heartbeat_wake_events_session" to table: "agent_heartbeat_wake_events"
CREATE INDEX `idx_agent_heartbeat_wake_events_session` ON `agent_heartbeat_wake_events` (`session_id`);
-- create index "idx_agent_heartbeat_wake_state_session" to table: "agent_heartbeat_wake_state"
CREATE INDEX `idx_agent_heartbeat_wake_state_session` ON `agent_heartbeat_wake_state` (`session_id`);
-- create index "idx_session_input_clear_traces_session" to table: "session_input_clear_traces"
CREATE INDEX `idx_session_input_clear_traces_session` ON `session_input_clear_traces` (`session_id`);
-- create index "idx_session_subagents_workspace_created" to table: "session_subagents"
CREATE INDEX `idx_session_subagents_workspace_created` ON `session_subagents` (`workspace_id`, `created_at` DESC, `id` DESC);
-- create index "idx_session_subagents_wake_created" to table: "session_subagents"
CREATE INDEX `idx_session_subagents_wake_created` ON `session_subagents` (`wake_message_id`, `created_at`, `id`);
