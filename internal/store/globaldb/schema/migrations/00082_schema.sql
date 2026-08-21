-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create the permanent default owner before any profile stamp is backfilled
CREATE TABLE `profiles` (`id` text NULL, `name` text NOT NULL, `color` text NOT NULL, `icon` text NULL, `emoji` text NULL, `state` text NOT NULL DEFAULT 'active', `created_at` text NOT NULL, `archived_at` text NULL, PRIMARY KEY (`id`), CHECK (length(id) = 26), CHECK (trim(name) <> ''), CHECK (trim(color) <> ''), CHECK (state IN ('active', 'archived')), CHECK ((icon IS NULL) <> (emoji IS NULL)), CHECK ((state = 'active' AND archived_at IS NULL) OR (state = 'archived' AND archived_at IS NOT NULL)));
CREATE UNIQUE INDEX `profiles_name` ON `profiles` (`name`);
INSERT INTO `profiles` (`id`, `name`, `color`, `icon`, `emoji`, `state`, `created_at`, `archived_at`)
VALUES ('00000000000000000000000000', 'default', '#8E8EB5', 'circle', NULL, 'active', '1970-01-01T00:00:00Z', NULL);
-- create "new_bridge_instances" table
CREATE TABLE `new_bridge_instances` (`id` text NULL, `profile_id` text NOT NULL, `scope` text NOT NULL, `workspace_id` text NULL, `platform` text NOT NULL, `extension_name` text NOT NULL, `display_name` text NOT NULL, `source` text NOT NULL DEFAULT 'dynamic', `enabled` boolean NOT NULL DEFAULT 1, `status` text NOT NULL, `dm_policy` text NOT NULL DEFAULT 'open', `routing_policy` text NOT NULL, `provider_config` text NULL, `delivery_defaults` text NULL, `notification_suppress` boolean NOT NULL DEFAULT 0, `degradation_reason` text NULL, `degradation_message` text NULL, `created_at` text NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION);
-- copy rows from old table "bridge_instances" to new temporary table "new_bridge_instances"
INSERT INTO `new_bridge_instances` (`profile_id`, `id`, `scope`, `workspace_id`, `platform`, `extension_name`, `display_name`, `source`, `enabled`, `status`, `dm_policy`, `routing_policy`, `provider_config`, `delivery_defaults`, `notification_suppress`, `degradation_reason`, `degradation_message`, `created_at`, `updated_at`) SELECT '00000000000000000000000000', `id`, `scope`, `workspace_id`, `platform`, `extension_name`, `display_name`, `source`, `enabled`, `status`, `dm_policy`, `routing_policy`, `provider_config`, `delivery_defaults`, `notification_suppress`, `degradation_reason`, `degradation_message`, `created_at`, `updated_at` FROM `bridge_instances`;
-- drop trigger "automation_jobs_profile_owner_active" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `automation_jobs_profile_owner_active`;
-- drop trigger "automation_jobs_profile_owner_immutable" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `automation_jobs_profile_owner_immutable`;
-- drop trigger "automation_suggestions_profile_owner_active" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `automation_suggestions_profile_owner_active`;
-- drop trigger "automation_suggestions_profile_owner_immutable" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `automation_suggestions_profile_owner_immutable`;
-- drop trigger "automation_triggers_profile_owner_active" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `automation_triggers_profile_owner_active`;
-- drop trigger "automation_triggers_profile_owner_immutable" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `automation_triggers_profile_owner_immutable`;
-- drop trigger "automation_watch_events_after_insert" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `automation_watch_events_after_insert`;
-- drop trigger "automation_watch_events_after_terminal_update" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `automation_watch_events_after_terminal_update`;
-- drop trigger "bridge_instances_profile_owner_active" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `bridge_instances_profile_owner_active`;
-- drop trigger "bridge_instances_profile_owner_immutable" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `bridge_instances_profile_owner_immutable`;
-- drop trigger "cmd_palette_pins_profile_lens_insert" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `cmd_palette_pins_profile_lens_insert`;
-- drop trigger "cmd_palette_query_hits_profile_lens_insert" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `cmd_palette_query_hits_profile_lens_insert`;
-- drop trigger "cmd_palette_usage_profile_lens_insert" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `cmd_palette_usage_profile_lens_insert`;
-- drop trigger "cmd_palette_workspace_delete" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `cmd_palette_workspace_delete`;
-- drop trigger "dead_entities_profile_owner_active" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `dead_entities_profile_owner_active`;
-- drop trigger "dead_entities_profile_owner_immutable" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `dead_entities_profile_owner_immutable`;
-- drop trigger "dead_entities_workspace_insert_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `dead_entities_workspace_insert_guard`;
-- drop trigger "dead_entities_workspace_update_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `dead_entities_workspace_update_guard`;
-- drop trigger "event_summaries_profile_owner_active" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `event_summaries_profile_owner_active`;
-- drop trigger "event_summaries_profile_owner_immutable" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `event_summaries_profile_owner_immutable`;
-- drop trigger "extension_env_bindings_profile_delete" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `extension_env_bindings_profile_delete`;
-- drop trigger "extension_env_bindings_profile_insert" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `extension_env_bindings_profile_insert`;
-- drop trigger "extension_env_bindings_workspace_delete" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `extension_env_bindings_workspace_delete`;
-- drop trigger "gateway_ingress_bridge_resource_identity_update" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `gateway_ingress_bridge_resource_identity_update`;
-- drop trigger "gateway_ingress_resource_delete" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `gateway_ingress_resource_delete`;
-- drop trigger "gateway_ingress_trigger_resource_identity_update" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `gateway_ingress_trigger_resource_identity_update`;
-- drop trigger "loop_runs_profile_owner_active" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `loop_runs_profile_owner_active`;
-- drop trigger "loop_runs_profile_owner_immutable" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `loop_runs_profile_owner_immutable`;
-- drop trigger "network_channels_profile_owner_active" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `network_channels_profile_owner_active`;
-- drop trigger "network_channels_profile_owner_immutable" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `network_channels_profile_owner_immutable`;
-- drop trigger "network_channels_workspace_insert_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `network_channels_workspace_insert_guard`;
-- drop trigger "network_channels_workspace_update_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `network_channels_workspace_update_guard`;
-- drop trigger "network_direct_rooms_profile_owner_active" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `network_direct_rooms_profile_owner_active`;
-- drop trigger "network_direct_rooms_profile_owner_immutable" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `network_direct_rooms_profile_owner_immutable`;
-- drop trigger "network_threads_profile_owner_active" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `network_threads_profile_owner_active`;
-- drop trigger "network_threads_profile_owner_immutable" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `network_threads_profile_owner_immutable`;
-- drop trigger "network_work_profile_owner_active" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `network_work_profile_owner_active`;
-- drop trigger "network_work_profile_owner_immutable" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `network_work_profile_owner_immutable`;
-- drop trigger "no_workspace_data_workspace_delete" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `no_workspace_data_workspace_delete`;
-- drop trigger "notification_cursors_profile_owner_active" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `notification_cursors_profile_owner_active`;
-- drop trigger "notification_cursors_profile_owner_immutable" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `notification_cursors_profile_owner_immutable`;
-- drop trigger "profiles_palette_cleanup" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `profiles_palette_cleanup`;
-- drop trigger "sessions_profile_owner_active" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `sessions_profile_owner_active`;
-- drop trigger "sessions_profile_owner_immutable" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `sessions_profile_owner_immutable`;
-- drop trigger "sessions_workspace_insert_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `sessions_workspace_insert_guard`;
-- drop trigger "sessions_workspace_update_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `sessions_workspace_update_guard`;
-- drop trigger "tasks_profile_owner_active" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `tasks_profile_owner_active`;
-- drop trigger "tasks_profile_owner_immutable" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `tasks_profile_owner_immutable`;
-- drop trigger "token_usage_daily_profile_owner_active" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `token_usage_daily_profile_owner_active`;
-- drop trigger "token_usage_daily_profile_owner_immutable" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `token_usage_daily_profile_owner_immutable`;
-- drop trigger "tool_approval_grants_profile_owner_active" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `tool_approval_grants_profile_owner_active`;
-- drop trigger "tool_approval_grants_profile_owner_immutable" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `tool_approval_grants_profile_owner_immutable`;
-- drop trigger "tool_approval_grants_workspace_insert_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `tool_approval_grants_workspace_insert_guard`;
-- drop trigger "tool_approval_grants_workspace_update_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `tool_approval_grants_workspace_update_guard`;
-- drop trigger "tool_approval_pending_profile_owner_active" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `tool_approval_pending_profile_owner_active`;
-- drop trigger "tool_approval_pending_profile_owner_immutable" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `tool_approval_pending_profile_owner_immutable`;
-- drop trigger "trg_bridge_instance_active_delivery_delete" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `trg_bridge_instance_active_delivery_delete`;
-- drop trigger "trg_bridge_instance_active_delivery_identity" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `trg_bridge_instance_active_delivery_identity`;
-- drop trigger "trg_sessions_archive_insert_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `trg_sessions_archive_insert_guard`;
-- drop trigger "trg_sessions_archive_update_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `trg_sessions_archive_update_guard`;
-- drop trigger "trg_tasks_terminal_command_delete_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `trg_tasks_terminal_command_delete_guard`;
-- drop trigger "worktrees_profile_owner_active" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `worktrees_profile_owner_active`;
-- drop trigger "worktrees_profile_owner_immutable" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `worktrees_profile_owner_immutable`;
-- drop "bridge_instances" table after copying rows
DROP TABLE `bridge_instances`;
-- rename temporary table "new_bridge_instances" to "bridge_instances"
ALTER TABLE `new_bridge_instances` RENAME TO `bridge_instances`;
-- create index "idx_bridge_instances_scope" to table: "bridge_instances"
CREATE INDEX `idx_bridge_instances_scope` ON `bridge_instances` (`scope`, `workspace_id`, `id`);
-- create "new_event_summaries" table
CREATE TABLE `new_event_summaries` (`id` text NULL, `profile_id` text NOT NULL, `session_id` text NOT NULL DEFAULT '', `workspace_id` text NOT NULL DEFAULT '', `worktree_id` text NOT NULL DEFAULT '', `type` text NOT NULL, `agent_name` text NOT NULL DEFAULT '', `content_json` text NOT NULL DEFAULT '', `task_id` text NOT NULL DEFAULT '', `run_id` text NOT NULL DEFAULT '', `workflow_id` text NOT NULL DEFAULT '', `claim_token_hash` text NOT NULL DEFAULT '', `lease_until` text NOT NULL DEFAULT '', `coordinator_session_id` text NOT NULL DEFAULT '', `scheduler_reason` text NOT NULL DEFAULT '', `hook_event` text NOT NULL DEFAULT '', `hook_name` text NOT NULL DEFAULT '', `actor_kind` text NOT NULL DEFAULT '', `actor_id` text NOT NULL DEFAULT '', `release_reason` text NOT NULL DEFAULT '', `parent_session_id` text NOT NULL DEFAULT '', `root_session_id` text NOT NULL DEFAULT '', `spawn_depth` integer NOT NULL DEFAULT 0, `summary` text NULL, `timestamp` text NOT NULL, `provider` text NOT NULL DEFAULT '', `outcome` text NOT NULL DEFAULT 'info', PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION);
-- copy rows from old table "event_summaries" to new temporary table "new_event_summaries"
INSERT INTO `new_event_summaries` (`profile_id`, `id`, `session_id`, `workspace_id`, `worktree_id`, `type`, `agent_name`, `content_json`, `task_id`, `run_id`, `workflow_id`, `claim_token_hash`, `lease_until`, `coordinator_session_id`, `scheduler_reason`, `hook_event`, `hook_name`, `actor_kind`, `actor_id`, `release_reason`, `parent_session_id`, `root_session_id`, `spawn_depth`, `summary`, `timestamp`, `provider`, `outcome`) SELECT '00000000000000000000000000', `id`, `session_id`, `workspace_id`, `worktree_id`, `type`, `agent_name`, `content_json`, `task_id`, `run_id`, `workflow_id`, `claim_token_hash`, `lease_until`, `coordinator_session_id`, `scheduler_reason`, `hook_event`, `hook_name`, `actor_kind`, `actor_id`, `release_reason`, `parent_session_id`, `root_session_id`, `spawn_depth`, `summary`, `timestamp`, `provider`, `outcome` FROM `event_summaries`;
-- drop "event_summaries" table after copying rows
DROP TABLE `event_summaries`;
-- rename temporary table "new_event_summaries" to "event_summaries"
ALTER TABLE `new_event_summaries` RENAME TO `event_summaries`;
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
-- create "new_network_threads" table
CREATE TABLE `new_network_threads` (`profile_id` text NOT NULL, `workspace_id` text NOT NULL, `channel` text NOT NULL, `thread_id` text NOT NULL, `root_message_id` text NOT NULL, `title` text NOT NULL DEFAULT '', `opened_by_peer_id` text NOT NULL DEFAULT '', `opened_session_id` text NOT NULL DEFAULT '', `opened_at` text NOT NULL, `last_activity_at` text NOT NULL, `message_count` integer NOT NULL DEFAULT 0, `participant_count` integer NOT NULL DEFAULT 0, `open_work_count` integer NOT NULL DEFAULT 0, `last_message_preview` text NOT NULL DEFAULT '', `opened_sequence` integer NOT NULL DEFAULT 0, `last_activity_sequence` integer NOT NULL DEFAULT 0, PRIMARY KEY (`workspace_id`, `channel`, `thread_id`), CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (message_count >= 0), CHECK (participant_count >= 0), CHECK (open_work_count >= 0));
-- copy rows from old table "network_threads" to new temporary table "new_network_threads"
INSERT INTO `new_network_threads` (`profile_id`, `workspace_id`, `channel`, `thread_id`, `root_message_id`, `title`, `opened_by_peer_id`, `opened_session_id`, `opened_at`, `last_activity_at`, `message_count`, `participant_count`, `open_work_count`, `last_message_preview`, `opened_sequence`, `last_activity_sequence`) SELECT '00000000000000000000000000', `workspace_id`, `channel`, `thread_id`, `root_message_id`, `title`, `opened_by_peer_id`, `opened_session_id`, `opened_at`, `last_activity_at`, `message_count`, `participant_count`, `open_work_count`, `last_message_preview`, `opened_sequence`, `last_activity_sequence` FROM `network_threads`;
-- drop "network_threads" table after copying rows
DROP TABLE `network_threads`;
-- rename temporary table "new_network_threads" to "network_threads"
ALTER TABLE `new_network_threads` RENAME TO `network_threads`;
-- create index "idx_network_threads_activity" to table: "network_threads"
CREATE INDEX `idx_network_threads_activity` ON `network_threads` (`workspace_id`, `channel`, `last_activity_sequence` DESC, `thread_id`);
-- create index "idx_network_threads_created" to table: "network_threads"
CREATE INDEX `idx_network_threads_created` ON `network_threads` (`workspace_id`, `channel`, `opened_sequence`, `thread_id`);
-- create index "idx_network_threads_open_work" to table: "network_threads"
CREATE INDEX `idx_network_threads_open_work` ON `network_threads` (`workspace_id`, `channel`, `open_work_count`, `last_activity_sequence` DESC, `thread_id`);
-- create index "idx_network_threads_title" to table: "network_threads"
CREATE INDEX `idx_network_threads_title` ON `network_threads` (`workspace_id`, `channel`, `title`, `thread_id`);
-- create "new_notification_presets" table
CREATE TEMP TABLE `profile_migration_disabled_notification_presets` AS
SELECT `name` FROM `notification_presets` WHERE `enabled` = 0;
CREATE TABLE `new_notification_presets` (`name` text NULL, `events` text NOT NULL, `targets` text NOT NULL, `filter` text NOT NULL DEFAULT '', `built_in` boolean NOT NULL DEFAULT 0, `default_version` text NOT NULL DEFAULT '', `default_hash` text NOT NULL DEFAULT '', `user_modified` boolean NOT NULL DEFAULT 0, `default_update_available` boolean NOT NULL DEFAULT 0, `created_at` text NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`name`), CHECK (trim(name) <> ''), CHECK (json_valid(events)), CHECK (json_valid(targets)), CHECK (built_in IN (0, 1)), CHECK (user_modified IN (0, 1)), CHECK (default_update_available IN (0, 1)));
-- copy rows from old table "notification_presets" to new temporary table "new_notification_presets"
INSERT INTO `new_notification_presets` (`name`, `events`, `targets`, `filter`, `built_in`, `default_version`, `default_hash`, `user_modified`, `default_update_available`, `created_at`, `updated_at`) SELECT `name`, `events`, `targets`, `filter`, `built_in`, `default_version`, `default_hash`, `user_modified`, `default_update_available`, `created_at`, `updated_at` FROM `notification_presets`;
-- drop "notification_presets" table after copying rows
DROP TABLE `notification_presets`;
-- rename temporary table "new_notification_presets" to "notification_presets"
ALTER TABLE `new_notification_presets` RENAME TO `notification_presets`;
-- create index "idx_notification_presets_builtin" to table: "notification_presets"
CREATE INDEX `idx_notification_presets_builtin` ON `notification_presets` (`built_in`, `name`);
-- create "new_resource_records" table
CREATE TABLE `new_resource_records` (`kind` text NOT NULL, `id` text NOT NULL, `version` integer NOT NULL, `scope_kind` text NOT NULL, `scope_id` text NULL, `owner_kind` text NOT NULL, `owner_id` text NOT NULL, `source_kind` text NOT NULL, `source_id` text NOT NULL, `spec_json` text NOT NULL, `created_at` text NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`kind`, `id`), CHECK (scope_kind IN ('user', 'workspace', 'profile', 'workspace_profile')), CHECK (
			(scope_kind = 'user' AND scope_id IS NULL) OR
			(scope_kind IN ('workspace', 'profile', 'workspace_profile') AND scope_id IS NOT NULL)
		));
