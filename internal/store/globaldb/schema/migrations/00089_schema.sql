-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_network_audit_log" table
CREATE TABLE `new_network_audit_log` (`id` text NULL, `profile_id` text NOT NULL, `session_id` text NOT NULL, `workspace_id` text NOT NULL, `direction` text NOT NULL, `kind` text NOT NULL, `channel` text NOT NULL, `surface` text NULL, `thread_id` text NULL, `direct_id` text NULL, `work_id` text NULL, `peer_from` text NOT NULL, `peer_to` text NULL, `message_id` text NOT NULL, `reason` text NULL, `size` integer NOT NULL, `timestamp` text NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION);
-- copy rows from old table "network_audit_log" to new temporary table "new_network_audit_log"
INSERT INTO `new_network_audit_log` (`id`, `profile_id`, `session_id`, `workspace_id`, `direction`, `kind`, `channel`, `surface`, `thread_id`, `direct_id`, `work_id`, `peer_from`, `peer_to`, `message_id`, `reason`, `size`, `timestamp`)
SELECT audit.`id`, COALESCE(
	(SELECT thread.`profile_id` FROM `network_threads` AS thread
		WHERE thread.`workspace_id` = audit.`workspace_id`
			AND thread.`channel` = audit.`channel`
			AND thread.`thread_id` = audit.`thread_id`),
	(SELECT direct.`profile_id` FROM `network_direct_rooms` AS direct
		WHERE direct.`workspace_id` = audit.`workspace_id`
			AND direct.`channel` = audit.`channel`
			AND direct.`direct_id` = audit.`direct_id`),
	(SELECT channel.`profile_id` FROM `network_channels` AS channel
		WHERE channel.`workspace_id` = audit.`workspace_id`
			AND channel.`channel` = audit.`channel`),
	(SELECT session.`profile_id` FROM `sessions` AS session
		WHERE session.`workspace_id` = audit.`workspace_id`
			AND session.`id` = audit.`session_id`),
	'00000000000000000000000000'
), audit.`session_id`, audit.`workspace_id`, audit.`direction`, audit.`kind`, audit.`channel`,
	audit.`surface`, audit.`thread_id`, audit.`direct_id`, audit.`work_id`, audit.`peer_from`,
	audit.`peer_to`, audit.`message_id`, audit.`reason`, audit.`size`, audit.`timestamp`
FROM `network_audit_log` AS audit;
-- drop trigger "workspace_scope_cleanup_after_delete" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `workspace_scope_cleanup_after_delete`;
-- remove the obsolete pre-profile name before installing the canonical cleanup trigger
DROP TRIGGER IF EXISTS `no_workspace_data_workspace_delete`;
-- drop "network_audit_log" table after copying rows
DROP TABLE `network_audit_log`;
-- rename temporary table "new_network_audit_log" to "network_audit_log"
ALTER TABLE `new_network_audit_log` RENAME TO `network_audit_log`;
-- create index "idx_net_audit_conversation" to table: "network_audit_log"
CREATE INDEX `idx_net_audit_conversation` ON `network_audit_log` (`workspace_id`, `channel`, `surface`, `thread_id`, `direct_id`, `timestamp`);
-- create index "idx_net_audit_ts" to table: "network_audit_log"
CREATE INDEX `idx_net_audit_ts` ON `network_audit_log` (`timestamp`);
-- create index "idx_net_audit_profile_workspace_timestamp" to table: "network_audit_log"
CREATE INDEX `idx_net_audit_profile_workspace_timestamp` ON `network_audit_log` (`profile_id`, `workspace_id`, `timestamp`, `id`);
-- create index "idx_net_audit_work" to table: "network_audit_log"
CREATE INDEX `idx_net_audit_work` ON `network_audit_log` (`workspace_id`, `work_id`, `timestamp`) WHERE work_id IS NOT NULL;
-- create index "idx_net_audit_workspace_session" to table: "network_audit_log"
CREATE INDEX `idx_net_audit_workspace_session` ON `network_audit_log` (`workspace_id`, `session_id`);
-- create index "idx_bridge_instances_profile_catalog" to table: "bridge_instances"
CREATE INDEX `idx_bridge_instances_profile_catalog` ON `bridge_instances` (`profile_id`, `display_name`, `created_at`, `id`);
-- create index "idx_automation_jobs_profile" to table: "automation_jobs"
CREATE INDEX `idx_automation_jobs_profile` ON `automation_jobs` (`profile_id`, `id`);
-- create index "idx_automation_triggers_profile" to table: "automation_triggers"
CREATE INDEX `idx_automation_triggers_profile` ON `automation_triggers` (`profile_id`, `id`);
-- drop index "idx_automation_suggestions_workspace_status" from table: "automation_suggestions"
DROP INDEX `idx_automation_suggestions_workspace_status`;
-- drop index "automation_suggestions_workspace_id_dedup_key" from table: "automation_suggestions"
DROP INDEX `automation_suggestions_workspace_id_dedup_key`;
-- create index "idx_automation_suggestions_profile_workspace_status" to table: "automation_suggestions"
CREATE INDEX `idx_automation_suggestions_profile_workspace_status` ON `automation_suggestions` (`profile_id`, `workspace_id`, `status`, `created_at`, `id`);
-- create index "automation_suggestions_profile_workspace_dedup_key" to table: "automation_suggestions"
CREATE UNIQUE INDEX `automation_suggestions_profile_workspace_dedup_key` ON `automation_suggestions` (`profile_id`, `workspace_id`, `dedup_key`);
-- create index "idx_sessions_profile_catalog_activity" to table: "sessions"
CREATE INDEX idx_sessions_profile_catalog_activity
			ON sessions(
				profile_id, workspace_id, state, COALESCE(last_update_at, updated_at) DESC,
				updated_at DESC, created_at DESC, id DESC
			);
-- create index "idx_sessions_profile_catalog_recent" to table: "sessions"
CREATE INDEX `idx_sessions_profile_catalog_recent` ON `sessions` (`profile_id`, `workspace_id`, `state`, `updated_at` DESC, `created_at` DESC, `id` DESC);
-- create index "idx_sessions_profile_catalog_archive_recent" to table: "sessions"
CREATE INDEX `idx_sessions_profile_catalog_archive_recent` ON `sessions` (`profile_id`, `workspace_id`, `archived_at`, `state`, `updated_at` DESC, `created_at` DESC, `id` DESC);
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
-- recreate trigger "workspace_scope_cleanup_after_delete" after rebuilding table "workspaces"
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
	DELETE FROM loop_goal_session_cleanup WHERE workspace_id = OLD.id;
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
END;
-- +goose StatementEnd

