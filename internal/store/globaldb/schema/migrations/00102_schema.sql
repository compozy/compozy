-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_sessions" table
CREATE TABLE `new_sessions` (`id` text NULL, `profile_id` text NOT NULL, `name` text NULL, `agent_name` text NOT NULL, `provider` text NOT NULL DEFAULT '', `model` text NOT NULL DEFAULT '', `reasoning_effort` text NOT NULL DEFAULT '', `speed` text NOT NULL DEFAULT '', `acp_options_json` text NOT NULL DEFAULT '[]', `speed_resolution_json` text NOT NULL DEFAULT '', `runtime_status` text NOT NULL DEFAULT 'unbound', `runtime_transition` text NOT NULL DEFAULT '', `runtime_failure` text NOT NULL DEFAULT '', `runtime_generation` integer NOT NULL DEFAULT 0, `runtime_recovery_json` text NOT NULL DEFAULT '', `selected_provider` text NOT NULL DEFAULT '', `selected_model` text NOT NULL DEFAULT '', `selected_reasoning_effort` text NOT NULL DEFAULT '', `selected_speed` text NOT NULL DEFAULT '', `selected_acp_options_json` text NOT NULL DEFAULT '[]', `runtime_selection_revision` integer NOT NULL DEFAULT 0, `workspace_id` text NOT NULL, `scope` text NOT NULL DEFAULT 'workspace', `worktree_id` text NULL, `session_type` text NOT NULL DEFAULT 'user', `state` text NOT NULL, `archived_at` text NULL, `acp_session_id` text NULL, `stop_reason` text NULL, `stop_escalated` boolean NOT NULL DEFAULT FALSE, `stop_detail` text NULL, `subprocess_pid` integer NOT NULL DEFAULT 0, `subprocess_started_at` text NULL, `last_update_at` text NULL, `stall_state` text NOT NULL DEFAULT '', `stall_reason` text NOT NULL DEFAULT '', `activity_json` text NOT NULL DEFAULT '', `attached_to` text NOT NULL DEFAULT '', `attach_expires_at` text NULL, `transcript_epoch` integer NOT NULL DEFAULT 0, `pending_permission_count` integer NOT NULL DEFAULT 0, `pending_clarify_count` integer NOT NULL DEFAULT 0, `attention_revision` integer NOT NULL DEFAULT 0, `last_settled_revision` integer NOT NULL DEFAULT 0, `last_seen_revision` integer NOT NULL DEFAULT 0, `last_seen_at` text NULL, `attention_changed_at` text NULL, `sandbox_id` text NOT NULL DEFAULT '', `sandbox_backend` text NOT NULL DEFAULT 'local', `sandbox_profile` text NOT NULL DEFAULT '', `sandbox_instance_id` text NOT NULL DEFAULT '', `sandbox_state` text NOT NULL DEFAULT '', `sandbox_provider_state_json` text NOT NULL DEFAULT '', `sandbox_last_sync_at` text NULL, `sandbox_last_sync_error` text NOT NULL DEFAULT '', `created_at` text NOT NULL, `updated_at` text NOT NULL, `failure_kind` text NULL, `failure_summary` text NOT NULL DEFAULT '', `crash_bundle_path` text NOT NULL DEFAULT '', `parent_session_id` text NULL, `root_session_id` text NULL, `spawn_depth` integer NOT NULL DEFAULT 0, `spawn_role` text NULL, `ttl_expires_at` text NULL, `auto_stop_on_parent` boolean NOT NULL DEFAULT 0, `notify_creator` boolean NOT NULL DEFAULT 1, `spawn_budget_json` text NOT NULL DEFAULT '{}', `permission_policy_json` text NOT NULL DEFAULT '{}', `soul_snapshot_id` text NULL, `soul_digest` text NOT NULL DEFAULT '', `parent_soul_digest` text NOT NULL DEFAULT '', `input_generation` integer NOT NULL DEFAULT 0, `creation_digest` text NULL, `policy_spec_digest` text NULL, `creation_profile_ref` text NULL, `network_spec_json` text NOT NULL DEFAULT '{"version":"network-participation/v1","mode":"local","source":"built_in_local"}', `network_mode` text NOT NULL DEFAULT 'local', `network_channel` text NULL, `network_source` text NOT NULL DEFAULT 'built_in_local', PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`, `worktree_id`) REFERENCES `worktrees` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `1` FOREIGN KEY (`soul_snapshot_id`) REFERENCES `agent_soul_snapshots` (`id`) ON UPDATE NO ACTION ON DELETE SET NULL, CONSTRAINT `2` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (json_valid(acp_options_json)), CHECK (runtime_generation >= 0), CHECK (runtime_recovery_json = '' OR json_valid(runtime_recovery_json)), CHECK (json_valid(selected_acp_options_json)), CHECK (scope IN ('global', 'workspace')), CHECK (stop_escalated IN (0, 1)), CHECK (pending_permission_count >= 0), CHECK (pending_clarify_count >= 0), CHECK (attention_revision >= 0), CHECK (last_settled_revision >= 0), CHECK (last_seen_revision >= 0), CHECK (creation_digest IS NULL OR length(trim(creation_digest)) > 0), CHECK (policy_spec_digest IS NULL OR length(trim(policy_spec_digest)) > 0), CHECK (creation_profile_ref IS NULL OR length(trim(creation_profile_ref)) > 0), CHECK (json_valid(network_spec_json)), CHECK (network_mode IN ('local', 'live')), CHECK (network_source IN (
					'explicit_request', 'task_profile', 'workspace_coordination',
					'loop_definition', 'automation_job', 'built_in_local'
				)), CHECK ((scope = 'workspace') = (workspace_id <> '')));
-- copy rows from old table "sessions" to new temporary table "new_sessions"
INSERT INTO `new_sessions` (`id`, `profile_id`, `name`, `agent_name`, `provider`, `model`, `reasoning_effort`, `speed`, `acp_options_json`, `speed_resolution_json`, `runtime_status`, `runtime_transition`, `runtime_failure`, `runtime_generation`, `runtime_recovery_json`, `selected_provider`, `selected_model`, `selected_reasoning_effort`, `selected_speed`, `selected_acp_options_json`, `runtime_selection_revision`, `workspace_id`, `scope`, `worktree_id`, `session_type`, `state`, `archived_at`, `acp_session_id`, `stop_reason`, `stop_detail`, `subprocess_pid`, `subprocess_started_at`, `last_update_at`, `stall_state`, `stall_reason`, `activity_json`, `attached_to`, `attach_expires_at`, `transcript_epoch`, `pending_permission_count`, `pending_clarify_count`, `attention_revision`, `last_settled_revision`, `last_seen_revision`, `last_seen_at`, `attention_changed_at`, `sandbox_id`, `sandbox_backend`, `sandbox_profile`, `sandbox_instance_id`, `sandbox_state`, `sandbox_provider_state_json`, `sandbox_last_sync_at`, `sandbox_last_sync_error`, `created_at`, `updated_at`, `failure_kind`, `failure_summary`, `crash_bundle_path`, `parent_session_id`, `root_session_id`, `spawn_depth`, `spawn_role`, `ttl_expires_at`, `auto_stop_on_parent`, `notify_creator`, `spawn_budget_json`, `permission_policy_json`, `soul_snapshot_id`, `soul_digest`, `parent_soul_digest`, `input_generation`, `creation_digest`, `policy_spec_digest`, `creation_profile_ref`, `network_spec_json`, `network_mode`, `network_channel`, `network_source`) SELECT `id`, `profile_id`, `name`, `agent_name`, `provider`, `model`, `reasoning_effort`, `speed`, `acp_options_json`, `speed_resolution_json`, `runtime_status`, `runtime_transition`, `runtime_failure`, `runtime_generation`, `runtime_recovery_json`, `selected_provider`, `selected_model`, `selected_reasoning_effort`, `selected_speed`, `selected_acp_options_json`, `runtime_selection_revision`, `workspace_id`, `scope`, `worktree_id`, `session_type`, `state`, `archived_at`, `acp_session_id`, `stop_reason`, `stop_detail`, `subprocess_pid`, `subprocess_started_at`, `last_update_at`, `stall_state`, `stall_reason`, `activity_json`, `attached_to`, `attach_expires_at`, `transcript_epoch`, `pending_permission_count`, `pending_clarify_count`, `attention_revision`, `last_settled_revision`, `last_seen_revision`, `last_seen_at`, `attention_changed_at`, `sandbox_id`, `sandbox_backend`, `sandbox_profile`, `sandbox_instance_id`, `sandbox_state`, `sandbox_provider_state_json`, `sandbox_last_sync_at`, `sandbox_last_sync_error`, `created_at`, `updated_at`, `failure_kind`, `failure_summary`, `crash_bundle_path`, `parent_session_id`, `root_session_id`, `spawn_depth`, `spawn_role`, `ttl_expires_at`, `auto_stop_on_parent`, `notify_creator`, `spawn_budget_json`, `permission_policy_json`, `soul_snapshot_id`, `soul_digest`, `parent_soul_digest`, `input_generation`, `creation_digest`, `policy_spec_digest`, `creation_profile_ref`, `network_spec_json`, `network_mode`, `network_channel`, `network_source` FROM `sessions`;
-- drop trigger "sessions_profile_owner_active" before applying its declarative change
DROP TRIGGER IF EXISTS `sessions_profile_owner_active`;
-- drop trigger "sessions_profile_owner_immutable" before applying its declarative change
DROP TRIGGER IF EXISTS `sessions_profile_owner_immutable`;
-- drop trigger "sessions_workspace_insert_guard" before applying its declarative change
DROP TRIGGER IF EXISTS `sessions_workspace_insert_guard`;
-- drop trigger "sessions_workspace_update_guard" before applying its declarative change
DROP TRIGGER IF EXISTS `sessions_workspace_update_guard`;
-- drop trigger "trg_sessions_archive_insert_guard" before applying its declarative change
DROP TRIGGER IF EXISTS `trg_sessions_archive_insert_guard`;
-- drop trigger "trg_sessions_archive_update_guard" before applying its declarative change
DROP TRIGGER IF EXISTS `trg_sessions_archive_update_guard`;
-- drop trigger "workspace_scope_cleanup_after_delete" before applying its declarative change
DROP TRIGGER IF EXISTS `workspace_scope_cleanup_after_delete`;
-- drop "sessions" table after copying rows
DROP TABLE `sessions`;
-- rename temporary table "new_sessions" to "sessions"
ALTER TABLE `new_sessions` RENAME TO `sessions`;
-- create index "sessions_workspace_id_id" to table: "sessions"
CREATE UNIQUE INDEX `sessions_workspace_id_id` ON `sessions` (`workspace_id`, `id`);
-- create index "idx_sessions_attach_lock" to table: "sessions"
CREATE INDEX `idx_sessions_attach_lock` ON `sessions` (`attached_to`, `attach_expires_at`);
-- create index "idx_sessions_catalog_activity" to table: "sessions"
CREATE INDEX idx_sessions_catalog_activity
			ON sessions(
				workspace_id, state, COALESCE(last_update_at, updated_at) DESC,
				updated_at DESC, created_at DESC, id DESC
			);
-- create index "idx_sessions_profile_catalog_activity" to table: "sessions"
CREATE INDEX idx_sessions_profile_catalog_activity
			ON sessions(
				profile_id, workspace_id, state, COALESCE(last_update_at, updated_at) DESC,
				updated_at DESC, created_at DESC, id DESC
			);
-- create index "idx_sessions_catalog_recent" to table: "sessions"
CREATE INDEX `idx_sessions_catalog_recent` ON `sessions` (`workspace_id`, `state`, `updated_at` DESC, `created_at` DESC, `id` DESC);
-- create index "idx_sessions_profile_catalog_recent" to table: "sessions"
CREATE INDEX `idx_sessions_profile_catalog_recent` ON `sessions` (`profile_id`, `workspace_id`, `state`, `updated_at` DESC, `created_at` DESC, `id` DESC);
-- create index "idx_sessions_catalog_archive_recent" to table: "sessions"
CREATE INDEX `idx_sessions_catalog_archive_recent` ON `sessions` (`workspace_id`, `archived_at`, `state`, `updated_at` DESC, `created_at` DESC, `id` DESC);
-- create index "idx_sessions_profile_catalog_archive_recent" to table: "sessions"
CREATE INDEX `idx_sessions_profile_catalog_archive_recent` ON `sessions` (`profile_id`, `workspace_id`, `archived_at`, `state`, `updated_at` DESC, `created_at` DESC, `id` DESC);
-- create index "idx_sessions_parent" to table: "sessions"
CREATE INDEX `idx_sessions_parent` ON `sessions` (`parent_session_id`);
-- create index "idx_sessions_resumable" to table: "sessions"
CREATE INDEX `idx_sessions_resumable` ON `sessions` (`state`, `failure_kind`, `last_update_at`, `updated_at`);
-- create index "idx_sessions_root" to table: "sessions"
CREATE INDEX `idx_sessions_root` ON `sessions` (`root_session_id`);
-- create index "idx_sessions_soul_snapshot" to table: "sessions"
CREATE INDEX `idx_sessions_soul_snapshot` ON `sessions` (`soul_snapshot_id`);
-- create index "idx_sessions_spawn_role" to table: "sessions"
CREATE INDEX `idx_sessions_spawn_role` ON `sessions` (`spawn_role`);
-- create index "idx_sessions_type_depth" to table: "sessions"
CREATE INDEX `idx_sessions_type_depth` ON `sessions` (`session_type`, `spawn_depth`);
-- create index "idx_sessions_worktree" to table: "sessions"
CREATE INDEX `idx_sessions_worktree` ON `sessions` (`worktree_id`) WHERE worktree_id IS NOT NULL;
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
-- apply declarative trigger "sessions_profile_owner_active" on table "sessions"
-- +goose StatementBegin
CREATE TRIGGER sessions_profile_owner_active BEFORE INSERT ON sessions BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd
-- apply declarative trigger "sessions_profile_owner_immutable" on table "sessions"
-- +goose StatementBegin
CREATE TRIGGER sessions_profile_owner_immutable BEFORE UPDATE OF profile_id ON sessions
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
-- apply declarative trigger "sessions_workspace_insert_guard" on table "sessions"
-- +goose StatementBegin
CREATE TRIGGER sessions_workspace_insert_guard
BEFORE INSERT ON sessions
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- apply declarative trigger "sessions_workspace_update_guard" on table "sessions"
-- +goose StatementBegin
CREATE TRIGGER sessions_workspace_update_guard
BEFORE UPDATE OF workspace_id ON sessions
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- apply declarative trigger "trg_sessions_archive_insert_guard" on table "sessions"
-- +goose StatementBegin
CREATE TRIGGER trg_sessions_archive_insert_guard
			BEFORE INSERT ON sessions
			WHEN NEW.archived_at IS NOT NULL AND NEW.state != 'stopped'
			BEGIN
				SELECT RAISE(ABORT, 'session is archived');
			END;
-- +goose StatementEnd
-- apply declarative trigger "trg_sessions_archive_update_guard" on table "sessions"
-- +goose StatementBegin
CREATE TRIGGER trg_sessions_archive_update_guard
			BEFORE UPDATE OF state, archived_at ON sessions
			WHEN NEW.archived_at IS NOT NULL AND NEW.state != 'stopped'
			BEGIN
				SELECT RAISE(ABORT, 'session is archived');
			END;
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