-- copy rows from old table "resource_records" to new temporary table "new_resource_records"
INSERT INTO `new_resource_records` (`kind`, `id`, `version`, `scope_kind`, `scope_id`, `owner_kind`, `owner_id`, `source_kind`, `source_id`, `spec_json`, `created_at`, `updated_at`) SELECT `kind`, `id`, `version`, CASE WHEN `scope_kind` = 'global' THEN 'user' ELSE `scope_kind` END, `scope_id`, `owner_kind`, `owner_id`, `source_kind`, `source_id`, `spec_json`, `created_at`, `updated_at` FROM `resource_records`;
-- drop "resource_records" table after copying rows
DROP TABLE `resource_records`;
-- rename temporary table "new_resource_records" to "resource_records"
ALTER TABLE `new_resource_records` RENAME TO `resource_records`;
-- create index "idx_resource_kind" to table: "resource_records"
CREATE INDEX `idx_resource_kind` ON `resource_records` (`kind`);
-- create index "idx_resource_owner" to table: "resource_records"
CREATE INDEX `idx_resource_owner` ON `resource_records` (`owner_kind`, `owner_id`, `kind`);
-- create index "idx_resource_scope" to table: "resource_records"
CREATE INDEX `idx_resource_scope` ON `resource_records` (`scope_kind`, `scope_id`, `kind`);
-- create index "idx_resource_source" to table: "resource_records"
CREATE INDEX `idx_resource_source` ON `resource_records` (`source_kind`, `source_id`, `kind`);
-- create "new_tasks" table
CREATE TABLE `new_tasks` (`id` text NULL, `profile_id` text NOT NULL, `identifier` text NULL, `scope` text NOT NULL, `workspace_id` text NULL, `parent_task_id` text NULL, `title` text NOT NULL, `description` text NULL, `priority` text NOT NULL DEFAULT 'medium', `max_attempts` integer NOT NULL DEFAULT 3, `status` text NOT NULL, `approval_policy` text NOT NULL DEFAULT 'none', `approval_state` text NOT NULL DEFAULT 'not_required', `owner_kind` text NULL, `owner_ref` text NULL, `created_by_kind` text NOT NULL, `created_by_ref` text NOT NULL, `origin_kind` text NOT NULL, `origin_ref` text NOT NULL, `created_at` text NOT NULL, `updated_at` text NOT NULL, `closed_at` text NULL, `metadata_json` text NULL, `current_run_id` text NULL, `paused` integer NOT NULL DEFAULT 0, `paused_by` text NOT NULL DEFAULT '', `paused_at` text NULL, `paused_reason` text NOT NULL DEFAULT '', `max_runtime_seconds` integer NOT NULL DEFAULT 0, `spawn_failure_count` integer NOT NULL DEFAULT 0, `last_spawn_error` text NOT NULL DEFAULT '', `review_policy` text NOT NULL DEFAULT 'none', `review_max_rounds` integer NOT NULL DEFAULT 3, `review_round` integer NOT NULL DEFAULT 0, `last_review_id` text NULL, `last_review_outcome` text NULL, `review_circuit_opened_at` text NULL, `review_circuit_reason` text NULL, `auto_enqueue_on_ready` integer NOT NULL DEFAULT 0, `needs_attention_reason` text NULL, `needs_attention_at` text NULL, `needs_attention_by_kind` text NULL, `needs_attention_by_ref` text NULL, `wake_creator` integer NOT NULL DEFAULT 1, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`current_run_id`) REFERENCES `task_runs` (`id`) ON UPDATE NO ACTION ON DELETE SET NULL, CONSTRAINT `1` FOREIGN KEY (`parent_task_id`) REFERENCES `tasks` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `2` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `3` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (scope IN ('global', 'workspace')), CHECK (
			priority IN ('low', 'medium', 'high', 'urgent')
		), CHECK (max_attempts > 0 AND max_attempts <= 10), CHECK (
			approval_policy IN ('none', 'manual')
		), CHECK (
			approval_state IN ('not_required', 'pending', 'approved', 'rejected')
		), CHECK (
			owner_kind IS NULL OR owner_kind IN (
				'human', 'agent_session', 'automation', 'extension', 'network_peer', 'pool'
			)
		), CHECK (
			created_by_kind IN (
				'human', 'agent_session', 'automation', 'extension', 'network_peer', 'daemon'
			)
		), CHECK (
			origin_kind IN (
				'cli', 'web', 'uds', 'http', 'automation', 'extension', 'network', 'agent_session', 'daemon'
			)
		), CHECK (paused IN (0, 1)), CHECK (max_runtime_seconds >= 0), CHECK (spawn_failure_count >= 0), CHECK (
			review_policy IN ('none', 'on_success', 'on_failure', 'always')
		), CHECK (review_max_rounds >= 0), CHECK (review_round >= 0), CHECK (
			last_review_outcome IS NULL OR last_review_outcome IN (
				'approved', 'rejected', 'blocked', 'error', 'timeout', 'invalid_output'
			)
		), CHECK (auto_enqueue_on_ready IN (0, 1)), CHECK (
			(scope = 'global' AND workspace_id IS NULL) OR
			(scope = 'workspace' AND workspace_id IS NOT NULL)
		), CHECK (
			(owner_kind IS NULL AND owner_ref IS NULL) OR
			(owner_kind IS NOT NULL AND owner_ref IS NOT NULL)
		), CHECK (parent_task_id IS NULL OR parent_task_id <> id), CHECK (
			(approval_policy = 'none' AND approval_state = 'not_required') OR
			(approval_policy = 'manual' AND approval_state IN ('pending', 'approved', 'rejected'))
		));
-- copy rows from old table "tasks" to new temporary table "new_tasks"
INSERT INTO `new_tasks` (`profile_id`, `id`, `identifier`, `scope`, `workspace_id`, `parent_task_id`, `title`, `description`, `priority`, `max_attempts`, `status`, `approval_policy`, `approval_state`, `owner_kind`, `owner_ref`, `created_by_kind`, `created_by_ref`, `origin_kind`, `origin_ref`, `created_at`, `updated_at`, `closed_at`, `metadata_json`, `current_run_id`, `paused`, `paused_by`, `paused_at`, `paused_reason`, `max_runtime_seconds`, `spawn_failure_count`, `last_spawn_error`, `review_policy`, `review_max_rounds`, `review_round`, `last_review_id`, `last_review_outcome`, `review_circuit_opened_at`, `review_circuit_reason`, `auto_enqueue_on_ready`, `needs_attention_reason`, `needs_attention_at`, `needs_attention_by_kind`, `needs_attention_by_ref`, `wake_creator`) SELECT '00000000000000000000000000', `id`, `identifier`, `scope`, `workspace_id`, `parent_task_id`, `title`, `description`, `priority`, `max_attempts`, `status`, `approval_policy`, `approval_state`, `owner_kind`, `owner_ref`, `created_by_kind`, `created_by_ref`, `origin_kind`, `origin_ref`, `created_at`, `updated_at`, `closed_at`, `metadata_json`, `current_run_id`, `paused`, `paused_by`, `paused_at`, `paused_reason`, `max_runtime_seconds`, `spawn_failure_count`, `last_spawn_error`, `review_policy`, `review_max_rounds`, `review_round`, `last_review_id`, `last_review_outcome`, `review_circuit_opened_at`, `review_circuit_reason`, `auto_enqueue_on_ready`, `needs_attention_reason`, `needs_attention_at`, `needs_attention_by_kind`, `needs_attention_by_ref`, `wake_creator` FROM `tasks`;
-- drop "tasks" table after copying rows
DROP TABLE `tasks`;
-- rename temporary table "new_tasks" to "tasks"
ALTER TABLE `new_tasks` RENAME TO `tasks`;
-- create index "idx_tasks_approval_state" to table: "tasks"
CREATE INDEX `idx_tasks_approval_state` ON `tasks` (`approval_state`);
-- create index "idx_tasks_created_by" to table: "tasks"
CREATE INDEX `idx_tasks_created_by` ON `tasks` (`created_by_kind`, `created_by_ref`);
-- create index "idx_tasks_current_run" to table: "tasks"
CREATE INDEX `idx_tasks_current_run` ON `tasks` (`current_run_id`);
-- create index "idx_tasks_owner" to table: "tasks"
CREATE INDEX `idx_tasks_owner` ON `tasks` (`owner_kind`, `owner_ref`);
-- create index "idx_tasks_parent" to table: "tasks"
CREATE INDEX `idx_tasks_parent` ON `tasks` (`parent_task_id`);
-- create index "idx_tasks_paused" to table: "tasks"
CREATE INDEX `idx_tasks_paused` ON `tasks` (`paused`, `updated_at` DESC);
-- create index "idx_tasks_priority" to table: "tasks"
CREATE INDEX `idx_tasks_priority` ON `tasks` (`priority`);
-- create index "idx_tasks_review_policy" to table: "tasks"
CREATE INDEX `idx_tasks_review_policy` ON `tasks` (`review_policy`);
-- create index "idx_tasks_review_round" to table: "tasks"
CREATE INDEX `idx_tasks_review_round` ON `tasks` (`review_round`);
-- create index "idx_tasks_scope" to table: "tasks"
CREATE INDEX `idx_tasks_scope` ON `tasks` (`scope`);
-- create index "idx_tasks_status" to table: "tasks"
CREATE INDEX `idx_tasks_status` ON `tasks` (`status`);
-- create index "idx_tasks_workspace" to table: "tasks"
CREATE INDEX `idx_tasks_workspace` ON `tasks` (`workspace_id`);
-- create "new_mcp_auth_tokens" table
CREATE TABLE `new_mcp_auth_tokens` (`scope` text NOT NULL, `workspace_id` text NOT NULL DEFAULT '', `server_name` text NOT NULL, `definition_fingerprint` text NOT NULL DEFAULT '', `issuer` text NOT NULL DEFAULT '', `client_id` text NOT NULL, `scopes_json` text NOT NULL DEFAULT '[]', `access_token_ref` text NOT NULL, `refresh_token_ref` text NOT NULL DEFAULT '', `token_type` text NOT NULL DEFAULT 'Bearer', `expires_at` text NULL, `obtained_at` text NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`scope`, `workspace_id`, `server_name`), CHECK (scope IN ('user', 'workspace', 'profile', 'workspace_profile')), CHECK (trim(server_name) <> ''), CHECK (
				(scope = 'user' AND workspace_id = '') OR
				(scope IN ('workspace', 'profile', 'workspace_profile') AND trim(workspace_id) <> '')
			));
