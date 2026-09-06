-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_event_summaries" table
CREATE TABLE `new_event_summaries` (`seq` integer NULL PRIMARY KEY AUTOINCREMENT, `id` text NOT NULL, `profile_id` text NOT NULL, `session_id` text NOT NULL DEFAULT '', `workspace_id` text NOT NULL DEFAULT '', `worktree_id` text NOT NULL DEFAULT '', `type` text NOT NULL, `agent_name` text NOT NULL DEFAULT '', `content_json` text NOT NULL DEFAULT '', `task_id` text NOT NULL DEFAULT '', `run_id` text NOT NULL DEFAULT '', `workflow_id` text NOT NULL DEFAULT '', `claim_token_hash` text NOT NULL DEFAULT '', `lease_until` text NOT NULL DEFAULT '', `coordinator_session_id` text NOT NULL DEFAULT '', `scheduler_reason` text NOT NULL DEFAULT '', `hook_event` text NOT NULL DEFAULT '', `hook_name` text NOT NULL DEFAULT '', `actor_kind` text NOT NULL DEFAULT '', `actor_id` text NOT NULL DEFAULT '', `release_reason` text NOT NULL DEFAULT '', `parent_session_id` text NOT NULL DEFAULT '', `root_session_id` text NOT NULL DEFAULT '', `spawn_depth` integer NOT NULL DEFAULT 0, `summary` text NULL, `timestamp` text NOT NULL, `provider` text NOT NULL DEFAULT '', `outcome` text NOT NULL DEFAULT 'info', CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION);
-- copy rows from old table "event_summaries" to new temporary table "new_event_summaries"
-- Preserve the pre-migration rowid cursor, including retained gaps.
INSERT INTO `new_event_summaries` (`seq`, `id`, `profile_id`, `session_id`, `workspace_id`, `worktree_id`, `type`, `agent_name`, `content_json`, `task_id`, `run_id`, `workflow_id`, `claim_token_hash`, `lease_until`, `coordinator_session_id`, `scheduler_reason`, `hook_event`, `hook_name`, `actor_kind`, `actor_id`, `release_reason`, `parent_session_id`, `root_session_id`, `spawn_depth`, `summary`, `timestamp`, `provider`, `outcome`) SELECT rowid, `id`, `profile_id`, `session_id`, `workspace_id`, `worktree_id`, `type`, `agent_name`, `content_json`, `task_id`, `run_id`, `workflow_id`, `claim_token_hash`, `lease_until`, `coordinator_session_id`, `scheduler_reason`, `hook_event`, `hook_name`, `actor_kind`, `actor_id`, `release_reason`, `parent_session_id`, `root_session_id`, `spawn_depth`, `summary`, `timestamp`, `provider`, `outcome` FROM `event_summaries`;
-- drop trigger "event_summaries_profile_owner_active" before applying its declarative change
DROP TRIGGER IF EXISTS `event_summaries_profile_owner_active`;
-- drop trigger "event_summaries_profile_owner_immutable" before applying its declarative change
DROP TRIGGER IF EXISTS `event_summaries_profile_owner_immutable`;
-- drop trigger "workspace_scope_cleanup_after_delete" before applying its declarative change
DROP TRIGGER IF EXISTS `workspace_scope_cleanup_after_delete`;
-- drop "event_summaries" table after copying rows
DROP TABLE `event_summaries`;
-- rename temporary table "new_event_summaries" to "event_summaries"
ALTER TABLE `new_event_summaries` RENAME TO `event_summaries`;
-- create index "event_summaries_id" to table: "event_summaries"
CREATE UNIQUE INDEX `event_summaries_id` ON `event_summaries` (`id`);
-- create index "idx_summaries_actor" to table: "event_summaries"
CREATE INDEX `idx_summaries_actor` ON `event_summaries` (`actor_kind`, `actor_id`);
-- create index "idx_summaries_hook_event" to table: "event_summaries"
CREATE INDEX `idx_summaries_hook_event` ON `event_summaries` (`hook_event`);
-- create index "idx_summaries_outcome_timestamp" to table: "event_summaries"
CREATE INDEX `idx_summaries_outcome_timestamp` ON `event_summaries` (`outcome`, `timestamp` DESC);
-- create index "idx_summaries_parent" to table: "event_summaries"
CREATE INDEX `idx_summaries_parent` ON `event_summaries` (`parent_session_id`);
-- create index "idx_summaries_provider_timestamp" to table: "event_summaries"
CREATE INDEX `idx_summaries_provider_timestamp` ON `event_summaries` (`provider`, `timestamp` DESC);
-- create index "idx_summaries_root" to table: "event_summaries"
CREATE INDEX `idx_summaries_root` ON `event_summaries` (`root_session_id`);
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
-- create index "idx_summaries_workflow" to table: "event_summaries"
CREATE INDEX `idx_summaries_workflow` ON `event_summaries` (`workflow_id`);
-- create index "idx_summaries_workspace" to table: "event_summaries"
CREATE INDEX `idx_summaries_workspace` ON `event_summaries` (`workspace_id`);
-- create index "idx_summaries_worktree" to table: "event_summaries"
CREATE INDEX `idx_summaries_worktree` ON `event_summaries` (`worktree_id`);
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
-- apply declarative trigger "event_summaries_profile_owner_active" on table "event_summaries"
-- +goose StatementBegin
CREATE TRIGGER event_summaries_profile_owner_active BEFORE INSERT ON event_summaries BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd
-- apply declarative trigger "event_summaries_profile_owner_immutable" on table "event_summaries"
-- +goose StatementBegin
CREATE TRIGGER event_summaries_profile_owner_immutable BEFORE UPDATE OF profile_id ON event_summaries
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
-- apply declarative trigger "workspace_scope_cleanup_after_delete" on table "workspaces"
-- +goose StatementBegin
CREATE TRIGGER workspace_scope_cleanup_after_delete
AFTER DELETE ON workspaces
BEGIN
	DELETE FROM network_wake_events WHERE workspace_id = OLD.id;
	DELETE FROM network_wake_sources WHERE workspace_id = OLD.id;
	DELETE FROM network_message_dispositions WHERE workspace_id = OLD.id;
	DELETE FROM network_live_wakes WHERE workspace_id = OLD.id;
	DELETE FROM network_participation_budgets WHERE workspace_id = OLD.id;
	DELETE FROM network_task_status_projections WHERE workspace_id = OLD.id;
	DELETE FROM network_task_thread_origins WHERE workspace_id = OLD.id;
	DELETE FROM network_thread_session_token_stats WHERE workspace_id = OLD.id;
	DELETE FROM network_thread_participants WHERE workspace_id = OLD.id;
	DELETE FROM network_subscriptions WHERE workspace_id = OLD.id;
	DELETE FROM network_work WHERE workspace_id = OLD.id;
	DELETE FROM network_direct_rooms WHERE workspace_id = OLD.id;
	DELETE FROM network_threads WHERE workspace_id = OLD.id;
	DELETE FROM network_channel_participants WHERE workspace_id = OLD.id;
	DELETE FROM network_channel_kind_counts WHERE workspace_id = OLD.id;
	DELETE FROM network_channel_stats WHERE workspace_id = OLD.id;
	DELETE FROM network_timeline_log WHERE workspace_id = OLD.id;
	DELETE FROM network_channels WHERE workspace_id = OLD.id;
	DELETE FROM network_audit_log WHERE workspace_id = OLD.id;
	DELETE FROM network_coordination_invitations WHERE workspace_id = OLD.id;
	DELETE FROM task_network_coordination WHERE workspace_id = OLD.id;
	DELETE FROM loop_ui_annotations WHERE workspace_id = OLD.id;
	DELETE FROM loop_session_bindings WHERE workspace_id = OLD.id;
	DELETE FROM loop_run_events WHERE workspace_id = OLD.id;
	DELETE FROM loop_runs WHERE workspace_id = OLD.id;
	DELETE FROM loop_goal_session_outbox WHERE workspace_id = OLD.id;
	DELETE FROM loop_session_cleanup WHERE workspace_id = OLD.id;
	DELETE FROM loop_admission_claims WHERE workspace_id = OLD.id;
	DELETE FROM loop_node_lane_pauses WHERE workspace_id = OLD.id;
	DELETE FROM loop_node_amendments WHERE workspace_id = OLD.id;
	DELETE FROM loop_timetravel_ops WHERE workspace_id = OLD.id;
	DELETE FROM loop_requests WHERE workspace_id = OLD.id;
	DELETE FROM loop_gate_decisions WHERE workspace_id = OLD.id;
	DELETE FROM loop_definition_snapshots WHERE workspace_id = OLD.id;
	DELETE FROM loop_config WHERE workspace_id = OLD.id;
	DELETE FROM agent_heartbeat_wake_events WHERE workspace_id = OLD.id;
	DELETE FROM agent_heartbeat_wake_state WHERE workspace_id = OLD.id;
	DELETE FROM agent_heartbeat_revisions WHERE workspace_id = OLD.id;
	DELETE FROM agent_heartbeat_snapshots WHERE workspace_id = OLD.id;
	DELETE FROM agent_soul_revisions WHERE workspace_id = OLD.id;
	DELETE FROM agent_soul_snapshots WHERE workspace_id = OLD.id;
	DELETE FROM session_health WHERE workspace_id = OLD.id;
	DELETE FROM sessions WHERE workspace_id = OLD.id;
	DELETE FROM token_usage_daily WHERE workspace_id = OLD.id;
	DELETE FROM event_summaries WHERE workspace_id = OLD.id;
	DELETE FROM tool_approval_grants WHERE workspace_id = OLD.id;
	DELETE FROM dead_entities WHERE workspace_id = OLD.id;
	DELETE FROM notification_cursors WHERE workspace_id = OLD.id;
	DELETE FROM skill_exposures WHERE workspace_id = OLD.id;
END;
-- +goose StatementEnd
