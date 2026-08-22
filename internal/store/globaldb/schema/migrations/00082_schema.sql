-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_sessions" table
CREATE TABLE `new_sessions` (`id` text NULL, `name` text NULL, `agent_name` text NOT NULL, `provider` text NOT NULL DEFAULT '', `model` text NOT NULL DEFAULT '', `reasoning_effort` text NOT NULL DEFAULT '', `speed` text NOT NULL DEFAULT '', `speed_resolution_json` text NOT NULL DEFAULT '', `runtime_status` text NOT NULL DEFAULT 'unbound', `runtime_transition` text NOT NULL DEFAULT '', `runtime_failure` text NOT NULL DEFAULT '', `runtime_generation` integer NOT NULL DEFAULT 0, `runtime_recovery_json` text NOT NULL DEFAULT '', `selected_provider` text NOT NULL DEFAULT '', `selected_model` text NOT NULL DEFAULT '', `selected_reasoning_effort` text NOT NULL DEFAULT '', `selected_speed` text NOT NULL DEFAULT '', `runtime_selection_revision` integer NOT NULL DEFAULT 0, `workspace_id` text NOT NULL, `worktree_id` text NULL, `session_type` text NOT NULL DEFAULT 'user', `state` text NOT NULL, `archived_at` text NULL, `acp_session_id` text NULL, `stop_reason` text NULL, `stop_detail` text NULL, `subprocess_pid` integer NOT NULL DEFAULT 0, `subprocess_started_at` text NULL, `last_update_at` text NULL, `stall_state` text NOT NULL DEFAULT '', `stall_reason` text NOT NULL DEFAULT '', `activity_json` text NOT NULL DEFAULT '', `attached_to` text NOT NULL DEFAULT '', `attach_expires_at` text NULL, `transcript_epoch` integer NOT NULL DEFAULT 0, `pending_permission_count` integer NOT NULL DEFAULT 0, `pending_clarify_count` integer NOT NULL DEFAULT 0, `attention_revision` integer NOT NULL DEFAULT 0, `last_settled_revision` integer NOT NULL DEFAULT 0, `last_seen_revision` integer NOT NULL DEFAULT 0, `last_seen_at` text NULL, `attention_changed_at` text NULL, `sandbox_id` text NOT NULL DEFAULT '', `sandbox_backend` text NOT NULL DEFAULT 'local', `sandbox_profile` text NOT NULL DEFAULT '', `sandbox_instance_id` text NOT NULL DEFAULT '', `sandbox_state` text NOT NULL DEFAULT '', `sandbox_provider_state_json` text NOT NULL DEFAULT '', `sandbox_last_sync_at` text NULL, `sandbox_last_sync_error` text NOT NULL DEFAULT '', `created_at` text NOT NULL, `updated_at` text NOT NULL, `failure_kind` text NULL, `failure_summary` text NOT NULL DEFAULT '', `crash_bundle_path` text NOT NULL DEFAULT '', `parent_session_id` text NULL, `root_session_id` text NULL, `spawn_depth` integer NOT NULL DEFAULT 0, `spawn_role` text NULL, `ttl_expires_at` text NULL, `auto_stop_on_parent` boolean NOT NULL DEFAULT 0, `notify_creator` boolean NOT NULL DEFAULT 1, `spawn_budget_json` text NOT NULL DEFAULT '{}', `permission_policy_json` text NOT NULL DEFAULT '{}', `soul_snapshot_id` text NULL, `soul_digest` text NOT NULL DEFAULT '', `parent_soul_digest` text NOT NULL DEFAULT '', `input_generation` integer NOT NULL DEFAULT 0, `creation_digest` text NULL, `policy_spec_digest` text NULL, `creation_profile_ref` text NULL, `network_spec_json` text NOT NULL DEFAULT '{"version":"network-participation/v1","mode":"local","source":"built_in_local"}', `network_mode` text NOT NULL DEFAULT 'local', `network_channel` text NULL, `network_source` text NOT NULL DEFAULT 'built_in_local', PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`, `worktree_id`) REFERENCES `worktrees` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `1` FOREIGN KEY (`soul_snapshot_id`) REFERENCES `agent_soul_snapshots` (`id`) ON UPDATE NO ACTION ON DELETE SET NULL, CONSTRAINT `2` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (runtime_generation >= 0), CHECK (runtime_recovery_json = '' OR json_valid(runtime_recovery_json)), CHECK (pending_permission_count >= 0), CHECK (pending_clarify_count >= 0), CHECK (attention_revision >= 0), CHECK (last_settled_revision >= 0), CHECK (last_seen_revision >= 0), CHECK (creation_digest IS NULL OR length(trim(creation_digest)) > 0), CHECK (policy_spec_digest IS NULL OR length(trim(policy_spec_digest)) > 0), CHECK (creation_profile_ref IS NULL OR length(trim(creation_profile_ref)) > 0), CHECK (json_valid(network_spec_json)), CHECK (network_mode IN ('local', 'live')), CHECK (network_source IN (
					'explicit_request', 'task_profile', 'workspace_coordination',
					'loop_definition', 'automation_job', 'built_in_local'
				)));
-- copy rows from old table "sessions" to new temporary table "new_sessions"
INSERT INTO `new_sessions` (`id`, `name`, `agent_name`, `provider`, `model`, `reasoning_effort`, `speed`, `speed_resolution_json`, `runtime_status`, `runtime_transition`, `runtime_failure`, `selected_provider`, `selected_model`, `selected_reasoning_effort`, `selected_speed`, `runtime_selection_revision`, `workspace_id`, `worktree_id`, `session_type`, `state`, `archived_at`, `acp_session_id`, `stop_reason`, `stop_detail`, `subprocess_pid`, `subprocess_started_at`, `last_update_at`, `stall_state`, `stall_reason`, `activity_json`, `attached_to`, `attach_expires_at`, `transcript_epoch`, `pending_permission_count`, `pending_clarify_count`, `attention_revision`, `last_settled_revision`, `last_seen_revision`, `last_seen_at`, `attention_changed_at`, `sandbox_id`, `sandbox_backend`, `sandbox_profile`, `sandbox_instance_id`, `sandbox_state`, `sandbox_provider_state_json`, `sandbox_last_sync_at`, `sandbox_last_sync_error`, `created_at`, `updated_at`, `failure_kind`, `failure_summary`, `crash_bundle_path`, `parent_session_id`, `root_session_id`, `spawn_depth`, `spawn_role`, `ttl_expires_at`, `auto_stop_on_parent`, `notify_creator`, `spawn_budget_json`, `permission_policy_json`, `soul_snapshot_id`, `soul_digest`, `parent_soul_digest`, `input_generation`, `creation_digest`, `policy_spec_digest`, `creation_profile_ref`, `network_spec_json`, `network_mode`, `network_channel`, `network_source`) SELECT `id`, `name`, `agent_name`, `provider`, `model`, `reasoning_effort`, `speed`, `speed_resolution_json`, `runtime_status`, `runtime_transition`, `runtime_failure`, `selected_provider`, `selected_model`, `selected_reasoning_effort`, `selected_speed`, `runtime_selection_revision`, `workspace_id`, `worktree_id`, `session_type`, `state`, `archived_at`, `acp_session_id`, `stop_reason`, `stop_detail`, `subprocess_pid`, `subprocess_started_at`, `last_update_at`, `stall_state`, `stall_reason`, `activity_json`, `attached_to`, `attach_expires_at`, `transcript_epoch`, `pending_permission_count`, `pending_clarify_count`, `attention_revision`, `last_settled_revision`, `last_seen_revision`, `last_seen_at`, `attention_changed_at`, `sandbox_id`, `sandbox_backend`, `sandbox_profile`, `sandbox_instance_id`, `sandbox_state`, `sandbox_provider_state_json`, `sandbox_last_sync_at`, `sandbox_last_sync_error`, `created_at`, `updated_at`, `failure_kind`, `failure_summary`, `crash_bundle_path`, `parent_session_id`, `root_session_id`, `spawn_depth`, `spawn_role`, `ttl_expires_at`, `auto_stop_on_parent`, `notify_creator`, `spawn_budget_json`, `permission_policy_json`, `soul_snapshot_id`, `soul_digest`, `parent_soul_digest`, `input_generation`, `creation_digest`, `policy_spec_digest`, `creation_profile_ref`, `network_spec_json`, `network_mode`, `network_channel`, `network_source` FROM `sessions`;
-- drop trigger "trg_sessions_archive_insert_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `trg_sessions_archive_insert_guard`;
-- drop trigger "trg_sessions_archive_update_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `trg_sessions_archive_update_guard`;
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
-- create index "idx_sessions_catalog_recent" to table: "sessions"
CREATE INDEX `idx_sessions_catalog_recent` ON `sessions` (`workspace_id`, `state`, `updated_at` DESC, `created_at` DESC, `id` DESC);
-- create index "idx_sessions_catalog_archive_recent" to table: "sessions"
CREATE INDEX `idx_sessions_catalog_archive_recent` ON `sessions` (`workspace_id`, `archived_at`, `state`, `updated_at` DESC, `created_at` DESC, `id` DESC);
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
-- recreate trigger "trg_sessions_archive_insert_guard" after rebuilding table "sessions"
-- +goose StatementBegin
CREATE TRIGGER trg_sessions_archive_insert_guard
			BEFORE INSERT ON sessions
			WHEN NEW.archived_at IS NOT NULL AND NEW.state != 'stopped'
			BEGIN
				SELECT RAISE(ABORT, 'session is archived');
			END;
-- +goose StatementEnd
-- recreate trigger "trg_sessions_archive_update_guard" after rebuilding table "sessions"
-- +goose StatementBegin
CREATE TRIGGER trg_sessions_archive_update_guard
			BEFORE UPDATE OF state, archived_at ON sessions
			WHEN NEW.archived_at IS NOT NULL AND NEW.state != 'stopped'
			BEGIN
				SELECT RAISE(ABORT, 'session is archived');
			END;
-- +goose StatementEnd