-- copy rows from old table "mcp_auth_tokens" to new temporary table "new_mcp_auth_tokens"
INSERT INTO `new_mcp_auth_tokens` (`scope`, `workspace_id`, `server_name`, `definition_fingerprint`, `issuer`, `client_id`, `scopes_json`, `access_token_ref`, `refresh_token_ref`, `token_type`, `expires_at`, `obtained_at`, `updated_at`) SELECT CASE WHEN `scope` = 'global' THEN 'user' ELSE `scope` END, `workspace_id`, `server_name`, `definition_fingerprint`, `issuer`, `client_id`, `scopes_json`, `access_token_ref`, `refresh_token_ref`, `token_type`, `expires_at`, `obtained_at`, `updated_at` FROM `mcp_auth_tokens`;
-- drop "mcp_auth_tokens" table after copying rows
DROP TABLE `mcp_auth_tokens`;
-- rename temporary table "new_mcp_auth_tokens" to "mcp_auth_tokens"
ALTER TABLE `new_mcp_auth_tokens` RENAME TO `mcp_auth_tokens`;
-- create index "idx_mcp_auth_tokens_updated_at" to table: "mcp_auth_tokens"
CREATE INDEX `idx_mcp_auth_tokens_updated_at` ON `mcp_auth_tokens` (`updated_at`);
-- create "new_network_direct_rooms" table
CREATE TABLE `new_network_direct_rooms` (`profile_id` text NOT NULL, `workspace_id` text NOT NULL, `channel` text NOT NULL, `direct_id` text NOT NULL, `session_a` text NOT NULL, `session_b` text NOT NULL, `opened_at` text NOT NULL, `last_activity_at` text NOT NULL, `message_count` integer NOT NULL DEFAULT 0, `open_work_count` integer NOT NULL DEFAULT 0, `last_message_preview` text NOT NULL DEFAULT '', `opened_sequence` integer NOT NULL DEFAULT 0, `last_activity_sequence` integer NOT NULL DEFAULT 0, PRIMARY KEY (`workspace_id`, `channel`, `direct_id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`, `session_b`) REFERENCES `sessions` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`workspace_id`, `session_a`) REFERENCES `sessions` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `2` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (message_count >= 0), CHECK (open_work_count >= 0), CHECK (session_a < session_b));
-- copy rows from old table "network_direct_rooms" to new temporary table "new_network_direct_rooms"
INSERT INTO `new_network_direct_rooms` (`profile_id`, `workspace_id`, `channel`, `direct_id`, `session_a`, `session_b`, `opened_at`, `last_activity_at`, `message_count`, `open_work_count`, `last_message_preview`, `opened_sequence`, `last_activity_sequence`) SELECT '00000000000000000000000000', `workspace_id`, `channel`, `direct_id`, `session_a`, `session_b`, `opened_at`, `last_activity_at`, `message_count`, `open_work_count`, `last_message_preview`, `opened_sequence`, `last_activity_sequence` FROM `network_direct_rooms`;
-- drop "network_direct_rooms" table after copying rows
DROP TABLE `network_direct_rooms`;
-- rename temporary table "new_network_direct_rooms" to "network_direct_rooms"
ALTER TABLE `new_network_direct_rooms` RENAME TO `network_direct_rooms`;
-- create index "network_direct_rooms_workspace_id_channel_session_a_session_b" to table: "network_direct_rooms"
CREATE UNIQUE INDEX `network_direct_rooms_workspace_id_channel_session_a_session_b` ON `network_direct_rooms` (`workspace_id`, `channel`, `session_a`, `session_b`);
-- create index "idx_network_direct_rooms_activity" to table: "network_direct_rooms"
CREATE INDEX `idx_network_direct_rooms_activity` ON `network_direct_rooms` (`workspace_id`, `channel`, `last_activity_sequence` DESC, `direct_id`);
-- create index "idx_network_direct_rooms_created" to table: "network_direct_rooms"
CREATE INDEX `idx_network_direct_rooms_created` ON `network_direct_rooms` (`workspace_id`, `channel`, `opened_at`, `direct_id`);
-- create index "idx_network_direct_rooms_open_work" to table: "network_direct_rooms"
CREATE INDEX `idx_network_direct_rooms_open_work` ON `network_direct_rooms` (`workspace_id`, `channel`, `open_work_count`, `last_activity_sequence` DESC, `direct_id`);
-- create index "idx_network_direct_rooms_session_a" to table: "network_direct_rooms"
CREATE INDEX `idx_network_direct_rooms_session_a` ON `network_direct_rooms` (`workspace_id`, `channel`, `session_a`, `last_activity_sequence` DESC);
-- create index "idx_network_direct_rooms_session_b" to table: "network_direct_rooms"
CREATE INDEX `idx_network_direct_rooms_session_b` ON `network_direct_rooms` (`workspace_id`, `channel`, `session_b`, `last_activity_sequence` DESC);
-- create "new_network_work" table
CREATE TABLE `new_network_work` (`work_id` text NOT NULL, `profile_id` text NOT NULL, `workspace_id` text NOT NULL, `channel` text NOT NULL, `surface` text NOT NULL, `thread_id` text NULL, `direct_id` text NULL, `opened_by_session_id` text NOT NULL, `target_session_id` text NULL, `state` text NOT NULL, `opened_at` text NOT NULL, `last_activity_at` text NOT NULL, `terminal_at` text NULL, PRIMARY KEY (`work_id`, `workspace_id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`, `target_session_id`) REFERENCES `sessions` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`workspace_id`, `opened_by_session_id`) REFERENCES `sessions` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `2` FOREIGN KEY (`workspace_id`, `channel`, `direct_id`) REFERENCES `network_direct_rooms` (`workspace_id`, `channel`, `direct_id`) ON UPDATE NO ACTION ON DELETE RESTRICT, CONSTRAINT `3` FOREIGN KEY (`workspace_id`, `channel`, `thread_id`) REFERENCES `network_threads` (`workspace_id`, `channel`, `thread_id`) ON UPDATE NO ACTION ON DELETE RESTRICT, CONSTRAINT `4` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (surface IN ('thread', 'direct')), CHECK (
			state IN ('submitted', 'working', 'needs_input', 'completed', 'failed', 'canceled')
		), CHECK (
			(surface = 'thread' AND thread_id IS NOT NULL AND direct_id IS NULL)
			OR (surface = 'direct' AND direct_id IS NOT NULL AND thread_id IS NULL)
		));
-- copy rows from old table "network_work" to new temporary table "new_network_work"
INSERT INTO `new_network_work` (`profile_id`, `work_id`, `workspace_id`, `channel`, `surface`, `thread_id`, `direct_id`, `opened_by_session_id`, `target_session_id`, `state`, `opened_at`, `last_activity_at`, `terminal_at`) SELECT '00000000000000000000000000', `work_id`, `workspace_id`, `channel`, `surface`, `thread_id`, `direct_id`, `opened_by_session_id`, `target_session_id`, `state`, `opened_at`, `last_activity_at`, `terminal_at` FROM `network_work`;
-- drop "network_work" table after copying rows
DROP TABLE `network_work`;
-- rename temporary table "new_network_work" to "network_work"
ALTER TABLE `new_network_work` RENAME TO `network_work`;
-- create index "idx_network_work_conversation" to table: "network_work"
CREATE INDEX `idx_network_work_conversation` ON `network_work` (`workspace_id`, `channel`, `surface`, `thread_id`, `direct_id`, `last_activity_at` DESC);
-- create index "idx_network_work_state" to table: "network_work"
CREATE INDEX `idx_network_work_state` ON `network_work` (`workspace_id`, `state`, `last_activity_at` DESC);
-- create "new_automation_jobs" table
CREATE TABLE `new_automation_jobs` (`id` text NULL, `profile_id` text NOT NULL, `scope` text NOT NULL, `name` text NOT NULL, `agent_name` text NOT NULL, `workspace_id` text NULL, `prompt` text NOT NULL, `schedule` text NULL, `task` text NULL, `enabled` boolean NOT NULL DEFAULT 1, `retry` text NOT NULL, `fire_limit` text NOT NULL, `source` text NOT NULL DEFAULT 'dynamic', `target_kind` text NOT NULL DEFAULT 'agent', `loop_workspace_id` text NULL, `loop_name` text NULL, `loop_inputs` text NULL, `loop_input_mapping` text NULL, `loop_network_participation` text NULL, `created_at` text NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`loop_workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `2` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (scope IN ('global', 'workspace')), CHECK (target_kind IN ('agent', 'loop')), CHECK (
			loop_network_participation IS NULL OR json_valid(loop_network_participation)
		), CHECK (
			(scope = 'global' AND workspace_id IS NULL) OR
			(scope = 'workspace' AND workspace_id IS NOT NULL)
		));
-- copy rows from old table "automation_jobs" to new temporary table "new_automation_jobs"
INSERT INTO `new_automation_jobs` (`profile_id`, `id`, `scope`, `name`, `agent_name`, `workspace_id`, `prompt`, `schedule`, `task`, `enabled`, `retry`, `fire_limit`, `source`, `target_kind`, `loop_workspace_id`, `loop_name`, `loop_inputs`, `loop_input_mapping`, `loop_network_participation`, `created_at`, `updated_at`) SELECT '00000000000000000000000000', `id`, `scope`, `name`, `agent_name`, `workspace_id`, `prompt`, `schedule`, `task`, `enabled`, `retry`, `fire_limit`, `source`, `target_kind`, `loop_workspace_id`, `loop_name`, `loop_inputs`, `loop_input_mapping`, `loop_network_participation`, `created_at`, `updated_at` FROM `automation_jobs`;
-- drop "automation_jobs" table after copying rows
DROP TABLE `automation_jobs`;
-- rename temporary table "new_automation_jobs" to "automation_jobs"
ALTER TABLE `new_automation_jobs` RENAME TO `automation_jobs`;
-- create index "idx_automation_jobs_enabled" to table: "automation_jobs"
CREATE INDEX `idx_automation_jobs_enabled` ON `automation_jobs` (`enabled`);
-- create index "idx_automation_jobs_loop_target" to table: "automation_jobs"
CREATE INDEX `idx_automation_jobs_loop_target` ON `automation_jobs` (`loop_name`, `loop_workspace_id`) WHERE target_kind = 'loop';
-- create index "uq_automation_jobs_global_name" to table: "automation_jobs"
CREATE UNIQUE INDEX `uq_automation_jobs_global_name` ON `automation_jobs` (`name`) WHERE scope = 'global';
-- create index "uq_automation_jobs_workspace_name" to table: "automation_jobs"
CREATE UNIQUE INDEX `uq_automation_jobs_workspace_name` ON `automation_jobs` (`workspace_id`, `name`) WHERE scope = 'workspace';
-- create "new_automation_triggers" table
CREATE TABLE `new_automation_triggers` (`id` text NULL, `profile_id` text NOT NULL, `scope` text NOT NULL, `name` text NOT NULL, `agent_name` text NOT NULL, `workspace_id` text NULL, `prompt` text NOT NULL, `event` text NOT NULL, `filter` text NULL, `enabled` boolean NOT NULL DEFAULT 1, `retry` text NOT NULL, `fire_limit` text NOT NULL, `source` text NOT NULL DEFAULT 'dynamic', `webhook_id` text NULL, `endpoint_slug` text NULL, `webhook_secret_ref` text NULL, `target_kind` text NOT NULL DEFAULT 'agent', `loop_workspace_id` text NULL, `loop_name` text NULL, `loop_inputs` text NULL, `loop_input_mapping` text NULL, `loop_network_participation` text NULL, `created_at` text NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`loop_workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `2` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (scope IN ('global', 'workspace')), CHECK (target_kind IN ('agent', 'loop')), CHECK (
			loop_network_participation IS NULL OR json_valid(loop_network_participation)
		), CHECK (
			(scope = 'global' AND workspace_id IS NULL) OR
			(scope = 'workspace' AND workspace_id IS NOT NULL)
		));
-- copy rows from old table "automation_triggers" to new temporary table "new_automation_triggers"
INSERT INTO `new_automation_triggers` (`profile_id`, `id`, `scope`, `name`, `agent_name`, `workspace_id`, `prompt`, `event`, `filter`, `enabled`, `retry`, `fire_limit`, `source`, `webhook_id`, `endpoint_slug`, `webhook_secret_ref`, `target_kind`, `loop_workspace_id`, `loop_name`, `loop_inputs`, `loop_input_mapping`, `loop_network_participation`, `created_at`, `updated_at`) SELECT '00000000000000000000000000', `id`, `scope`, `name`, `agent_name`, `workspace_id`, `prompt`, `event`, `filter`, `enabled`, `retry`, `fire_limit`, `source`, `webhook_id`, `endpoint_slug`, `webhook_secret_ref`, `target_kind`, `loop_workspace_id`, `loop_name`, `loop_inputs`, `loop_input_mapping`, `loop_network_participation`, `created_at`, `updated_at` FROM `automation_triggers`;
-- drop "automation_triggers" table after copying rows
DROP TABLE `automation_triggers`;
-- rename temporary table "new_automation_triggers" to "automation_triggers"
ALTER TABLE `new_automation_triggers` RENAME TO `automation_triggers`;
-- create index "idx_automation_triggers_enabled" to table: "automation_triggers"
CREATE INDEX `idx_automation_triggers_enabled` ON `automation_triggers` (`enabled`);
-- create index "idx_automation_triggers_event" to table: "automation_triggers"
CREATE INDEX `idx_automation_triggers_event` ON `automation_triggers` (`event`);
-- create index "idx_automation_triggers_loop_target" to table: "automation_triggers"
CREATE INDEX `idx_automation_triggers_loop_target` ON `automation_triggers` (`loop_name`, `loop_workspace_id`) WHERE target_kind = 'loop';
-- create index "uq_automation_triggers_global_name" to table: "automation_triggers"
CREATE UNIQUE INDEX `uq_automation_triggers_global_name` ON `automation_triggers` (`name`) WHERE scope = 'global';
-- create index "uq_automation_triggers_webhook_id" to table: "automation_triggers"
CREATE UNIQUE INDEX `uq_automation_triggers_webhook_id` ON `automation_triggers` (`webhook_id`) WHERE webhook_id IS NOT NULL;
-- create index "uq_automation_triggers_workspace_name" to table: "automation_triggers"
CREATE UNIQUE INDEX `uq_automation_triggers_workspace_name` ON `automation_triggers` (`workspace_id`, `name`) WHERE scope = 'workspace';
-- create "new_token_usage_daily" table
CREATE TABLE `new_token_usage_daily` (`day` text NOT NULL, `profile_id` text NOT NULL, `workspace_id` text NOT NULL DEFAULT '', `agent_name` text NOT NULL DEFAULT '', `input_tokens` integer NOT NULL DEFAULT 0, `output_tokens` integer NOT NULL DEFAULT 0, `total_tokens` integer NOT NULL DEFAULT 0, `total_cost` real NULL, `cost_currency` text NULL, `cost_status` text NOT NULL DEFAULT 'unknown', `cost_source` text NOT NULL DEFAULT 'none', `turn_count` integer NOT NULL DEFAULT 0, `updated_at` text NOT NULL, PRIMARY KEY (`day`, `profile_id`, `workspace_id`, `agent_name`), CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (length(day) = 10), CHECK (input_tokens >= 0), CHECK (output_tokens >= 0), CHECK (total_tokens >= 0), CHECK (cost_status IN ('actual', 'estimated', 'included', 'unknown')), CHECK (cost_source IN ('agent_reported', 'catalog_config', 'models_dev', 'builtin', 'none')), CHECK (turn_count >= 0));
-- copy rows from old table "token_usage_daily" to new temporary table "new_token_usage_daily"
INSERT INTO `new_token_usage_daily` (`profile_id`, `day`, `workspace_id`, `agent_name`, `input_tokens`, `output_tokens`, `total_tokens`, `total_cost`, `cost_currency`, `cost_status`, `cost_source`, `turn_count`, `updated_at`) SELECT '00000000000000000000000000', `day`, `workspace_id`, `agent_name`, `input_tokens`, `output_tokens`, `total_tokens`, `total_cost`, `cost_currency`, `cost_status`, `cost_source`, `turn_count`, `updated_at` FROM `token_usage_daily`;
-- drop "token_usage_daily" table after copying rows
DROP TABLE `token_usage_daily`;
-- rename temporary table "new_token_usage_daily" to "token_usage_daily"
ALTER TABLE `new_token_usage_daily` RENAME TO `token_usage_daily`;
-- create index "idx_token_usage_daily_profile_day" to table: "token_usage_daily"
CREATE INDEX `idx_token_usage_daily_profile_day` ON `token_usage_daily` (`profile_id`, `day`);
-- create index "idx_token_usage_daily_workspace" to table: "token_usage_daily"
CREATE INDEX `idx_token_usage_daily_workspace` ON `token_usage_daily` (`workspace_id`, `day`);
-- create "new_extensions" table
CREATE TEMP TABLE `profile_migration_disabled_extensions` AS
SELECT `name` FROM `extensions` WHERE `enabled` = 0;
CREATE TABLE `new_extensions` (`name` text NULL, `version` text NOT NULL, `source` text NOT NULL, `manifest_path` text NOT NULL, `format` text NOT NULL DEFAULT 'compozy', `ingest_diagnostics_json` text NOT NULL DEFAULT '[]', `installed_at` text NOT NULL, `provides_json` text NOT NULL DEFAULT '[]', `permissions_json` text NOT NULL DEFAULT '[]', `checksum` text NOT NULL, `registry_slug` text NULL, `registry_name` text NULL, `remote_version` text NULL, `provenance_json` text NOT NULL DEFAULT '{}', `network_requirement_digest` text NOT NULL DEFAULT '', `network_confirmed_by` text NULL, `network_confirmed_at` text NULL, PRIMARY KEY (`name`));
-- copy rows from old table "extensions" to new temporary table "new_extensions"
INSERT INTO `new_extensions` (`name`, `version`, `source`, `manifest_path`, `format`, `ingest_diagnostics_json`, `installed_at`, `provides_json`, `permissions_json`, `checksum`, `registry_slug`, `registry_name`, `remote_version`, `provenance_json`, `network_requirement_digest`, `network_confirmed_by`, `network_confirmed_at`) SELECT `name`, `version`, `source`, `manifest_path`, `format`, `ingest_diagnostics_json`, `installed_at`, `provides_json`, `permissions_json`, `checksum`, `registry_slug`, `registry_name`, `remote_version`, `provenance_json`, `network_requirement_digest`, `network_confirmed_by`, `network_confirmed_at` FROM `extensions`;
-- drop "extensions" table after copying rows
DROP TABLE `extensions`;
-- rename temporary table "new_extensions" to "extensions"
ALTER TABLE `new_extensions` RENAME TO `extensions`;
-- create "new_mcp_oauth_registrations" table
CREATE TABLE `new_mcp_oauth_registrations` (`scope` text NOT NULL, `workspace_id` text NOT NULL DEFAULT '', `server_name` text NOT NULL, `definition_fingerprint` text NOT NULL, `resource_url` text NOT NULL, `issuer` text NOT NULL, `client_id` text NOT NULL, `token_endpoint_auth_method` text NOT NULL DEFAULT '', `client_secret_ref` text NOT NULL DEFAULT '', `registration_access_token_ref` text NOT NULL DEFAULT '', `registration_client_uri` text NOT NULL DEFAULT '', `client_id_issued_at` text NULL, `client_secret_expires_at` text NULL, `redirect_uri` text NOT NULL, `scopes_json` text NOT NULL DEFAULT '[]', `updated_at` text NOT NULL, PRIMARY KEY (`scope`, `workspace_id`, `server_name`), CHECK (scope IN ('user', 'workspace', 'profile', 'workspace_profile')), CHECK (trim(server_name) <> ''), CHECK (trim(definition_fingerprint) <> ''), CHECK (trim(resource_url) <> ''), CHECK (trim(issuer) <> ''), CHECK (trim(client_id) <> ''), CHECK (trim(redirect_uri) <> ''), CHECK (json_valid(scopes_json)), CHECK (trim(updated_at) <> ''), CHECK (
				(scope = 'user' AND workspace_id = '') OR
				(scope IN ('workspace', 'profile', 'workspace_profile') AND trim(workspace_id) <> '')
			), CHECK (
				(registration_access_token_ref = '' AND registration_client_uri = '') OR
				(trim(registration_access_token_ref) <> '' AND trim(registration_client_uri) <> '')
			));
-- copy rows from old table "mcp_oauth_registrations" to new temporary table "new_mcp_oauth_registrations"
INSERT INTO `new_mcp_oauth_registrations` (`scope`, `workspace_id`, `server_name`, `definition_fingerprint`, `resource_url`, `issuer`, `client_id`, `token_endpoint_auth_method`, `client_secret_ref`, `registration_access_token_ref`, `registration_client_uri`, `client_id_issued_at`, `client_secret_expires_at`, `redirect_uri`, `scopes_json`, `updated_at`) SELECT CASE WHEN `scope` = 'global' THEN 'user' ELSE `scope` END, `workspace_id`, `server_name`, `definition_fingerprint`, `resource_url`, `issuer`, `client_id`, `token_endpoint_auth_method`, `client_secret_ref`, `registration_access_token_ref`, `registration_client_uri`, `client_id_issued_at`, `client_secret_expires_at`, `redirect_uri`, `scopes_json`, `updated_at` FROM `mcp_oauth_registrations`;
-- drop "mcp_oauth_registrations" table after copying rows
DROP TABLE `mcp_oauth_registrations`;
-- rename temporary table "new_mcp_oauth_registrations" to "mcp_oauth_registrations"
ALTER TABLE `new_mcp_oauth_registrations` RENAME TO `mcp_oauth_registrations`;
-- create "new_notification_cursors" table
CREATE TABLE `new_notification_cursors` (`scope_kind` text NOT NULL, `profile_id` text NOT NULL, `workspace_id` text NOT NULL DEFAULT '', `consumer_id` text NOT NULL, `stream_name` text NOT NULL, `subject_id` text NOT NULL DEFAULT '', `last_sequence` integer NOT NULL DEFAULT 0, `last_delivery_id` text NOT NULL DEFAULT '', `last_delivered_at` text NULL, `last_error` text NOT NULL DEFAULT '', `updated_at` text NOT NULL, PRIMARY KEY (`scope_kind`, `profile_id`, `workspace_id`, `consumer_id`, `stream_name`, `subject_id`), CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (scope_kind IN ('global', 'workspace')), CHECK (
				(scope_kind = 'global' AND workspace_id = '') OR
(scope_kind = 'workspace' AND workspace_id <> '')
			), CHECK (consumer_id <> ''), CHECK (stream_name <> ''), CHECK (last_sequence >= 0));
-- copy rows from old table "notification_cursors" to new temporary table "new_notification_cursors"
INSERT INTO `new_notification_cursors` (`profile_id`, `scope_kind`, `workspace_id`, `consumer_id`, `stream_name`, `subject_id`, `last_sequence`, `last_delivery_id`, `last_delivered_at`, `last_error`, `updated_at`) SELECT '00000000000000000000000000', `scope_kind`, `workspace_id`, `consumer_id`, `stream_name`, `subject_id`, `last_sequence`, `last_delivery_id`, `last_delivered_at`, `last_error`, `updated_at` FROM `notification_cursors`;
-- drop "notification_cursors" table after copying rows
DROP TABLE `notification_cursors`;
-- rename temporary table "new_notification_cursors" to "notification_cursors"
ALTER TABLE `new_notification_cursors` RENAME TO `notification_cursors`;
-- create index "notification_cursors_stream_sequence_idx" to table: "notification_cursors"
CREATE INDEX `notification_cursors_stream_sequence_idx` ON `notification_cursors` (`scope_kind`, `profile_id`, `workspace_id`, `stream_name`, `last_sequence` DESC) WHERE last_sequence > 0;
-- create "new_worktrees" table
CREATE TABLE `new_worktrees` (`id` text NULL, `profile_id` text NOT NULL, `workspace_id` text NOT NULL, `name` text NOT NULL, `branch` text NOT NULL DEFAULT '', `path` text NOT NULL, `git_dir` text NOT NULL DEFAULT '', `state` text NOT NULL, `pending_phase` text NOT NULL DEFAULT '', `origin` text NOT NULL, `setup_state` text NOT NULL DEFAULT 'none', `setup_error` text NOT NULL DEFAULT '', `base_ref` text NOT NULL DEFAULT '', `created_branch` integer NOT NULL DEFAULT 0, `run_namespace` text NOT NULL DEFAULT '', `created_head` text NOT NULL DEFAULT '', `run_id` text NOT NULL DEFAULT '', `created_at` text NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (
			state IN ('pending', 'ready', 'failed', 'removing', 'missing', 'removed', 'dismissed')
		), CHECK (
			pending_phase IN ('', 'branch', 'checkout', 'copy', 'setup')
		), CHECK (origin IN ('manual', 'per_run', 'adopted')), CHECK (setup_state IN ('none', 'ok', 'failed')), CHECK (created_branch IN (0, 1)));
-- copy rows from old table "worktrees" to new temporary table "new_worktrees"
INSERT INTO `new_worktrees` (`profile_id`, `id`, `workspace_id`, `name`, `branch`, `path`, `git_dir`, `state`, `pending_phase`, `origin`, `setup_state`, `setup_error`, `base_ref`, `created_branch`, `run_namespace`, `created_head`, `run_id`, `created_at`, `updated_at`) SELECT '00000000000000000000000000', `id`, `workspace_id`, `name`, `branch`, `path`, `git_dir`, `state`, `pending_phase`, `origin`, `setup_state`, `setup_error`, `base_ref`, `created_branch`, `run_namespace`, `created_head`, `run_id`, `created_at`, `updated_at` FROM `worktrees`;
-- drop "worktrees" table after copying rows
DROP TABLE `worktrees`;
-- rename temporary table "new_worktrees" to "worktrees"
ALTER TABLE `new_worktrees` RENAME TO `worktrees`;
-- create index "worktrees_workspace_id_id" to table: "worktrees"
CREATE UNIQUE INDEX `worktrees_workspace_id_id` ON `worktrees` (`workspace_id`, `id`);
-- create index "idx_worktrees_workspace_state" to table: "worktrees"
CREATE INDEX `idx_worktrees_workspace_state` ON `worktrees` (`workspace_id`, `state`);
-- create index "idx_worktrees_reserved_name" to table: "worktrees"
CREATE UNIQUE INDEX `idx_worktrees_reserved_name` ON `worktrees` (`workspace_id`, `name`) WHERE state <> 'dismissed';
-- create index "idx_worktrees_live_path" to table: "worktrees"
CREATE UNIQUE INDEX `idx_worktrees_live_path` ON `worktrees` (`path`) WHERE state IN ('pending', 'ready', 'removing');
-- create "new_extension_env_bindings" table
CREATE TABLE `new_extension_env_bindings` (`extension_name` text NOT NULL, `profile_id` text NOT NULL DEFAULT '', `workspace_id` text NOT NULL DEFAULT '', `env_name` text NOT NULL, `secret_ref` text NOT NULL, `mcp_server` text NOT NULL DEFAULT '', `header_name` text NOT NULL DEFAULT '', `kind` text NOT NULL, `created_at` text NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`extension_name`, `profile_id`, `workspace_id`, `env_name`), CHECK (kind = 'extension_env'), CHECK ((mcp_server = '' AND header_name = '') OR (mcp_server <> '' AND header_name <> '')));
-- copy rows from old table "extension_env_bindings" to new temporary table "new_extension_env_bindings"
INSERT INTO `new_extension_env_bindings` (`profile_id`, `extension_name`, `workspace_id`, `env_name`, `secret_ref`, `mcp_server`, `header_name`, `kind`, `created_at`, `updated_at`) SELECT '', `extension_name`, `workspace_id`, `env_name`, `secret_ref`, `mcp_server`, `header_name`, `kind`, `created_at`, `updated_at` FROM `extension_env_bindings`;
-- drop "extension_env_bindings" table after copying rows
DROP TABLE `extension_env_bindings`;
-- rename temporary table "new_extension_env_bindings" to "extension_env_bindings"
ALTER TABLE `new_extension_env_bindings` RENAME TO `extension_env_bindings`;
-- create index "idx_extension_env_bindings_secret_ref" to table: "extension_env_bindings"
CREATE INDEX `idx_extension_env_bindings_secret_ref` ON `extension_env_bindings` (`secret_ref`);
-- create "new_loop_runs" table
CREATE TABLE `new_loop_runs` (`id` text NULL, `profile_id` text NOT NULL, `workspace_id` text NOT NULL, `loop_name` text NOT NULL, `status` text NOT NULL, `completion_state` text NOT NULL DEFAULT 'complete', `forked_from_run_id` text NULL, `forked_from_generation` integer NULL, `generation` integer NOT NULL DEFAULT 0, `reattempt_strategy` text NOT NULL DEFAULT 'failed_only', `last_progress_at` timestamp NOT NULL, `budget_tokens` integer NOT NULL DEFAULT 0, `budget_wall_sec` integer NOT NULL DEFAULT 0, `budget_on_exceeded` text NOT NULL DEFAULT 'halt', `tokens_used` integer NOT NULL DEFAULT 0, `parent_loop_run_id` text NULL, `pause_requested` integer NOT NULL DEFAULT 0, `inputs_json` text NOT NULL, `created_at` text NOT NULL DEFAULT '1970-01-01T00:00:00.000000000Z', `iteration_cap` integer NOT NULL DEFAULT 0, `started_by_kind` text NOT NULL DEFAULT '', `started_by_ref` text NOT NULL DEFAULT '', `started_origin_kind` text NOT NULL DEFAULT '', `started_origin_ref` text NOT NULL DEFAULT '', `started_at` text NOT NULL DEFAULT '1970-01-01T00:00:00.000000000Z', `definition_version` integer NOT NULL DEFAULT 0, `definition_digest` text NOT NULL DEFAULT '', `active_gate_id` text NOT NULL DEFAULT '', `active_human_criteria_json` text NOT NULL DEFAULT '[]', `budget_approval_seq` integer NOT NULL DEFAULT 0, `start_metadata_json` text NOT NULL DEFAULT '{}', `origin_kind` text NOT NULL DEFAULT 'catalog', `origin_session_id` text NULL, `goal_cleared_at` timestamp NULL, `budget_version` integer NOT NULL DEFAULT 0, `goal_context_nudge_ratio` real NOT NULL DEFAULT 0.8, `control_actor_kind` text NULL, `control_actor_id` text NULL, `control_requested_at` timestamp NULL, `origin_creation_profile_ref` text NULL, `origin_policy_spec_digest` text NULL, `origin_creation_digest` text NULL, `network_spec_json` text NOT NULL DEFAULT '{"version":"network-participation/v1","mode":"local","source":"built_in_local"}', `network_mode` text NOT NULL DEFAULT 'local', `network_channel` text NULL, `network_source` text NOT NULL DEFAULT 'built_in_local', `best_generation` integer NULL, `best_score` real NULL, `cancel_requested` integer NOT NULL DEFAULT 0, `cancel_kind` text NOT NULL DEFAULT '', PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (completion_state IN ('complete','partial')), CHECK (budget_version >= 0), CHECK (goal_context_nudge_ratio >= 0.0 AND goal_context_nudge_ratio <= 1.0), CHECK (origin_creation_profile_ref IS NULL OR length(trim(origin_creation_profile_ref)) > 0), CHECK (origin_policy_spec_digest IS NULL OR length(trim(origin_policy_spec_digest)) > 0), CHECK (origin_creation_digest IS NULL OR length(trim(origin_creation_digest)) > 0), CHECK (json_valid(network_spec_json)), CHECK (network_mode IN ('local', 'live')), CHECK (network_source IN (
						'explicit_request', 'task_profile', 'workspace_coordination',
						'loop_definition', 'automation_job', 'built_in_local'
					)), CHECK (cancel_kind IN ('', 'cancel', 'kill')), CHECK (
						(best_generation IS NULL AND best_score IS NULL)
						OR (best_generation IS NOT NULL AND best_score IS NOT NULL
							AND best_generation >= 1 AND best_generation <= generation)
					), CHECK (
						(forked_from_run_id IS NULL AND forked_from_generation IS NULL)
						OR (
							forked_from_run_id IS NOT NULL
							AND forked_from_generation IS NOT NULL
							AND length(trim(forked_from_run_id)) > 0
							AND forked_from_generation >= 1
						)
					));
-- copy rows from old table "loop_runs" to new temporary table "new_loop_runs"
INSERT INTO `new_loop_runs` (`profile_id`, `id`, `workspace_id`, `loop_name`, `status`, `completion_state`, `forked_from_run_id`, `forked_from_generation`, `generation`, `reattempt_strategy`, `last_progress_at`, `budget_tokens`, `budget_wall_sec`, `budget_on_exceeded`, `tokens_used`, `parent_loop_run_id`, `pause_requested`, `inputs_json`, `created_at`, `iteration_cap`, `started_by_kind`, `started_by_ref`, `started_origin_kind`, `started_origin_ref`, `started_at`, `definition_version`, `definition_digest`, `active_gate_id`, `active_human_criteria_json`, `budget_approval_seq`, `start_metadata_json`, `origin_kind`, `origin_session_id`, `goal_cleared_at`, `budget_version`, `goal_context_nudge_ratio`, `control_actor_kind`, `control_actor_id`, `control_requested_at`, `origin_creation_profile_ref`, `origin_policy_spec_digest`, `origin_creation_digest`, `network_spec_json`, `network_mode`, `network_channel`, `network_source`, `best_generation`, `best_score`, `cancel_requested`, `cancel_kind`) SELECT '00000000000000000000000000', `id`, `workspace_id`, `loop_name`, `status`, `completion_state`, `forked_from_run_id`, `forked_from_generation`, `generation`, `reattempt_strategy`, `last_progress_at`, `budget_tokens`, `budget_wall_sec`, `budget_on_exceeded`, `tokens_used`, `parent_loop_run_id`, `pause_requested`, `inputs_json`, `created_at`, `iteration_cap`, `started_by_kind`, `started_by_ref`, `started_origin_kind`, `started_origin_ref`, `started_at`, `definition_version`, `definition_digest`, `active_gate_id`, `active_human_criteria_json`, `budget_approval_seq`, `start_metadata_json`, `origin_kind`, `origin_session_id`, `goal_cleared_at`, `budget_version`, `goal_context_nudge_ratio`, `control_actor_kind`, `control_actor_id`, `control_requested_at`, `origin_creation_profile_ref`, `origin_policy_spec_digest`, `origin_creation_digest`, `network_spec_json`, `network_mode`, `network_channel`, `network_source`, `best_generation`, `best_score`, `cancel_requested`, `cancel_kind` FROM `loop_runs`;
-- drop "loop_runs" table after copying rows
DROP TABLE `loop_runs`;
-- rename temporary table "new_loop_runs" to "loop_runs"
ALTER TABLE `new_loop_runs` RENAME TO `loop_runs`;
-- create index "idx_loop_runs_catalog" to table: "loop_runs"
CREATE INDEX `idx_loop_runs_catalog` ON `loop_runs` (`workspace_id`, `loop_name`, `created_at` DESC, `id` DESC, `status`);
-- create index "idx_loop_runs_queue_order" to table: "loop_runs"
CREATE INDEX `idx_loop_runs_queue_order` ON `loop_runs` (`workspace_id`, `loop_name`, `status`, `created_at`, `id`);
-- create index "uq_loop_runs_active_session_goal" to table: "loop_runs"
CREATE UNIQUE INDEX `uq_loop_runs_active_session_goal` ON `loop_runs` (`origin_session_id`) WHERE origin_kind='session'
			  AND status IN ('queued','running','watching','needs-approval','paused');
-- create "new_cmd_palette_usage" table
CREATE TABLE `new_cmd_palette_usage` (`workspace_id` text NOT NULL, `profile_lens_id` text NOT NULL, `command_id` text NOT NULL, `use_count` integer NOT NULL DEFAULT 0, `frecency_weight` real NOT NULL DEFAULT 0, `last_used_at` integer NOT NULL, `updated_at` integer NOT NULL, PRIMARY KEY (`workspace_id`, `profile_lens_id`, `command_id`), CHECK (trim(command_id) <> ''), CHECK (use_count >= 0), CHECK (frecency_weight >= 0), CHECK (last_used_at >= 0), CHECK (updated_at >= 0));
-- copy rows from old table "cmd_palette_usage" to new temporary table "new_cmd_palette_usage"
INSERT INTO `new_cmd_palette_usage` (`profile_lens_id`, `workspace_id`, `command_id`, `use_count`, `frecency_weight`, `last_used_at`, `updated_at`) SELECT '00000000000000000000000000', `workspace_id`, `command_id`, `use_count`, `frecency_weight`, `last_used_at`, `updated_at` FROM `cmd_palette_usage`;
-- drop "cmd_palette_usage" table after copying rows
DROP TABLE `cmd_palette_usage`;
-- rename temporary table "new_cmd_palette_usage" to "cmd_palette_usage"
ALTER TABLE `new_cmd_palette_usage` RENAME TO `cmd_palette_usage`;
-- create index "idx_cmd_palette_usage_recents" to table: "cmd_palette_usage"
CREATE INDEX `idx_cmd_palette_usage_recents` ON `cmd_palette_usage` (`workspace_id`, `profile_lens_id`, `last_used_at` DESC, `command_id`);
-- create "new_cmd_palette_query_hits" table
CREATE TABLE `new_cmd_palette_query_hits` (`workspace_id` text NOT NULL, `profile_lens_id` text NOT NULL, `query` text NOT NULL, `command_id` text NOT NULL, `weight` real NOT NULL DEFAULT 0, `last_used_at` integer NOT NULL, PRIMARY KEY (`workspace_id`, `profile_lens_id`, `query`, `command_id`), CHECK (trim(query) <> ''), CHECK (trim(command_id) <> ''), CHECK (weight >= 0), CHECK (last_used_at >= 0));
-- copy rows from old table "cmd_palette_query_hits" to new temporary table "new_cmd_palette_query_hits"
INSERT INTO `new_cmd_palette_query_hits` (`profile_lens_id`, `workspace_id`, `query`, `command_id`, `weight`, `last_used_at`) SELECT '00000000000000000000000000', `workspace_id`, `query`, `command_id`, `weight`, `last_used_at` FROM `cmd_palette_query_hits`;
-- drop "cmd_palette_query_hits" table after copying rows
DROP TABLE `cmd_palette_query_hits`;
-- rename temporary table "new_cmd_palette_query_hits" to "cmd_palette_query_hits"
ALTER TABLE `new_cmd_palette_query_hits` RENAME TO `cmd_palette_query_hits`;
-- create index "idx_cmd_palette_query_hits_lookup" to table: "cmd_palette_query_hits"
CREATE INDEX `idx_cmd_palette_query_hits_lookup` ON `cmd_palette_query_hits` (`workspace_id`, `profile_lens_id`, `query`, `last_used_at` DESC, `command_id`);
-- create "new_cmd_palette_pins" table
CREATE TABLE `new_cmd_palette_pins` (`workspace_id` text NOT NULL, `profile_lens_id` text NOT NULL, `command_id` text NOT NULL, `pinned_at` integer NOT NULL, PRIMARY KEY (`workspace_id`, `profile_lens_id`, `command_id`), CHECK (trim(command_id) <> ''), CHECK (pinned_at >= 0));
-- copy rows from old table "cmd_palette_pins" to new temporary table "new_cmd_palette_pins"
INSERT INTO `new_cmd_palette_pins` (`profile_lens_id`, `workspace_id`, `command_id`, `pinned_at`) SELECT '00000000000000000000000000', `workspace_id`, `command_id`, `pinned_at` FROM `cmd_palette_pins`;
-- drop "cmd_palette_pins" table after copying rows
DROP TABLE `cmd_palette_pins`;
-- rename temporary table "new_cmd_palette_pins" to "cmd_palette_pins"
ALTER TABLE `new_cmd_palette_pins` RENAME TO `cmd_palette_pins`;
-- create index "idx_cmd_palette_pins_order" to table: "cmd_palette_pins"
CREATE INDEX `idx_cmd_palette_pins_order` ON `cmd_palette_pins` (`workspace_id`, `profile_lens_id`, `pinned_at`, `command_id`);
-- create "new_network_channels" table
CREATE TABLE `new_network_channels` (`profile_id` text NOT NULL, `workspace_id` text NOT NULL, `channel` text NOT NULL, `purpose` text NOT NULL, `created_by` text NOT NULL DEFAULT '', `created_at` text NOT NULL, `updated_at` text NOT NULL, `fanout_policy` text NOT NULL DEFAULT 'capability_match', `coordinator_peer_id` text NOT NULL DEFAULT '', PRIMARY KEY (`workspace_id`, `channel`), CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (
					fanout_policy IN ('capability_match', 'coordinator', 'all_members')
				));
-- copy rows from old table "network_channels" to new temporary table "new_network_channels"
INSERT INTO `new_network_channels` (`profile_id`, `workspace_id`, `channel`, `purpose`, `created_by`, `created_at`, `updated_at`, `fanout_policy`, `coordinator_peer_id`) SELECT '00000000000000000000000000', `workspace_id`, `channel`, `purpose`, `created_by`, `created_at`, `updated_at`, `fanout_policy`, `coordinator_peer_id` FROM `network_channels`;
-- drop "network_channels" table after copying rows
DROP TABLE `network_channels`;
-- rename temporary table "new_network_channels" to "network_channels"
ALTER TABLE `new_network_channels` RENAME TO `network_channels`;
-- create index "idx_network_channels_updated_at" to table: "network_channels"
CREATE INDEX `idx_network_channels_updated_at` ON `network_channels` (`updated_at`);
-- create index "idx_network_channels_workspace" to table: "network_channels"
CREATE INDEX `idx_network_channels_workspace` ON `network_channels` (`workspace_id`);
-- create index "idx_network_channels_workspace_updated_at" to table: "network_channels"
CREATE INDEX `idx_network_channels_workspace_updated_at` ON `network_channels` (`workspace_id`, `updated_at` DESC, `channel`);
-- create "new_tool_approval_grants" table
CREATE TABLE `new_tool_approval_grants` (`id` text NOT NULL, `profile_id` text NOT NULL, `workspace_id` text NOT NULL, `agent_name` text NOT NULL DEFAULT '', `tool_id` text NOT NULL, `input_digest` text NOT NULL DEFAULT '', `decision` text NOT NULL, `created_at` text NOT NULL, `last_used_at` text NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (trim(id) <> ''), CHECK (trim(tool_id) <> ''), CHECK (input_digest = '' OR input_digest LIKE 'sha256:%'), CHECK (decision IN ('allow', 'reject')));
-- copy rows from old table "tool_approval_grants" to new temporary table "new_tool_approval_grants"
INSERT INTO `new_tool_approval_grants` (`profile_id`, `id`, `workspace_id`, `agent_name`, `tool_id`, `input_digest`, `decision`, `created_at`, `last_used_at`) SELECT '00000000000000000000000000', `id`, `workspace_id`, `agent_name`, `tool_id`, `input_digest`, `decision`, `created_at`, `last_used_at` FROM `tool_approval_grants`;
-- drop "tool_approval_grants" table after copying rows
DROP TABLE `tool_approval_grants`;
-- rename temporary table "new_tool_approval_grants" to "tool_approval_grants"
ALTER TABLE `new_tool_approval_grants` RENAME TO `tool_approval_grants`;
-- create index "tool_approval_grants_workspace_id_agent_name_tool_id_input_digest" to table: "tool_approval_grants"
CREATE UNIQUE INDEX `tool_approval_grants_workspace_id_agent_name_tool_id_input_digest` ON `tool_approval_grants` (`workspace_id`, `agent_name`, `tool_id`, `input_digest`);
-- create index "idx_tool_approval_grants_lookup" to table: "tool_approval_grants"
CREATE INDEX `idx_tool_approval_grants_lookup` ON `tool_approval_grants` (`workspace_id`, `tool_id`, `agent_name`, `input_digest`);
-- create index "idx_tool_approval_grants_list" to table: "tool_approval_grants"
CREATE INDEX `idx_tool_approval_grants_list` ON `tool_approval_grants` (`workspace_id`, `created_at` DESC, `id`);
-- create "new_automation_suggestions" table
CREATE TABLE `new_automation_suggestions` (`id` text NULL, `profile_id` text NOT NULL, `workspace_id` text NULL, `source` text NOT NULL, `dedup_key` text NOT NULL, `status` text NOT NULL, `payload` text NOT NULL, `created_at` text NOT NULL, `resolved_at` text NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (source IN ('catalog', 'usage', 'integration')), CHECK (status IN ('pending', 'accepted', 'dismissed')), CHECK (json_valid(payload) AND json_type(payload) = 'object'), CHECK (
			(status = 'pending' AND resolved_at IS NULL) OR
			(status IN ('accepted', 'dismissed') AND resolved_at IS NOT NULL)
		));
-- copy rows from old table "automation_suggestions" to new temporary table "new_automation_suggestions"
INSERT INTO `new_automation_suggestions` (`profile_id`, `id`, `workspace_id`, `source`, `dedup_key`, `status`, `payload`, `created_at`, `resolved_at`) SELECT '00000000000000000000000000', `id`, `workspace_id`, `source`, `dedup_key`, `status`, `payload`, `created_at`, `resolved_at` FROM `automation_suggestions`;
-- drop "automation_suggestions" table after copying rows
DROP TABLE `automation_suggestions`;
-- rename temporary table "new_automation_suggestions" to "automation_suggestions"
ALTER TABLE `new_automation_suggestions` RENAME TO `automation_suggestions`;
-- create index "idx_automation_suggestions_workspace_status" to table: "automation_suggestions"
CREATE INDEX `idx_automation_suggestions_workspace_status` ON `automation_suggestions` (`workspace_id`, `status`, `created_at`, `id`);
-- create index "automation_suggestions_workspace_id_dedup_key" to table: "automation_suggestions"
CREATE UNIQUE INDEX `automation_suggestions_workspace_id_dedup_key` ON `automation_suggestions` (`workspace_id`, `dedup_key`);
-- create "new_dead_entities" table
CREATE TABLE `new_dead_entities` (`profile_id` text NOT NULL, `workspace_id` text NOT NULL, `kind` text NOT NULL, `entity_id` text NOT NULL, `reason` text NOT NULL, `marked_at` text NOT NULL, PRIMARY KEY (`workspace_id`, `kind`, `entity_id`), CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (kind IN ('extension', 'bridge', 'mcp_sidecar', 'loop_target')), CHECK (trim(entity_id) <> ''), CHECK (trim(reason) <> ''));
-- copy rows from old table "dead_entities" to new temporary table "new_dead_entities"
INSERT INTO `new_dead_entities` (`profile_id`, `workspace_id`, `kind`, `entity_id`, `reason`, `marked_at`) SELECT '00000000000000000000000000', `workspace_id`, `kind`, `entity_id`, `reason`, `marked_at` FROM `dead_entities`;
-- drop "dead_entities" table after copying rows
DROP TABLE `dead_entities`;
-- rename temporary table "new_dead_entities" to "dead_entities"
ALTER TABLE `new_dead_entities` RENAME TO `dead_entities`;
-- create "new_sessions" table
CREATE TABLE `new_sessions` (`id` text NULL, `profile_id` text NOT NULL, `name` text NULL, `agent_name` text NOT NULL, `provider` text NOT NULL DEFAULT '', `model` text NOT NULL DEFAULT '', `reasoning_effort` text NOT NULL DEFAULT '', `speed` text NOT NULL DEFAULT '', `speed_resolution_json` text NOT NULL DEFAULT '', `runtime_status` text NOT NULL DEFAULT 'unbound', `runtime_transition` text NOT NULL DEFAULT '', `runtime_failure` text NOT NULL DEFAULT '', `selected_provider` text NOT NULL DEFAULT '', `selected_model` text NOT NULL DEFAULT '', `selected_reasoning_effort` text NOT NULL DEFAULT '', `selected_speed` text NOT NULL DEFAULT '', `runtime_selection_revision` integer NOT NULL DEFAULT 0, `workspace_id` text NOT NULL, `scope` text NOT NULL DEFAULT 'workspace', `worktree_id` text NULL, `session_type` text NOT NULL DEFAULT 'user', `state` text NOT NULL, `archived_at` text NULL, `acp_session_id` text NULL, `stop_reason` text NULL, `stop_detail` text NULL, `subprocess_pid` integer NOT NULL DEFAULT 0, `subprocess_started_at` text NULL, `last_update_at` text NULL, `stall_state` text NOT NULL DEFAULT '', `stall_reason` text NOT NULL DEFAULT '', `activity_json` text NOT NULL DEFAULT '', `attached_to` text NOT NULL DEFAULT '', `attach_expires_at` text NULL, `transcript_epoch` integer NOT NULL DEFAULT 0, `pending_permission_count` integer NOT NULL DEFAULT 0, `pending_clarify_count` integer NOT NULL DEFAULT 0, `attention_revision` integer NOT NULL DEFAULT 0, `last_settled_revision` integer NOT NULL DEFAULT 0, `last_seen_revision` integer NOT NULL DEFAULT 0, `last_seen_at` text NULL, `attention_changed_at` text NULL, `sandbox_id` text NOT NULL DEFAULT '', `sandbox_backend` text NOT NULL DEFAULT 'local', `sandbox_profile` text NOT NULL DEFAULT '', `sandbox_instance_id` text NOT NULL DEFAULT '', `sandbox_state` text NOT NULL DEFAULT '', `sandbox_provider_state_json` text NOT NULL DEFAULT '', `sandbox_last_sync_at` text NULL, `sandbox_last_sync_error` text NOT NULL DEFAULT '', `created_at` text NOT NULL, `updated_at` text NOT NULL, `failure_kind` text NULL, `failure_summary` text NOT NULL DEFAULT '', `crash_bundle_path` text NOT NULL DEFAULT '', `parent_session_id` text NULL, `root_session_id` text NULL, `spawn_depth` integer NOT NULL DEFAULT 0, `spawn_role` text NULL, `ttl_expires_at` text NULL, `auto_stop_on_parent` boolean NOT NULL DEFAULT 0, `notify_creator` boolean NOT NULL DEFAULT 1, `spawn_budget_json` text NOT NULL DEFAULT '{}', `permission_policy_json` text NOT NULL DEFAULT '{}', `soul_snapshot_id` text NULL, `soul_digest` text NOT NULL DEFAULT '', `parent_soul_digest` text NOT NULL DEFAULT '', `input_generation` integer NOT NULL DEFAULT 0, `creation_digest` text NULL, `policy_spec_digest` text NULL, `creation_profile_ref` text NULL, `network_spec_json` text NOT NULL DEFAULT '{"version":"network-participation/v1","mode":"local","source":"built_in_local"}', `network_mode` text NOT NULL DEFAULT 'local', `network_channel` text NULL, `network_source` text NOT NULL DEFAULT 'built_in_local', PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`, `worktree_id`) REFERENCES `worktrees` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `1` FOREIGN KEY (`soul_snapshot_id`) REFERENCES `agent_soul_snapshots` (`id`) ON UPDATE NO ACTION ON DELETE SET NULL, CONSTRAINT `2` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (scope IN ('global', 'workspace')), CHECK (pending_permission_count >= 0), CHECK (pending_clarify_count >= 0), CHECK (attention_revision >= 0), CHECK (last_settled_revision >= 0), CHECK (last_seen_revision >= 0), CHECK (creation_digest IS NULL OR length(trim(creation_digest)) > 0), CHECK (policy_spec_digest IS NULL OR length(trim(policy_spec_digest)) > 0), CHECK (creation_profile_ref IS NULL OR length(trim(creation_profile_ref)) > 0), CHECK (json_valid(network_spec_json)), CHECK (network_mode IN ('local', 'live')), CHECK (network_source IN (
					'explicit_request', 'task_profile', 'workspace_coordination',
					'loop_definition', 'automation_job', 'built_in_local'
				)), CHECK ((scope = 'workspace') = (workspace_id <> '')));
-- copy rows from old table "sessions" to new temporary table "new_sessions"
INSERT INTO `new_sessions` (`profile_id`, `id`, `name`, `agent_name`, `provider`, `model`, `reasoning_effort`, `speed`, `speed_resolution_json`, `runtime_status`, `runtime_transition`, `runtime_failure`, `selected_provider`, `selected_model`, `selected_reasoning_effort`, `selected_speed`, `runtime_selection_revision`, `workspace_id`, `scope`, `worktree_id`, `session_type`, `state`, `archived_at`, `acp_session_id`, `stop_reason`, `stop_detail`, `subprocess_pid`, `subprocess_started_at`, `last_update_at`, `stall_state`, `stall_reason`, `activity_json`, `attached_to`, `attach_expires_at`, `transcript_epoch`, `pending_permission_count`, `pending_clarify_count`, `attention_revision`, `last_settled_revision`, `last_seen_revision`, `last_seen_at`, `attention_changed_at`, `sandbox_id`, `sandbox_backend`, `sandbox_profile`, `sandbox_instance_id`, `sandbox_state`, `sandbox_provider_state_json`, `sandbox_last_sync_at`, `sandbox_last_sync_error`, `created_at`, `updated_at`, `failure_kind`, `failure_summary`, `crash_bundle_path`, `parent_session_id`, `root_session_id`, `spawn_depth`, `spawn_role`, `ttl_expires_at`, `auto_stop_on_parent`, `notify_creator`, `spawn_budget_json`, `permission_policy_json`, `soul_snapshot_id`, `soul_digest`, `parent_soul_digest`, `input_generation`, `creation_digest`, `policy_spec_digest`, `creation_profile_ref`, `network_spec_json`, `network_mode`, `network_channel`, `network_source`) SELECT '00000000000000000000000000', `id`, `name`, `agent_name`, `provider`, `model`, `reasoning_effort`, `speed`, `speed_resolution_json`, `runtime_status`, `runtime_transition`, `runtime_failure`, `selected_provider`, `selected_model`, `selected_reasoning_effort`, `selected_speed`, `runtime_selection_revision`, `workspace_id`, `scope`, `worktree_id`, `session_type`, `state`, `archived_at`, `acp_session_id`, `stop_reason`, `stop_detail`, `subprocess_pid`, `subprocess_started_at`, `last_update_at`, `stall_state`, `stall_reason`, `activity_json`, `attached_to`, `attach_expires_at`, `transcript_epoch`, `pending_permission_count`, `pending_clarify_count`, `attention_revision`, `last_settled_revision`, `last_seen_revision`, `last_seen_at`, `attention_changed_at`, `sandbox_id`, `sandbox_backend`, `sandbox_profile`, `sandbox_instance_id`, `sandbox_state`, `sandbox_provider_state_json`, `sandbox_last_sync_at`, `sandbox_last_sync_error`, `created_at`, `updated_at`, `failure_kind`, `failure_summary`, `crash_bundle_path`, `parent_session_id`, `root_session_id`, `spawn_depth`, `spawn_role`, `ttl_expires_at`, `auto_stop_on_parent`, `notify_creator`, `spawn_budget_json`, `permission_policy_json`, `soul_snapshot_id`, `soul_digest`, `parent_soul_digest`, `input_generation`, `creation_digest`, `policy_spec_digest`, `creation_profile_ref`, `network_spec_json`, `network_mode`, `network_channel`, `network_source` FROM `sessions`;
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
-- create "new_tool_approval_pending" table
CREATE TABLE `new_tool_approval_pending` (`approval_id` text NOT NULL, `profile_id` text NOT NULL, `workspace_id` text NULL, `invocation_id` text NOT NULL, `target_kind` text NOT NULL, `tool_id` text NULL, `target_json` text NOT NULL DEFAULT '{}', `command_id` text NULL, `args_json` text NOT NULL, `approval_status` text NOT NULL, `execution_status` text NULL, `result_json` text NULL, `error_json` text NULL, `requested_at` integer NOT NULL, `expires_at` integer NOT NULL, `resolved_at` integer NULL, `executed_at` integer NULL, `resume_fence` integer NOT NULL DEFAULT 0, PRIMARY KEY (`approval_id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (approval_id LIKE 'apr_%'), CHECK (trim(invocation_id) <> ''), CHECK (target_kind IN ('tool', 'client_op', 'navigate', 'view')), CHECK (json_valid(target_json)), CHECK (json_valid(args_json)), CHECK (
		approval_status IN ('pending', 'approved', 'denied', 'timeout', 'canceled')
	), CHECK (
		execution_status IS NULL OR execution_status IN ('dispatching', 'completed', 'failed', 'uncertain')
	), CHECK (result_json IS NULL OR json_valid(result_json)), CHECK (error_json IS NULL OR json_valid(error_json)), CHECK (expires_at > requested_at), CHECK (resume_fence IN (0, 1)), CHECK ((target_kind = 'tool' AND trim(coalesce(tool_id, '')) <> '') OR target_kind <> 'tool'), CHECK ((approval_status = 'pending' AND resolved_at IS NULL) OR (approval_status <> 'pending' AND resolved_at IS NOT NULL)), CHECK ((execution_status IS NULL AND executed_at IS NULL) OR execution_status IS NOT NULL));
-- copy rows from old table "tool_approval_pending" to new temporary table "new_tool_approval_pending"
INSERT INTO `new_tool_approval_pending` (`profile_id`, `approval_id`, `workspace_id`, `invocation_id`, `target_kind`, `tool_id`, `target_json`, `command_id`, `args_json`, `approval_status`, `execution_status`, `result_json`, `error_json`, `requested_at`, `expires_at`, `resolved_at`, `executed_at`, `resume_fence`) SELECT '00000000000000000000000000', `approval_id`, `workspace_id`, `invocation_id`, `target_kind`, `tool_id`, `target_json`, `command_id`, `args_json`, `approval_status`, `execution_status`, `result_json`, `error_json`, `requested_at`, `expires_at`, `resolved_at`, `executed_at`, `resume_fence` FROM `tool_approval_pending`;
-- drop "tool_approval_pending" table after copying rows
DROP TABLE `tool_approval_pending`;
-- rename temporary table "new_tool_approval_pending" to "tool_approval_pending"
ALTER TABLE `new_tool_approval_pending` RENAME TO `tool_approval_pending`;
-- create index "tool_approval_pending_invocation_id" to table: "tool_approval_pending"
CREATE UNIQUE INDEX `tool_approval_pending_invocation_id` ON `tool_approval_pending` (`invocation_id`);
-- create index "idx_tool_approval_pending_workspace_status" to table: "tool_approval_pending"
CREATE INDEX `idx_tool_approval_pending_workspace_status` ON `tool_approval_pending` (`workspace_id`, `approval_status`, `expires_at`, `approval_id`);
-- create index "idx_tool_approval_pending_recovery" to table: "tool_approval_pending"
CREATE INDEX `idx_tool_approval_pending_recovery` ON `tool_approval_pending` (`approval_status`, `execution_status`, `resume_fence`, `expires_at`);
-- create "profile_selections" table
CREATE TABLE `profile_selections` (`lens` text NOT NULL, `workspace_id` text NOT NULL DEFAULT '', `profile_id` text NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`lens`, `workspace_id`), CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (lens IN ('workspace', 'global')), CHECK ((lens = 'global' AND workspace_id = '') OR (lens = 'workspace' AND workspace_id <> '')));
-- create "profile_lifecycle_ops" table
CREATE TABLE `profile_lifecycle_ops` (`id` text NULL, `kind` text NOT NULL, `profile_id` text NOT NULL, `old_name` text NULL, `new_name` text NULL, `plan_revision` text NOT NULL, `status` text NOT NULL, `created_at` text NOT NULL, `updated_at` text NOT NULL, `completed_at` text NULL, `error_code` text NULL, `error_message` text NULL, PRIMARY KEY (`id`), CHECK (id LIKE 'op_%'), CHECK (kind IN ('create', 'rename', 'archive', 'unarchive', 'delete')), CHECK (trim(plan_revision) <> ''), CHECK (status IN ('applied', 'finalizing', 'done', 'failed')));
-- create index "idx_profile_lifecycle_ops_profile_status" to table: "profile_lifecycle_ops"
CREATE INDEX `idx_profile_lifecycle_ops_profile_status` ON `profile_lifecycle_ops` (`profile_id`, `status`, `updated_at` DESC);
-- create "profile_lifecycle_op_steps" table
CREATE TABLE `profile_lifecycle_op_steps` (`op_id` text NOT NULL, `seq` integer NOT NULL, `action` text NOT NULL, `path_old` text NULL, `path_new` text NULL, `status` text NOT NULL, `updated_at` text NOT NULL, `error_message` text NULL, PRIMARY KEY (`op_id`, `seq`), CONSTRAINT `0` FOREIGN KEY (`op_id`) REFERENCES `profile_lifecycle_ops` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (seq >= 0), CHECK (trim(action) <> ''), CHECK (status IN ('pending', 'done', 'failed')));
-- create "profile_lifecycle_op_seed" table
CREATE TABLE `profile_lifecycle_op_seed` (`op_id` text NULL, `color` text NOT NULL, `icon` text NULL, `emoji` text NULL, `default_agent` text NULL, `default_provider` text NULL, `default_sandbox` text NULL, `declaration_digest` text NOT NULL, PRIMARY KEY (`op_id`), CONSTRAINT `0` FOREIGN KEY (`op_id`) REFERENCES `profile_lifecycle_ops` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (trim(color) <> ''), CHECK (trim(declaration_digest) <> ''), CHECK ((icon IS NULL) <> (emoji IS NULL)));
-- create "profile_lifecycle_op_credential_asks" table
CREATE TABLE `profile_lifecycle_op_credential_asks` (`op_id` text NOT NULL, `provider` text NOT NULL, `slot` text NOT NULL, PRIMARY KEY (`op_id`, `provider`, `slot`), CONSTRAINT `0` FOREIGN KEY (`op_id`) REFERENCES `profile_lifecycle_ops` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (trim(provider) <> ''), CHECK (trim(slot) <> ''));
-- create "profile_credential_requirements" table
CREATE TABLE `profile_credential_requirements` (`profile_id` text NOT NULL, `provider` text NOT NULL, `slot` text NOT NULL, `source_extension` text NOT NULL, `declaration_digest` text NOT NULL, `created_at` text NOT NULL, PRIMARY KEY (`profile_id`, `provider`, `slot`), CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (trim(provider) <> ''), CHECK (trim(slot) <> ''), CHECK (trim(source_extension) <> ''), CHECK (trim(declaration_digest) <> ''));
-- create "notification_delivery_permits" table
CREATE TABLE `notification_delivery_permits` (`scope_kind` text NOT NULL, `profile_id` text NOT NULL, `workspace_id` text NOT NULL DEFAULT '', `consumer_id` text NOT NULL, `stream_name` text NOT NULL, `subject_id` text NOT NULL DEFAULT '', `delivery_id` text NOT NULL, `acquired_at` text NOT NULL, PRIMARY KEY (`scope_kind`, `profile_id`, `workspace_id`, `consumer_id`, `stream_name`, `subject_id`, `delivery_id`), CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (scope_kind IN ('global', 'workspace')), CHECK (consumer_id <> ''), CHECK (stream_name <> ''), CHECK (delivery_id <> ''), CHECK ((scope_kind = 'global' AND workspace_id = '') OR (scope_kind = 'workspace' AND workspace_id <> '')));
-- create "extension_profile_enablement" table
CREATE TABLE `extension_profile_enablement` (`extension_name` text NOT NULL, `profile_id` text NOT NULL, `enabled` integer NOT NULL, PRIMARY KEY (`extension_name`, `profile_id`), CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`extension_name`) REFERENCES `extensions` (`name`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (enabled IN (0, 1)));
INSERT INTO `extension_profile_enablement` (`extension_name`, `profile_id`, `enabled`)
SELECT `name`, '00000000000000000000000000', 0
FROM `profile_migration_disabled_extensions`;
DROP TABLE `profile_migration_disabled_extensions`;
-- create "extension_profile_markers" table
CREATE TABLE `extension_profile_markers` (`extension_name` text NOT NULL, `profile_name` text NOT NULL, `created_profile_id` text NOT NULL, `created_at` text NOT NULL, PRIMARY KEY (`extension_name`, `profile_name`), CONSTRAINT `0` FOREIGN KEY (`extension_name`) REFERENCES `extensions` (`name`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (trim(profile_name) <> ''), CHECK (trim(created_profile_id) <> ''));
-- create "notification_preset_enablement" table
CREATE TABLE `notification_preset_enablement` (`preset_name` text NOT NULL, `profile_id` text NOT NULL, `enabled` integer NOT NULL, PRIMARY KEY (`preset_name`, `profile_id`), CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`preset_name`) REFERENCES `notification_presets` (`name`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (enabled IN (0, 1)));
INSERT INTO `notification_preset_enablement` (`preset_name`, `profile_id`, `enabled`)
SELECT `name`, '00000000000000000000000000', 0
FROM `profile_migration_disabled_notification_presets`;
DROP TABLE `profile_migration_disabled_notification_presets`;
-- create "attention_workspace_mutes" table
CREATE TABLE `attention_workspace_mutes` (`profile_id` text NOT NULL, `workspace_id` text NOT NULL, PRIMARY KEY (`profile_id`, `workspace_id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE);
UPDATE `vault_secrets`
SET `ref` = replace(`ref`, 'vault:mcp/global/', 'vault:mcp/user/')
WHERE `ref` LIKE 'vault:mcp/global/%';
UPDATE `vault_secrets`
SET `ref` = replace(`ref`, 'vault:extensions/global/', 'vault:extensions/user/')
WHERE `ref` LIKE 'vault:extensions/global/%';
UPDATE `mcp_auth_tokens`
SET `access_token_ref` = replace(`access_token_ref`, 'vault:mcp/global/', 'vault:mcp/user/'),
    `refresh_token_ref` = replace(`refresh_token_ref`, 'vault:mcp/global/', 'vault:mcp/user/');
UPDATE `mcp_oauth_registrations`
SET `client_secret_ref` = replace(`client_secret_ref`, 'vault:mcp/global/', 'vault:mcp/user/'),
    `registration_access_token_ref` = replace(`registration_access_token_ref`, 'vault:mcp/global/', 'vault:mcp/user/');
UPDATE `extension_env_bindings`
SET `secret_ref` = replace(`secret_ref`, 'vault:extensions/global/', 'vault:extensions/user/');
UPDATE `bridge_secret_bindings`
SET `secret_ref` = replace(`secret_ref`, 'vault:extensions/global/', 'vault:extensions/user/');
UPDATE `automation_triggers`
SET `webhook_secret_ref` = replace(`webhook_secret_ref`, 'vault:extensions/global/', 'vault:extensions/user/');
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
-- recreate trigger "automation_jobs_profile_owner_active" after rebuilding table "automation_jobs"
-- +goose StatementBegin
CREATE TRIGGER automation_jobs_profile_owner_active BEFORE INSERT ON automation_jobs BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER cmd_palette_usage_profile_lens_update BEFORE UPDATE OF profile_lens_id ON cmd_palette_usage
WHEN NEW.profile_lens_id <> '@all' AND NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_lens_id)
BEGIN SELECT RAISE(ABORT, 'profile_lens_not_found'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER cmd_palette_query_hits_profile_lens_update BEFORE UPDATE OF profile_lens_id ON cmd_palette_query_hits
WHEN NEW.profile_lens_id <> '@all' AND NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_lens_id)
BEGIN SELECT RAISE(ABORT, 'profile_lens_not_found'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER cmd_palette_pins_profile_lens_update BEFORE UPDATE OF profile_lens_id ON cmd_palette_pins
WHEN NEW.profile_lens_id <> '@all' AND NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_lens_id)
BEGIN SELECT RAISE(ABORT, 'profile_lens_not_found'); END;
-- +goose StatementEnd
-- recreate trigger "automation_jobs_profile_owner_immutable" after rebuilding table "automation_jobs"
-- +goose StatementBegin
CREATE TRIGGER automation_jobs_profile_owner_immutable BEFORE UPDATE OF profile_id ON automation_jobs
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
-- recreate trigger "automation_suggestions_profile_owner_active" after rebuilding table "automation_suggestions"
-- +goose StatementBegin
CREATE TRIGGER automation_suggestions_profile_owner_active BEFORE INSERT ON automation_suggestions BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd
-- recreate trigger "automation_suggestions_profile_owner_immutable" after rebuilding table "automation_suggestions"
-- +goose StatementBegin
CREATE TRIGGER automation_suggestions_profile_owner_immutable BEFORE UPDATE OF profile_id ON automation_suggestions
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
-- recreate trigger "automation_triggers_profile_owner_active" after rebuilding table "automation_triggers"
-- +goose StatementBegin
CREATE TRIGGER automation_triggers_profile_owner_active BEFORE INSERT ON automation_triggers BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd
-- recreate trigger "automation_triggers_profile_owner_immutable" after rebuilding table "automation_triggers"
-- +goose StatementBegin
CREATE TRIGGER automation_triggers_profile_owner_immutable BEFORE UPDATE OF profile_id ON automation_triggers
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
-- recreate trigger "automation_watch_events_after_insert" after rebuilding table "automation_runs"
-- +goose StatementBegin
CREATE TRIGGER automation_watch_events_after_insert
AFTER INSERT ON automation_runs
WHEN NEW.status IN ('completed', 'failed')
BEGIN
	INSERT INTO automation_watch_events (
		run_id, job_id, trigger_id, session_id, status, attempt,
		started_at, ended_at, error, agent_name, workspace_id, retry_json
	) VALUES (
		NEW.id,
		COALESCE(NEW.job_id, ''),
		COALESCE(NEW.trigger_id, ''),
		COALESCE(NEW.session_id, ''),
		NEW.status,
		NEW.attempt,
		NEW.started_at,
		NEW.ended_at,
		COALESCE(NEW.error, ''),
		COALESCE(
			(SELECT agent_name FROM automation_jobs WHERE id = NEW.job_id),
			(SELECT agent_name FROM automation_triggers WHERE id = NEW.trigger_id),
			''
		),
		COALESCE(
			NULLIF((SELECT workspace_id FROM automation_jobs WHERE id = NEW.job_id), ''),
			NULLIF((SELECT workspace_id FROM automation_triggers WHERE id = NEW.trigger_id), ''),
			NULLIF((SELECT workspace_id FROM loop_runs WHERE id = NEW.loop_run_id), ''),
			NULLIF((SELECT loop_workspace_id FROM automation_jobs WHERE id = NEW.job_id), ''),
			NULLIF((SELECT loop_workspace_id FROM automation_triggers WHERE id = NEW.trigger_id), ''),
			''
		),
		COALESCE(
			(SELECT retry FROM automation_jobs WHERE id = NEW.job_id),
			(SELECT retry FROM automation_triggers WHERE id = NEW.trigger_id),
			''
		)
	);
END;
-- +goose StatementEnd
-- recreate trigger "automation_watch_events_after_terminal_update" after rebuilding table "automation_runs"
-- +goose StatementBegin
CREATE TRIGGER automation_watch_events_after_terminal_update
AFTER UPDATE OF status ON automation_runs
WHEN NEW.status IN ('completed', 'failed') AND OLD.status NOT IN ('completed', 'failed')
BEGIN
	INSERT INTO automation_watch_events (
		run_id, job_id, trigger_id, session_id, status, attempt,
		started_at, ended_at, error, agent_name, workspace_id, retry_json
	) VALUES (
		NEW.id,
		COALESCE(NEW.job_id, ''),
		COALESCE(NEW.trigger_id, ''),
		COALESCE(NEW.session_id, ''),
		NEW.status,
		NEW.attempt,
		NEW.started_at,
		NEW.ended_at,
		COALESCE(NEW.error, ''),
		COALESCE(
			(SELECT agent_name FROM automation_jobs WHERE id = NEW.job_id),
			(SELECT agent_name FROM automation_triggers WHERE id = NEW.trigger_id),
			''
		),
		COALESCE(
			NULLIF((SELECT workspace_id FROM automation_jobs WHERE id = NEW.job_id), ''),
			NULLIF((SELECT workspace_id FROM automation_triggers WHERE id = NEW.trigger_id), ''),
			NULLIF((SELECT workspace_id FROM loop_runs WHERE id = NEW.loop_run_id), ''),
			NULLIF((SELECT loop_workspace_id FROM automation_jobs WHERE id = NEW.job_id), ''),
			NULLIF((SELECT loop_workspace_id FROM automation_triggers WHERE id = NEW.trigger_id), ''),
			''
		),
		COALESCE(
			(SELECT retry FROM automation_jobs WHERE id = NEW.job_id),
			(SELECT retry FROM automation_triggers WHERE id = NEW.trigger_id),
			''
		)
	);
END;
-- +goose StatementEnd
-- recreate trigger "bridge_instances_profile_owner_active" after rebuilding table "bridge_instances"
-- +goose StatementBegin
CREATE TRIGGER bridge_instances_profile_owner_active BEFORE INSERT ON bridge_instances BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd
-- recreate trigger "bridge_instances_profile_owner_immutable" after rebuilding table "bridge_instances"
-- +goose StatementBegin
CREATE TRIGGER bridge_instances_profile_owner_immutable BEFORE UPDATE OF profile_id ON bridge_instances
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
-- recreate trigger "cmd_palette_pins_profile_lens_insert" after rebuilding table "cmd_palette_pins"
-- +goose StatementBegin
CREATE TRIGGER cmd_palette_pins_profile_lens_insert BEFORE INSERT ON cmd_palette_pins
WHEN NEW.profile_lens_id <> '@all' AND NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_lens_id)
BEGIN SELECT RAISE(ABORT, 'profile_lens_not_found'); END;
-- +goose StatementEnd
-- recreate trigger "cmd_palette_query_hits_profile_lens_insert" after rebuilding table "cmd_palette_query_hits"
-- +goose StatementBegin
CREATE TRIGGER cmd_palette_query_hits_profile_lens_insert BEFORE INSERT ON cmd_palette_query_hits
WHEN NEW.profile_lens_id <> '@all' AND NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_lens_id)
BEGIN SELECT RAISE(ABORT, 'profile_lens_not_found'); END;
-- +goose StatementEnd
-- recreate trigger "cmd_palette_usage_profile_lens_insert" after rebuilding table "cmd_palette_usage"
-- +goose StatementBegin
CREATE TRIGGER cmd_palette_usage_profile_lens_insert BEFORE INSERT ON cmd_palette_usage
WHEN NEW.profile_lens_id <> '@all' AND NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_lens_id)
BEGIN SELECT RAISE(ABORT, 'profile_lens_not_found'); END;
-- +goose StatementEnd
-- recreate trigger "cmd_palette_workspace_delete" after rebuilding table "workspaces"
-- +goose StatementBegin
CREATE TRIGGER cmd_palette_workspace_delete
AFTER DELETE ON workspaces
BEGIN
	DELETE FROM cmd_palette_usage WHERE workspace_id = OLD.id;
	DELETE FROM cmd_palette_query_hits WHERE workspace_id = OLD.id;
	DELETE FROM cmd_palette_pins WHERE workspace_id = OLD.id;
END;
-- +goose StatementEnd
-- recreate trigger "dead_entities_profile_owner_active" after rebuilding table "dead_entities"
-- +goose StatementBegin
CREATE TRIGGER dead_entities_profile_owner_active BEFORE INSERT ON dead_entities BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd
-- recreate trigger "dead_entities_profile_owner_immutable" after rebuilding table "dead_entities"
-- +goose StatementBegin
CREATE TRIGGER dead_entities_profile_owner_immutable BEFORE UPDATE OF profile_id ON dead_entities
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
-- recreate trigger "dead_entities_workspace_insert_guard" after rebuilding table "dead_entities"
-- +goose StatementBegin
CREATE TRIGGER dead_entities_workspace_insert_guard
BEFORE INSERT ON dead_entities
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "dead_entities_workspace_update_guard" after rebuilding table "dead_entities"
-- +goose StatementBegin
CREATE TRIGGER dead_entities_workspace_update_guard
BEFORE UPDATE OF workspace_id ON dead_entities
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "event_summaries_profile_owner_active" after rebuilding table "event_summaries"
-- +goose StatementBegin
CREATE TRIGGER event_summaries_profile_owner_active BEFORE INSERT ON event_summaries BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd
-- recreate trigger "event_summaries_profile_owner_immutable" after rebuilding table "event_summaries"
-- +goose StatementBegin
CREATE TRIGGER event_summaries_profile_owner_immutable BEFORE UPDATE OF profile_id ON event_summaries
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
-- recreate trigger "extension_env_bindings_profile_delete" after rebuilding table "profiles"
-- +goose StatementBegin
CREATE TRIGGER extension_env_bindings_profile_delete
AFTER DELETE ON profiles
BEGIN
	DELETE FROM extension_env_bindings WHERE profile_id = OLD.id;
END;
-- +goose StatementEnd
-- recreate trigger "extension_env_bindings_profile_insert" after rebuilding table "extension_env_bindings"
-- +goose StatementBegin
CREATE TRIGGER extension_env_bindings_profile_insert
BEFORE INSERT ON extension_env_bindings
WHEN NEW.profile_id <> '' AND NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id)
BEGIN
	SELECT RAISE(ABORT, 'profile_not_found');
END;
-- +goose StatementEnd
-- recreate trigger "extension_env_bindings_workspace_delete" after rebuilding table "workspaces"
-- +goose StatementBegin
CREATE TRIGGER extension_env_bindings_workspace_delete
AFTER DELETE ON workspaces
BEGIN
	DELETE FROM extension_env_bindings WHERE workspace_id = OLD.id;
END;
-- +goose StatementEnd
-- recreate trigger "gateway_ingress_bridge_resource_identity_update" after rebuilding table "resource_records"
-- +goose StatementBegin
CREATE TRIGGER gateway_ingress_bridge_resource_identity_update
AFTER UPDATE OF scope_kind, scope_id, spec_json ON resource_records
WHEN OLD.kind = 'bridge.instance' AND (
	OLD.scope_kind IS NOT NEW.scope_kind OR
	COALESCE(OLD.scope_id, '') <> COALESCE(NEW.scope_id, '') OR
	json_extract(OLD.spec_json, '$.provider_config') IS NOT
		json_extract(NEW.spec_json, '$.provider_config')
)
BEGIN
	DELETE FROM gateway_ingress_bindings
	WHERE subject_kind = 'bridge_instance' AND subject_id = OLD.id;
END;
-- +goose StatementEnd
-- recreate trigger "gateway_ingress_resource_delete" after rebuilding table "resource_records"
-- +goose StatementBegin
CREATE TRIGGER gateway_ingress_resource_delete
AFTER DELETE ON resource_records
WHEN OLD.kind IN ('automation.trigger', 'bridge.instance')
BEGIN
	DELETE FROM gateway_ingress_bindings
	WHERE subject_id = OLD.id
		AND subject_kind = CASE OLD.kind
			WHEN 'automation.trigger' THEN 'webhook_trigger'
			WHEN 'bridge.instance' THEN 'bridge_instance'
		END;
END;
-- +goose StatementEnd
-- recreate trigger "gateway_ingress_trigger_resource_identity_update" after rebuilding table "resource_records"
-- +goose StatementBegin
CREATE TRIGGER gateway_ingress_trigger_resource_identity_update
AFTER UPDATE OF scope_kind, scope_id, spec_json ON resource_records
WHEN OLD.kind = 'automation.trigger' AND (
	OLD.scope_kind IS NOT NEW.scope_kind OR
	COALESCE(OLD.scope_id, '') <> COALESCE(NEW.scope_id, '') OR
	json_extract(OLD.spec_json, '$.event') IS NOT json_extract(NEW.spec_json, '$.event') OR
	json_extract(OLD.spec_json, '$.endpoint_slug') IS NOT json_extract(NEW.spec_json, '$.endpoint_slug') OR
	json_extract(OLD.spec_json, '$.webhook_id') IS NOT json_extract(NEW.spec_json, '$.webhook_id')
)
BEGIN
	DELETE FROM gateway_ingress_bindings
	WHERE subject_kind = 'webhook_trigger' AND subject_id = OLD.id;
END;
-- +goose StatementEnd
-- recreate trigger "loop_runs_profile_owner_active" after rebuilding table "loop_runs"
-- +goose StatementBegin
CREATE TRIGGER loop_runs_profile_owner_active BEFORE INSERT ON loop_runs BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd
-- recreate trigger "loop_runs_profile_owner_immutable" after rebuilding table "loop_runs"
-- +goose StatementBegin
CREATE TRIGGER loop_runs_profile_owner_immutable BEFORE UPDATE OF profile_id ON loop_runs
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
-- recreate trigger "network_channels_profile_owner_active" after rebuilding table "network_channels"
-- +goose StatementBegin
CREATE TRIGGER network_channels_profile_owner_active BEFORE INSERT ON network_channels BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd
-- recreate trigger "network_channels_profile_owner_immutable" after rebuilding table "network_channels"
-- +goose StatementBegin
CREATE TRIGGER network_channels_profile_owner_immutable BEFORE UPDATE OF profile_id ON network_channels
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
-- recreate trigger "network_channels_workspace_insert_guard" after rebuilding table "network_channels"
-- +goose StatementBegin
CREATE TRIGGER network_channels_workspace_insert_guard
BEFORE INSERT ON network_channels
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "network_channels_workspace_update_guard" after rebuilding table "network_channels"
-- +goose StatementBegin
CREATE TRIGGER network_channels_workspace_update_guard
BEFORE UPDATE OF workspace_id ON network_channels
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "network_direct_rooms_profile_owner_active" after rebuilding table "network_direct_rooms"
-- +goose StatementBegin
CREATE TRIGGER network_direct_rooms_profile_owner_active BEFORE INSERT ON network_direct_rooms BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd
-- recreate trigger "network_direct_rooms_profile_owner_immutable" after rebuilding table "network_direct_rooms"
-- +goose StatementBegin
CREATE TRIGGER network_direct_rooms_profile_owner_immutable BEFORE UPDATE OF profile_id ON network_direct_rooms
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
-- recreate trigger "network_threads_profile_owner_active" after rebuilding table "network_threads"
-- +goose StatementBegin
CREATE TRIGGER network_threads_profile_owner_active BEFORE INSERT ON network_threads BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd
-- recreate trigger "network_threads_profile_owner_immutable" after rebuilding table "network_threads"
-- +goose StatementBegin
CREATE TRIGGER network_threads_profile_owner_immutable BEFORE UPDATE OF profile_id ON network_threads
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
-- recreate trigger "network_work_profile_owner_active" after rebuilding table "network_work"
-- +goose StatementBegin
CREATE TRIGGER network_work_profile_owner_active BEFORE INSERT ON network_work BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd
-- recreate trigger "network_work_profile_owner_immutable" after rebuilding table "network_work"
-- +goose StatementBegin
CREATE TRIGGER network_work_profile_owner_immutable BEFORE UPDATE OF profile_id ON network_work
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
-- recreate trigger "no_workspace_data_workspace_delete" after rebuilding table "workspaces"
-- +goose StatementBegin
CREATE TRIGGER no_workspace_data_workspace_delete
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
-- recreate trigger "notification_cursors_profile_owner_active" after rebuilding table "notification_cursors"
-- +goose StatementBegin
CREATE TRIGGER notification_cursors_profile_owner_active BEFORE INSERT ON notification_cursors BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd
-- recreate trigger "notification_cursors_profile_owner_immutable" after rebuilding table "notification_cursors"
-- +goose StatementBegin
CREATE TRIGGER notification_cursors_profile_owner_immutable BEFORE UPDATE OF profile_id ON notification_cursors
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
-- recreate trigger "profiles_palette_cleanup" after rebuilding table "profiles"
-- +goose StatementBegin
CREATE TRIGGER profiles_palette_cleanup AFTER DELETE ON profiles BEGIN
	DELETE FROM cmd_palette_usage WHERE profile_lens_id = OLD.id;
	DELETE FROM cmd_palette_query_hits WHERE profile_lens_id = OLD.id;
	DELETE FROM cmd_palette_pins WHERE profile_lens_id = OLD.id;
END;
-- +goose StatementEnd
-- recreate trigger "sessions_profile_owner_active" after rebuilding table "sessions"
-- +goose StatementBegin
CREATE TRIGGER sessions_profile_owner_active BEFORE INSERT ON sessions BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd
-- recreate trigger "sessions_profile_owner_immutable" after rebuilding table "sessions"
-- +goose StatementBegin
CREATE TRIGGER sessions_profile_owner_immutable BEFORE UPDATE OF profile_id ON sessions
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
-- recreate trigger "sessions_workspace_insert_guard" after rebuilding table "sessions"
-- +goose StatementBegin
CREATE TRIGGER sessions_workspace_insert_guard
BEFORE INSERT ON sessions
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "sessions_workspace_update_guard" after rebuilding table "sessions"
-- +goose StatementBegin
CREATE TRIGGER sessions_workspace_update_guard
BEFORE UPDATE OF workspace_id ON sessions
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "tasks_profile_owner_active" after rebuilding table "tasks"
-- +goose StatementBegin
CREATE TRIGGER tasks_profile_owner_active BEFORE INSERT ON tasks BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd
-- recreate trigger "tasks_profile_owner_immutable" after rebuilding table "tasks"
-- +goose StatementBegin
CREATE TRIGGER tasks_profile_owner_immutable BEFORE UPDATE OF profile_id ON tasks
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
-- recreate trigger "token_usage_daily_profile_owner_active" after rebuilding table "token_usage_daily"
-- +goose StatementBegin
CREATE TRIGGER token_usage_daily_profile_owner_active BEFORE INSERT ON token_usage_daily BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd
-- recreate trigger "token_usage_daily_profile_owner_immutable" after rebuilding table "token_usage_daily"
-- +goose StatementBegin
CREATE TRIGGER token_usage_daily_profile_owner_immutable BEFORE UPDATE OF profile_id ON token_usage_daily
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
-- recreate trigger "tool_approval_grants_profile_owner_active" after rebuilding table "tool_approval_grants"
-- +goose StatementBegin
CREATE TRIGGER tool_approval_grants_profile_owner_active BEFORE INSERT ON tool_approval_grants BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd
-- recreate trigger "tool_approval_grants_profile_owner_immutable" after rebuilding table "tool_approval_grants"
-- +goose StatementBegin
CREATE TRIGGER tool_approval_grants_profile_owner_immutable BEFORE UPDATE OF profile_id ON tool_approval_grants
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
-- recreate trigger "tool_approval_grants_workspace_insert_guard" after rebuilding table "tool_approval_grants"
-- +goose StatementBegin
CREATE TRIGGER tool_approval_grants_workspace_insert_guard
BEFORE INSERT ON tool_approval_grants
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "tool_approval_grants_workspace_update_guard" after rebuilding table "tool_approval_grants"
-- +goose StatementBegin
CREATE TRIGGER tool_approval_grants_workspace_update_guard
BEFORE UPDATE OF workspace_id ON tool_approval_grants
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "tool_approval_pending_profile_owner_active" after rebuilding table "tool_approval_pending"
-- +goose StatementBegin
CREATE TRIGGER tool_approval_pending_profile_owner_active BEFORE INSERT ON tool_approval_pending BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd
-- recreate trigger "tool_approval_pending_profile_owner_immutable" after rebuilding table "tool_approval_pending"
-- +goose StatementBegin
CREATE TRIGGER tool_approval_pending_profile_owner_immutable BEFORE UPDATE OF profile_id ON tool_approval_pending
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
-- recreate trigger "trg_bridge_instance_active_delivery_delete" after rebuilding table "bridge_instances"
-- +goose StatementBegin
CREATE TRIGGER trg_bridge_instance_active_delivery_delete
			BEFORE DELETE ON bridge_instances
			WHEN EXISTS (
				SELECT 1 FROM bridge_deliveries
				WHERE bridge_instance_id = OLD.id AND state = 'active'
			)
			BEGIN
				SELECT RAISE(ABORT, 'bridge instance has active deliveries');
			END;
-- +goose StatementEnd
-- recreate trigger "trg_bridge_instance_active_delivery_identity" after rebuilding table "bridge_instances"
-- +goose StatementBegin
CREATE TRIGGER trg_bridge_instance_active_delivery_identity
			BEFORE UPDATE OF scope, workspace_id, platform, extension_name ON bridge_instances
			WHEN EXISTS (
				SELECT 1 FROM bridge_deliveries
				WHERE bridge_instance_id = OLD.id AND state = 'active'
			) AND (
				NEW.scope IS NOT OLD.scope OR
				NEW.workspace_id IS NOT OLD.workspace_id OR
				NEW.platform IS NOT OLD.platform OR
				NEW.extension_name IS NOT OLD.extension_name
			)
			BEGIN
				SELECT RAISE(ABORT, 'active delivery locks bridge instance identity');
			END;
-- +goose StatementEnd
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
-- recreate trigger "trg_tasks_terminal_command_delete_guard" after rebuilding table "tasks"
-- +goose StatementBegin
CREATE TRIGGER trg_tasks_terminal_command_delete_guard
	BEFORE DELETE ON tasks
	WHEN EXISTS (
		SELECT 1
		FROM task_run_terminal_commands
		WHERE task_id = OLD.id
	)
	BEGIN
		SELECT RAISE(ABORT, 'task run terminal command in progress');
	END;
-- +goose StatementEnd
-- recreate trigger "worktrees_profile_owner_active" after rebuilding table "worktrees"
-- +goose StatementBegin
CREATE TRIGGER worktrees_profile_owner_active BEFORE INSERT ON worktrees BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd
-- recreate trigger "worktrees_profile_owner_immutable" after rebuilding table "worktrees"
-- +goose StatementBegin
CREATE TRIGGER worktrees_profile_owner_immutable BEFORE UPDATE OF profile_id ON worktrees
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
-- recreate the workspace-owned selection cleanup after introducing profile_selections
-- +goose StatementBegin
CREATE TRIGGER profile_selections_workspace_delete AFTER DELETE ON workspaces BEGIN
	DELETE FROM profile_selections WHERE lens = 'workspace' AND workspace_id = OLD.id;
END;
-- +goose StatementEnd
