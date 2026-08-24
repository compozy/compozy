-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
CREATE TABLE IF NOT EXISTS phase0_operator_home_context (
	home_workspace_id TEXT PRIMARY KEY
);
CREATE TEMP TABLE phase0_home_workspaces (
	id TEXT PRIMARY KEY
);
INSERT INTO phase0_home_workspaces (id)
SELECT home_workspace_id
FROM phase0_operator_home_context;
CREATE TEMP TABLE phase0_assert_empty (
	family TEXT NOT NULL CHECK (family <> 'worktrees')
);
INSERT INTO phase0_assert_empty (family)
SELECT 'worktrees'
WHERE EXISTS (
	SELECT 1
	FROM worktrees
	WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces)
);
DROP TABLE phase0_assert_empty;
-- create "new_agent_heartbeat_revisions" table
CREATE TABLE `new_agent_heartbeat_revisions` (`id` text NULL, `workspace_id` text NOT NULL, `agent_name` text NOT NULL, `source_path` text NOT NULL, `operation` text NOT NULL, `previous_digest` text NULL, `new_digest` text NULL, `new_snapshot_id` text NULL, `body` text NULL, `actor_kind` text NOT NULL, `actor_id` text NOT NULL, `created_at` text NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`new_snapshot_id`) REFERENCES `agent_heartbeat_snapshots` (`id`) ON UPDATE NO ACTION ON DELETE SET NULL, CHECK (operation IN ('write', 'delete', 'rollback')), CHECK (actor_kind IN ('user', 'agent', 'extension', 'system')));
-- copy rows from old table "agent_heartbeat_revisions" to new temporary table "new_agent_heartbeat_revisions"
INSERT INTO `new_agent_heartbeat_revisions` (`id`, `workspace_id`, `agent_name`, `source_path`, `operation`, `previous_digest`, `new_digest`, `new_snapshot_id`, `body`, `actor_kind`, `actor_id`, `created_at`) SELECT `id`, `workspace_id`, `agent_name`, `source_path`, `operation`, `previous_digest`, `new_digest`, `new_snapshot_id`, `body`, `actor_kind`, `actor_id`, `created_at` FROM `agent_heartbeat_revisions`;
-- drop trigger "agent_heartbeat_revisions_workspace_insert_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `agent_heartbeat_revisions_workspace_insert_guard`;
-- drop trigger "agent_heartbeat_revisions_workspace_update_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `agent_heartbeat_revisions_workspace_update_guard`;
-- drop trigger "agent_heartbeat_snapshots_workspace_insert_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `agent_heartbeat_snapshots_workspace_insert_guard`;
-- drop trigger "agent_heartbeat_snapshots_workspace_update_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `agent_heartbeat_snapshots_workspace_update_guard`;
-- drop trigger "agent_heartbeat_wake_events_workspace_insert_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `agent_heartbeat_wake_events_workspace_insert_guard`;
-- drop trigger "agent_heartbeat_wake_events_workspace_update_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `agent_heartbeat_wake_events_workspace_update_guard`;
-- drop trigger "agent_heartbeat_wake_state_workspace_insert_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `agent_heartbeat_wake_state_workspace_insert_guard`;
-- drop trigger "agent_heartbeat_wake_state_workspace_update_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `agent_heartbeat_wake_state_workspace_update_guard`;
-- drop trigger "agent_soul_revisions_workspace_insert_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `agent_soul_revisions_workspace_insert_guard`;
-- drop trigger "agent_soul_revisions_workspace_update_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `agent_soul_revisions_workspace_update_guard`;
-- drop trigger "agent_soul_snapshots_workspace_insert_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `agent_soul_snapshots_workspace_insert_guard`;
-- drop trigger "agent_soul_snapshots_workspace_update_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `agent_soul_snapshots_workspace_update_guard`;
-- drop trigger "dead_entities_workspace_insert_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `dead_entities_workspace_insert_guard`;
-- drop trigger "dead_entities_workspace_update_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `dead_entities_workspace_update_guard`;
-- drop trigger "network_channels_workspace_insert_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `network_channels_workspace_insert_guard`;
-- drop trigger "network_channels_workspace_update_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `network_channels_workspace_update_guard`;
-- drop trigger "network_coordination_invitations_workspace_insert_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `network_coordination_invitations_workspace_insert_guard`;
-- drop trigger "network_coordination_invitations_workspace_update_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `network_coordination_invitations_workspace_update_guard`;
-- drop trigger "network_live_wakes_workspace_insert_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `network_live_wakes_workspace_insert_guard`;
-- drop trigger "network_live_wakes_workspace_update_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `network_live_wakes_workspace_update_guard`;
-- drop trigger "network_message_dispositions_workspace_insert_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `network_message_dispositions_workspace_insert_guard`;
-- drop trigger "network_message_dispositions_workspace_update_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `network_message_dispositions_workspace_update_guard`;
-- drop trigger "network_participation_budgets_workspace_insert_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `network_participation_budgets_workspace_insert_guard`;
-- drop trigger "network_participation_budgets_workspace_update_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `network_participation_budgets_workspace_update_guard`;
-- drop trigger "network_wake_events_workspace_insert_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `network_wake_events_workspace_insert_guard`;
-- drop trigger "network_wake_events_workspace_update_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `network_wake_events_workspace_update_guard`;
-- drop trigger "network_wake_sources_workspace_insert_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `network_wake_sources_workspace_insert_guard`;
-- drop trigger "network_wake_sources_workspace_update_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `network_wake_sources_workspace_update_guard`;
-- drop trigger "no_workspace_data_workspace_delete" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `no_workspace_data_workspace_delete`;
-- drop trigger "session_health_workspace_insert_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `session_health_workspace_insert_guard`;
-- drop trigger "session_health_workspace_update_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `session_health_workspace_update_guard`;
-- drop trigger "sessions_workspace_insert_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `sessions_workspace_insert_guard`;
-- drop trigger "sessions_workspace_update_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `sessions_workspace_update_guard`;
-- drop trigger "task_network_coordination_workspace_insert_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `task_network_coordination_workspace_insert_guard`;
-- drop trigger "task_network_coordination_workspace_update_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `task_network_coordination_workspace_update_guard`;
-- drop trigger "tool_approval_grants_workspace_insert_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `tool_approval_grants_workspace_insert_guard`;
-- drop trigger "tool_approval_grants_workspace_update_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `tool_approval_grants_workspace_update_guard`;
-- drop trigger "trg_sessions_archive_insert_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `trg_sessions_archive_insert_guard`;
-- drop trigger "trg_sessions_archive_update_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `trg_sessions_archive_update_guard`;
-- drop trigger "trg_task_runs_terminal_command_delete_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `trg_task_runs_terminal_command_delete_guard`;
-- drop trigger "trg_task_runs_terminal_command_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `trg_task_runs_terminal_command_guard`;
-- drop "agent_heartbeat_revisions" table after copying rows
DROP TABLE `agent_heartbeat_revisions`;
-- rename temporary table "new_agent_heartbeat_revisions" to "agent_heartbeat_revisions"
ALTER TABLE `new_agent_heartbeat_revisions` RENAME TO `agent_heartbeat_revisions`;
-- create index "idx_agent_heartbeat_revisions_agent_created" to table: "agent_heartbeat_revisions"
CREATE INDEX `idx_agent_heartbeat_revisions_agent_created` ON `agent_heartbeat_revisions` (`workspace_id`, `agent_name`, `created_at` DESC, `id` DESC);
-- create "new_agent_heartbeat_snapshots" table
CREATE TABLE `new_agent_heartbeat_snapshots` (`id` text NULL, `workspace_id` text NOT NULL, `agent_name` text NOT NULL, `source_path` text NOT NULL, `schema_version` integer NOT NULL DEFAULT 1, `digest` text NOT NULL, `config_digest` text NOT NULL, `body` text NOT NULL, `frontmatter_json` text NOT NULL, `resolved_json` text NOT NULL, `diagnostics_json` text NOT NULL, `created_at` text NOT NULL, PRIMARY KEY (`id`));
-- copy rows from old table "agent_heartbeat_snapshots" to new temporary table "new_agent_heartbeat_snapshots"
INSERT INTO `new_agent_heartbeat_snapshots` (`id`, `workspace_id`, `agent_name`, `source_path`, `schema_version`, `digest`, `config_digest`, `body`, `frontmatter_json`, `resolved_json`, `diagnostics_json`, `created_at`) SELECT `id`, `workspace_id`, `agent_name`, `source_path`, `schema_version`, `digest`, `config_digest`, `body`, `frontmatter_json`, `resolved_json`, `diagnostics_json`, `created_at` FROM `agent_heartbeat_snapshots`;
-- drop "agent_heartbeat_snapshots" table after copying rows
DROP TABLE `agent_heartbeat_snapshots`;
-- rename temporary table "new_agent_heartbeat_snapshots" to "agent_heartbeat_snapshots"
ALTER TABLE `new_agent_heartbeat_snapshots` RENAME TO `agent_heartbeat_snapshots`;
-- create index "agent_heartbeat_snapshots_workspace_id_agent_name_digest" to table: "agent_heartbeat_snapshots"
CREATE UNIQUE INDEX `agent_heartbeat_snapshots_workspace_id_agent_name_digest` ON `agent_heartbeat_snapshots` (`workspace_id`, `agent_name`, `digest`);
-- create index "idx_agent_heartbeat_snapshots_agent_created" to table: "agent_heartbeat_snapshots"
CREATE INDEX `idx_agent_heartbeat_snapshots_agent_created` ON `agent_heartbeat_snapshots` (`workspace_id`, `agent_name`, `created_at` DESC, `id` DESC);
-- create "new_agent_heartbeat_wake_events" table
CREATE TABLE `new_agent_heartbeat_wake_events` (`id` text NULL, `workspace_id` text NOT NULL, `agent_name` text NOT NULL, `session_id` text NULL, `policy_snapshot_id` text NULL, `source` text NOT NULL, `result` text NOT NULL, `reason` text NOT NULL, `synthetic_prompt_id` text NULL, `created_at` text NOT NULL, `expires_at` text NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`policy_snapshot_id`) REFERENCES `agent_heartbeat_snapshots` (`id`) ON UPDATE NO ACTION ON DELETE SET NULL, CONSTRAINT `1` FOREIGN KEY (`session_id`) REFERENCES `sessions` (`id`) ON UPDATE NO ACTION ON DELETE SET NULL, CHECK (source IN ('scheduler', 'manual', 'harness_reentry')), CHECK (
				result IN ('sent', 'skipped', 'coalesced', 'rate_limited', 'failed')
			), CHECK (
				reason IN (
					'wake_sent',
					'heartbeat_disabled',
					'heartbeat_invalid',
					'heartbeat_no_policy',
					'heartbeat_rate_limited',
					'heartbeat_no_eligible_session',
					'cooldown_active',
					'quiet_window',
					'session_not_found',
					'session_unhealthy',
					'session_not_attachable',
					'session_prompt_active',
					'session_prompt_active_race',
					'synthetic_prompt_failed',
					'wake_coalesced'
				)
			));
-- copy rows from old table "agent_heartbeat_wake_events" to new temporary table "new_agent_heartbeat_wake_events"
INSERT INTO `new_agent_heartbeat_wake_events` (`id`, `workspace_id`, `agent_name`, `session_id`, `policy_snapshot_id`, `source`, `result`, `reason`, `synthetic_prompt_id`, `created_at`, `expires_at`) SELECT `id`, `workspace_id`, `agent_name`, `session_id`, `policy_snapshot_id`, `source`, `result`, `reason`, `synthetic_prompt_id`, `created_at`, `expires_at` FROM `agent_heartbeat_wake_events`;
-- drop "agent_heartbeat_wake_events" table after copying rows
DROP TABLE `agent_heartbeat_wake_events`;
-- rename temporary table "new_agent_heartbeat_wake_events" to "agent_heartbeat_wake_events"
ALTER TABLE `new_agent_heartbeat_wake_events` RENAME TO `agent_heartbeat_wake_events`;
-- create index "idx_agent_heartbeat_wake_events_agent_created" to table: "agent_heartbeat_wake_events"
CREATE INDEX `idx_agent_heartbeat_wake_events_agent_created` ON `agent_heartbeat_wake_events` (`workspace_id`, `agent_name`, `created_at` DESC, `id` DESC);
-- create index "idx_agent_heartbeat_wake_events_expires" to table: "agent_heartbeat_wake_events"
CREATE INDEX `idx_agent_heartbeat_wake_events_expires` ON `agent_heartbeat_wake_events` (`expires_at`);
-- create "new_agent_heartbeat_wake_state" table
CREATE TABLE `new_agent_heartbeat_wake_state` (`workspace_id` text NOT NULL, `agent_name` text NOT NULL, `session_id` text NOT NULL, `policy_snapshot_id` text NULL, `last_wake_at` text NULL, `next_allowed_at` text NULL, `coalesced_count` integer NOT NULL DEFAULT 0, `last_result` text NOT NULL, `last_reason` text NULL, `updated_at` text NOT NULL, PRIMARY KEY (`workspace_id`, `agent_name`, `session_id`), CONSTRAINT `0` FOREIGN KEY (`policy_snapshot_id`) REFERENCES `agent_heartbeat_snapshots` (`id`) ON UPDATE NO ACTION ON DELETE SET NULL, CONSTRAINT `1` FOREIGN KEY (`session_id`) REFERENCES `sessions` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (coalesced_count >= 0), CHECK (
				last_result IN ('sent', 'skipped', 'coalesced', 'rate_limited', 'failed')
			), CHECK (
				last_reason IS NULL OR last_reason IN (
					'wake_sent',
					'heartbeat_disabled',
					'heartbeat_invalid',
					'heartbeat_no_policy',
					'heartbeat_rate_limited',
					'heartbeat_no_eligible_session',
					'cooldown_active',
					'quiet_window',
					'session_not_found',
					'session_unhealthy',
					'session_not_attachable',
					'session_prompt_active',
					'session_prompt_active_race',
					'synthetic_prompt_failed',
					'wake_coalesced'
				)
			));
-- copy rows from old table "agent_heartbeat_wake_state" to new temporary table "new_agent_heartbeat_wake_state"
INSERT INTO `new_agent_heartbeat_wake_state` (`workspace_id`, `agent_name`, `session_id`, `policy_snapshot_id`, `last_wake_at`, `next_allowed_at`, `coalesced_count`, `last_result`, `last_reason`, `updated_at`) SELECT `workspace_id`, `agent_name`, `session_id`, `policy_snapshot_id`, `last_wake_at`, `next_allowed_at`, `coalesced_count`, `last_result`, `last_reason`, `updated_at` FROM `agent_heartbeat_wake_state`;
-- drop "agent_heartbeat_wake_state" table after copying rows
DROP TABLE `agent_heartbeat_wake_state`;
-- rename temporary table "new_agent_heartbeat_wake_state" to "agent_heartbeat_wake_state"
ALTER TABLE `new_agent_heartbeat_wake_state` RENAME TO `agent_heartbeat_wake_state`;
-- create index "idx_agent_heartbeat_wake_state_next_allowed" to table: "agent_heartbeat_wake_state"
CREATE INDEX `idx_agent_heartbeat_wake_state_next_allowed` ON `agent_heartbeat_wake_state` (`next_allowed_at`, `updated_at` DESC);
-- create "new_agent_soul_revisions" table
CREATE TABLE `new_agent_soul_revisions` (`id` text NULL, `workspace_id` text NOT NULL, `agent_name` text NOT NULL, `source_path` text NOT NULL, `action` text NOT NULL, `previous_digest` text NOT NULL DEFAULT '', `new_digest` text NOT NULL DEFAULT '', `body` text NOT NULL DEFAULT '', `diagnostics_json` text NOT NULL DEFAULT '[]', `actor_kind` text NOT NULL DEFAULT '', `actor_id` text NOT NULL DEFAULT '', `origin_kind` text NOT NULL DEFAULT '', `origin_ref` text NOT NULL DEFAULT '', `created_at` text NOT NULL, PRIMARY KEY (`id`), CHECK (action IN ('put', 'delete', 'rollback')));
-- copy rows from old table "agent_soul_revisions" to new temporary table "new_agent_soul_revisions"
INSERT INTO `new_agent_soul_revisions` (`id`, `workspace_id`, `agent_name`, `source_path`, `action`, `previous_digest`, `new_digest`, `body`, `diagnostics_json`, `actor_kind`, `actor_id`, `origin_kind`, `origin_ref`, `created_at`) SELECT `id`, `workspace_id`, `agent_name`, `source_path`, `action`, `previous_digest`, `new_digest`, `body`, `diagnostics_json`, `actor_kind`, `actor_id`, `origin_kind`, `origin_ref`, `created_at` FROM `agent_soul_revisions`;
-- drop "agent_soul_revisions" table after copying rows
DROP TABLE `agent_soul_revisions`;
-- rename temporary table "new_agent_soul_revisions" to "agent_soul_revisions"
ALTER TABLE `new_agent_soul_revisions` RENAME TO `agent_soul_revisions`;
-- create index "idx_agent_soul_revisions_agent" to table: "agent_soul_revisions"
CREATE INDEX `idx_agent_soul_revisions_agent` ON `agent_soul_revisions` (`workspace_id`, `agent_name`, `created_at` DESC);
-- create "new_agent_soul_snapshots" table
CREATE TABLE `new_agent_soul_snapshots` (`id` text NULL, `workspace_id` text NOT NULL, `agent_name` text NOT NULL, `source_path` text NOT NULL, `digest` text NOT NULL, `profile_json` text NOT NULL DEFAULT '{}', `body` text NOT NULL DEFAULT '', `truncated` integer NOT NULL DEFAULT 0, `created_at` text NOT NULL, PRIMARY KEY (`id`), CHECK (truncated IN (0, 1)));
-- copy rows from old table "agent_soul_snapshots" to new temporary table "new_agent_soul_snapshots"
INSERT INTO `new_agent_soul_snapshots` (`id`, `workspace_id`, `agent_name`, `source_path`, `digest`, `profile_json`, `body`, `truncated`, `created_at`) SELECT `id`, `workspace_id`, `agent_name`, `source_path`, `digest`, `profile_json`, `body`, `truncated`, `created_at` FROM `agent_soul_snapshots`;
-- drop "agent_soul_snapshots" table after copying rows
DROP TABLE `agent_soul_snapshots`;
-- rename temporary table "new_agent_soul_snapshots" to "agent_soul_snapshots"
ALTER TABLE `new_agent_soul_snapshots` RENAME TO `agent_soul_snapshots`;
-- create index "agent_soul_snapshots_workspace_id_agent_name_digest" to table: "agent_soul_snapshots"
CREATE UNIQUE INDEX `agent_soul_snapshots_workspace_id_agent_name_digest` ON `agent_soul_snapshots` (`workspace_id`, `agent_name`, `digest`);
-- create index "idx_agent_soul_snapshots_agent" to table: "agent_soul_snapshots"
CREATE INDEX `idx_agent_soul_snapshots_agent` ON `agent_soul_snapshots` (`workspace_id`, `agent_name`, `created_at` DESC);
-- create "new_loop_goal_session_cleanup" table
CREATE TABLE `new_loop_goal_session_cleanup` (`id` integer NULL PRIMARY KEY AUTOINCREMENT, `cleanup_id` text NOT NULL, `workspace_id` text NOT NULL, `loop_run_id` text NOT NULL, `handle` text NOT NULL, `binding_epoch` integer NOT NULL, `session_id` text NOT NULL, `cause` text NOT NULL, `created_at` timestamp NOT NULL, `completed_at` timestamp NULL, CONSTRAINT `0` FOREIGN KEY (`loop_run_id`) REFERENCES `loop_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (length(trim(cleanup_id)) > 0), CHECK (length(trim(handle)) > 0), CHECK (binding_epoch >= 1), CHECK (length(trim(session_id)) > 0), CHECK (cause IN ('terminal','reseed','control-revoked','stop')), CHECK (completed_at IS NULL OR completed_at >= created_at));
-- copy rows from old table "loop_goal_session_cleanup" to new temporary table "new_loop_goal_session_cleanup"
INSERT INTO `new_loop_goal_session_cleanup` (`id`, `cleanup_id`, `workspace_id`, `loop_run_id`, `handle`, `binding_epoch`, `session_id`, `cause`, `created_at`, `completed_at`) SELECT `id`, `cleanup_id`, `workspace_id`, `loop_run_id`, `handle`, `binding_epoch`, `session_id`, `cause`, `created_at`, `completed_at` FROM `loop_goal_session_cleanup`;
-- drop "loop_goal_session_cleanup" table after copying rows
DROP TABLE `loop_goal_session_cleanup`;
-- rename temporary table "new_loop_goal_session_cleanup" to "loop_goal_session_cleanup"
ALTER TABLE `new_loop_goal_session_cleanup` RENAME TO `loop_goal_session_cleanup`;
-- create index "loop_goal_session_cleanup_cleanup_id" to table: "loop_goal_session_cleanup"
CREATE UNIQUE INDEX `loop_goal_session_cleanup_cleanup_id` ON `loop_goal_session_cleanup` (`cleanup_id`);
-- create index "loop_goal_session_cleanup_loop_run_id_handle_binding_epoch" to table: "loop_goal_session_cleanup"
CREATE UNIQUE INDEX `loop_goal_session_cleanup_loop_run_id_handle_binding_epoch` ON `loop_goal_session_cleanup` (`loop_run_id`, `handle`, `binding_epoch`);
-- create index "idx_loop_goal_session_cleanup_pending" to table: "loop_goal_session_cleanup"
CREATE INDEX `idx_loop_goal_session_cleanup_pending` ON `loop_goal_session_cleanup` (`id`) WHERE completed_at IS NULL;
-- create "new_loop_goal_session_outbox" table
CREATE TABLE `new_loop_goal_session_outbox` (`id` integer NULL PRIMARY KEY AUTOINCREMENT, `event_id` text NOT NULL, `workspace_id` text NOT NULL, `origin_session_id` text NOT NULL, `loop_run_id` text NOT NULL, `bound_session_id` text NULL, `cause` text NOT NULL, `created_at` timestamp NOT NULL, `delivered_at` timestamp NULL, CONSTRAINT `0` FOREIGN KEY (`loop_run_id`) REFERENCES `loop_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (length(trim(event_id)) > 0), CHECK (length(trim(origin_session_id)) > 0), CHECK (cause IN ('start','replace','status','clear','reseed')), CHECK (delivered_at IS NULL OR delivered_at >= created_at));
-- copy rows from old table "loop_goal_session_outbox" to new temporary table "new_loop_goal_session_outbox"
INSERT INTO `new_loop_goal_session_outbox` (`id`, `event_id`, `workspace_id`, `origin_session_id`, `loop_run_id`, `bound_session_id`, `cause`, `created_at`, `delivered_at`) SELECT `id`, `event_id`, `workspace_id`, `origin_session_id`, `loop_run_id`, `bound_session_id`, `cause`, `created_at`, `delivered_at` FROM `loop_goal_session_outbox`;
-- drop "loop_goal_session_outbox" table after copying rows
DROP TABLE `loop_goal_session_outbox`;
-- rename temporary table "new_loop_goal_session_outbox" to "loop_goal_session_outbox"
ALTER TABLE `new_loop_goal_session_outbox` RENAME TO `loop_goal_session_outbox`;
-- create index "loop_goal_session_outbox_event_id" to table: "loop_goal_session_outbox"
CREATE UNIQUE INDEX `loop_goal_session_outbox_event_id` ON `loop_goal_session_outbox` (`event_id`);
-- create index "idx_loop_goal_session_outbox_pending" to table: "loop_goal_session_outbox"
CREATE INDEX `idx_loop_goal_session_outbox_pending` ON `loop_goal_session_outbox` (`id`) WHERE delivered_at IS NULL;
-- create "new_loop_session_bindings" table
CREATE TABLE `new_loop_session_bindings` (`loop_run_id` text NOT NULL, `handle` text NOT NULL, `binding_epoch` integer NOT NULL, `binding_attempt_id` text NOT NULL, `session_id` text NOT NULL, `workspace_id` text NOT NULL, `creation_profile_ref` text NOT NULL, `policy_spec_digest` text NOT NULL, `creation_digest` text NOT NULL, `ownership` text NOT NULL, `state` text NOT NULL, `failure_code` text NULL, `created_at` timestamp NOT NULL, `activated_at` timestamp NULL, `failed_at` timestamp NULL, `closed_at` timestamp NULL, `adopted_generation` integer NOT NULL DEFAULT 0, `adoption_attempt_id` text NULL, PRIMARY KEY (`loop_run_id`, `handle`, `binding_epoch`), CONSTRAINT `0` FOREIGN KEY (`loop_run_id`) REFERENCES `loop_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (length(trim(handle)) > 0), CHECK (binding_epoch >= 1), CHECK (length(trim(binding_attempt_id)) > 0), CHECK (length(trim(session_id)) > 0), CHECK (length(trim(creation_profile_ref)) > 0), CHECK (length(trim(policy_spec_digest)) > 0), CHECK (length(trim(creation_digest)) > 0), CHECK (ownership IN ('origin-borrowed','run-owned')), CHECK (state IN ('creating','active','failed','closed','reseeded')), CHECK (activated_at IS NULL OR activated_at >= created_at), CHECK (failed_at IS NULL OR failed_at >= created_at), CHECK (closed_at IS NULL OR closed_at >= created_at), CHECK (adopted_generation >= 0), CHECK (adoption_attempt_id IS NULL OR length(trim(adoption_attempt_id)) > 0), CHECK (
		(state = 'creating' AND activated_at IS NULL AND failed_at IS NULL AND failure_code IS NULL AND closed_at IS NULL)
		OR
		(state = 'active' AND activated_at IS NOT NULL AND failed_at IS NULL AND failure_code IS NULL AND closed_at IS NULL)
		OR
		(state = 'failed' AND activated_at IS NULL AND failed_at IS NOT NULL
		 AND length(trim(failure_code)) > 0 AND closed_at IS NULL)
		OR
		(state IN ('closed','reseeded') AND activated_at IS NOT NULL AND failed_at IS NULL
		 AND failure_code IS NULL AND closed_at IS NOT NULL)
	), CHECK (ownership = 'run-owned' OR (binding_epoch = 1 AND state IN ('active','closed'))));
-- copy rows from old table "loop_session_bindings" to new temporary table "new_loop_session_bindings"
INSERT INTO `new_loop_session_bindings` (`loop_run_id`, `handle`, `binding_epoch`, `binding_attempt_id`, `session_id`, `workspace_id`, `creation_profile_ref`, `policy_spec_digest`, `creation_digest`, `ownership`, `state`, `failure_code`, `created_at`, `activated_at`, `failed_at`, `closed_at`, `adopted_generation`, `adoption_attempt_id`) SELECT `loop_run_id`, `handle`, `binding_epoch`, `binding_attempt_id`, `session_id`, `workspace_id`, `creation_profile_ref`, `policy_spec_digest`, `creation_digest`, `ownership`, `state`, `failure_code`, `created_at`, `activated_at`, `failed_at`, `closed_at`, `adopted_generation`, `adoption_attempt_id` FROM `loop_session_bindings`;
-- drop "loop_session_bindings" table after copying rows
DROP TABLE `loop_session_bindings`;
-- rename temporary table "new_loop_session_bindings" to "loop_session_bindings"
ALTER TABLE `new_loop_session_bindings` RENAME TO `loop_session_bindings`;
-- create index "loop_session_bindings_binding_attempt_id" to table: "loop_session_bindings"
CREATE UNIQUE INDEX `loop_session_bindings_binding_attempt_id` ON `loop_session_bindings` (`binding_attempt_id`);
-- create index "uq_loop_session_bindings_active" to table: "loop_session_bindings"
CREATE UNIQUE INDEX `uq_loop_session_bindings_active` ON `loop_session_bindings` (`loop_run_id`, `handle`) WHERE state='active';
-- create "new_network_channels" table
CREATE TABLE `new_network_channels` (`workspace_id` text NOT NULL, `channel` text NOT NULL, `purpose` text NOT NULL, `created_by` text NOT NULL DEFAULT '', `created_at` text NOT NULL, `updated_at` text NOT NULL, `fanout_policy` text NOT NULL DEFAULT 'capability_match', `coordinator_peer_id` text NOT NULL DEFAULT '', PRIMARY KEY (`workspace_id`, `channel`), CHECK (
					fanout_policy IN ('capability_match', 'coordinator', 'all_members')
				));
-- copy rows from old table "network_channels" to new temporary table "new_network_channels"
INSERT INTO `new_network_channels` (`workspace_id`, `channel`, `purpose`, `created_by`, `created_at`, `updated_at`, `fanout_policy`, `coordinator_peer_id`) SELECT `workspace_id`, `channel`, `purpose`, `created_by`, `created_at`, `updated_at`, `fanout_policy`, `coordinator_peer_id` FROM `network_channels`;
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
-- create "new_session_health" table
CREATE TABLE `new_session_health` (`session_id` text NULL, `workspace_id` text NOT NULL, `agent_name` text NOT NULL, `state` text NOT NULL, `health` text NOT NULL, `active_prompt` boolean NOT NULL, `attachable` boolean NOT NULL, `eligible_for_wake` boolean NOT NULL, `ineligibility_reason` text NULL, `last_activity_at` text NULL, `last_presence_at` text NULL, `last_error` text NULL, `updated_at` text NOT NULL, PRIMARY KEY (`session_id`), CONSTRAINT `0` FOREIGN KEY (`session_id`) REFERENCES `sessions` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (state IN ('idle', 'prompting', 'stopped', 'detached')), CHECK (health IN ('healthy', 'degraded', 'stale', 'dead', 'unknown')), CHECK (active_prompt IN (0, 1)), CHECK (attachable IN (0, 1)), CHECK (eligible_for_wake IN (0, 1)));
-- copy rows from old table "session_health" to new temporary table "new_session_health"
INSERT INTO `new_session_health` (`session_id`, `workspace_id`, `agent_name`, `state`, `health`, `active_prompt`, `attachable`, `eligible_for_wake`, `ineligibility_reason`, `last_activity_at`, `last_presence_at`, `last_error`, `updated_at`) SELECT `session_id`, `workspace_id`, `agent_name`, `state`, `health`, `active_prompt`, `attachable`, `eligible_for_wake`, `ineligibility_reason`, `last_activity_at`, `last_presence_at`, `last_error`, `updated_at` FROM `session_health`;
-- drop "session_health" table after copying rows
DROP TABLE `session_health`;
-- rename temporary table "new_session_health" to "session_health"
ALTER TABLE `new_session_health` RENAME TO `session_health`;
-- create index "idx_session_health_wake" to table: "session_health"
CREATE INDEX `idx_session_health_wake` ON `session_health` (`workspace_id`, `agent_name`, `eligible_for_wake`, `active_prompt`, `attachable`);
-- create index "idx_session_health_workspace_agent" to table: "session_health"
CREATE INDEX `idx_session_health_workspace_agent` ON `session_health` (`workspace_id`, `agent_name`, `health`, `updated_at` DESC);
-- create "new_network_coordination_invitations" table
CREATE TABLE `new_network_coordination_invitations` (`workspace_id` text NOT NULL, `scope_kind` text NOT NULL, `scope_id` text NOT NULL, `dismissed_at` text NOT NULL, `dismissed_by` text NOT NULL, PRIMARY KEY (`workspace_id`, `scope_kind`, `scope_id`), CHECK (scope_kind IN ('workspace', 'task')), CHECK (length(trim(scope_id)) > 0), CHECK (length(trim(dismissed_by)) > 0), CHECK (scope_kind <> 'workspace' OR scope_id = workspace_id));
-- copy rows from old table "network_coordination_invitations" to new temporary table "new_network_coordination_invitations"
INSERT INTO `new_network_coordination_invitations` (`workspace_id`, `scope_kind`, `scope_id`, `dismissed_at`, `dismissed_by`) SELECT `workspace_id`, `scope_kind`, `scope_id`, `dismissed_at`, `dismissed_by` FROM `network_coordination_invitations`;
-- drop "network_coordination_invitations" table after copying rows
DROP TABLE `network_coordination_invitations`;
-- rename temporary table "new_network_coordination_invitations" to "network_coordination_invitations"
ALTER TABLE `new_network_coordination_invitations` RENAME TO `network_coordination_invitations`;
-- create "new_network_message_dispositions" table
CREATE TABLE `new_network_message_dispositions` (`workspace_id` text NOT NULL, `message_id` text NOT NULL, `recipient_session_id` text NOT NULL, `decision` text NOT NULL, `decided_at` text NOT NULL, `acceptance_seq` integer NOT NULL, PRIMARY KEY (`workspace_id`, `message_id`, `recipient_session_id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`, `message_id`) REFERENCES `network_timeline_log` (`workspace_id`, `message_id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`workspace_id`, `recipient_session_id`) REFERENCES `sessions` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (acceptance_seq >= 1));
-- copy rows from old table "network_message_dispositions" to new temporary table "new_network_message_dispositions"
INSERT INTO `new_network_message_dispositions` (`workspace_id`, `message_id`, `recipient_session_id`, `decision`, `decided_at`, `acceptance_seq`) SELECT `workspace_id`, `message_id`, `recipient_session_id`, `decision`, `decided_at`, `acceptance_seq` FROM `network_message_dispositions`;
-- drop "network_message_dispositions" table after copying rows
DROP TABLE `network_message_dispositions`;
-- rename temporary table "new_network_message_dispositions" to "network_message_dispositions"
ALTER TABLE `new_network_message_dispositions` RENAME TO `network_message_dispositions`;
-- create "new_network_live_wakes" table
CREATE TABLE `new_network_live_wakes` (`wake_id` text NOT NULL, `task_run_id` text NOT NULL, `owner_key` text NOT NULL, `workspace_id` text NOT NULL, `channel` text NOT NULL, `root_id` text NOT NULL, `depth` integer NOT NULL, `state` text NOT NULL, `coalesce_until` text NOT NULL, `reserved_wall_ms` integer NOT NULL, `actual_wall_ms` integer NULL, `reserved_at` text NOT NULL, `settled_at` text NULL, `input_tokens` integer NULL, `output_tokens` integer NULL, `usage_state` text NOT NULL DEFAULT '', `reason` text NOT NULL DEFAULT '', PRIMARY KEY (`wake_id`, `workspace_id`), CONSTRAINT `0` FOREIGN KEY (`task_run_id`) REFERENCES `task_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (length(trim(owner_key)) > 0), CHECK (depth >= 0), CHECK (state IN ('open', 'succeeded', 'failed', 'canceled', 'deadline_exceeded')), CHECK (reserved_wall_ms > 0), CHECK (actual_wall_ms IS NULL OR actual_wall_ms >= 0), CHECK (input_tokens IS NULL OR input_tokens >= 0), CHECK (output_tokens IS NULL OR output_tokens >= 0), CHECK (usage_state IN ('', 'actual', 'usage_unavailable')));
-- copy rows from old table "network_live_wakes" to new temporary table "new_network_live_wakes"
INSERT INTO `new_network_live_wakes` (`wake_id`, `task_run_id`, `owner_key`, `workspace_id`, `channel`, `root_id`, `depth`, `state`, `coalesce_until`, `reserved_wall_ms`, `actual_wall_ms`, `reserved_at`, `settled_at`, `input_tokens`, `output_tokens`, `usage_state`, `reason`) SELECT `wake_id`, `task_run_id`, `owner_key`, `workspace_id`, `channel`, `root_id`, `depth`, `state`, `coalesce_until`, `reserved_wall_ms`, `actual_wall_ms`, `reserved_at`, `settled_at`, `input_tokens`, `output_tokens`, `usage_state`, `reason` FROM `network_live_wakes`;
-- drop "network_live_wakes" table after copying rows
DROP TABLE `network_live_wakes`;
-- rename temporary table "new_network_live_wakes" to "network_live_wakes"
ALTER TABLE `new_network_live_wakes` RENAME TO `network_live_wakes`;
-- create index "network_live_wakes_task_run_id" to table: "network_live_wakes"
CREATE UNIQUE INDEX `network_live_wakes_task_run_id` ON `network_live_wakes` (`task_run_id`);
-- create index "network_live_wakes_workspace_id_wake_id_owner_key" to table: "network_live_wakes"
CREATE UNIQUE INDEX `network_live_wakes_workspace_id_wake_id_owner_key` ON `network_live_wakes` (`workspace_id`, `wake_id`, `owner_key`);
-- create index "uq_network_live_wakes_open_owner" to table: "network_live_wakes"
CREATE UNIQUE INDEX `uq_network_live_wakes_open_owner` ON `network_live_wakes` (`workspace_id`, `owner_key`) WHERE state = 'open';
-- create index "idx_network_live_wakes_workspace_channel" to table: "network_live_wakes"
CREATE INDEX `idx_network_live_wakes_workspace_channel` ON `network_live_wakes` (`workspace_id`, `channel`, `reserved_at` DESC, `wake_id`);
-- create "new_network_wake_sources" table
CREATE TABLE `new_network_wake_sources` (`workspace_id` text NOT NULL, `owner_key` text NOT NULL, `envelope_id` text NOT NULL, `wake_id` text NOT NULL, PRIMARY KEY (`workspace_id`, `owner_key`, `envelope_id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`, `envelope_id`) REFERENCES `network_timeline_log` (`workspace_id`, `message_id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`workspace_id`, `wake_id`, `owner_key`) REFERENCES `network_live_wakes` (`workspace_id`, `wake_id`, `owner_key`) ON UPDATE NO ACTION ON DELETE CASCADE);
-- copy rows from old table "network_wake_sources" to new temporary table "new_network_wake_sources"
INSERT INTO `new_network_wake_sources` (`workspace_id`, `owner_key`, `envelope_id`, `wake_id`) SELECT `workspace_id`, `owner_key`, `envelope_id`, `wake_id` FROM `network_wake_sources`;
-- drop "network_wake_sources" table after copying rows
DROP TABLE `network_wake_sources`;
-- rename temporary table "new_network_wake_sources" to "network_wake_sources"
ALTER TABLE `new_network_wake_sources` RENAME TO `network_wake_sources`;
-- create "new_network_participation_budgets" table
CREATE TABLE `new_network_participation_budgets` (`workspace_id` text NOT NULL, `owner_key` text NOT NULL, `wakes_used` integer NOT NULL DEFAULT 0, `wall_ms_used` integer NOT NULL DEFAULT 0, `input_tokens_used` integer NOT NULL DEFAULT 0, `output_tokens_used` integer NOT NULL DEFAULT 0, `exhausted_reason` text NOT NULL DEFAULT '', `updated_at` text NOT NULL, PRIMARY KEY (`workspace_id`, `owner_key`), CHECK (length(trim(owner_key)) > 0), CHECK (wakes_used >= 0), CHECK (wall_ms_used >= 0), CHECK (input_tokens_used >= 0), CHECK (output_tokens_used >= 0));
-- copy rows from old table "network_participation_budgets" to new temporary table "new_network_participation_budgets"
INSERT INTO `new_network_participation_budgets` (`workspace_id`, `owner_key`, `wakes_used`, `wall_ms_used`, `input_tokens_used`, `output_tokens_used`, `exhausted_reason`, `updated_at`) SELECT `workspace_id`, `owner_key`, `wakes_used`, `wall_ms_used`, `input_tokens_used`, `output_tokens_used`, `exhausted_reason`, `updated_at` FROM `network_participation_budgets`;
-- drop "network_participation_budgets" table after copying rows
DROP TABLE `network_participation_budgets`;
-- rename temporary table "new_network_participation_budgets" to "network_participation_budgets"
ALTER TABLE `new_network_participation_budgets` RENAME TO `network_participation_budgets`;
-- create "new_task_network_coordination" table
CREATE TABLE `new_task_network_coordination` (`task_id` text NOT NULL, `workspace_id` text NOT NULL, `enabled` integer NOT NULL, `revision` integer NOT NULL, `updated_at` text NOT NULL, `updated_by` text NOT NULL, PRIMARY KEY (`task_id`), CONSTRAINT `0` FOREIGN KEY (`task_id`) REFERENCES `tasks` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (enabled IN (0, 1)), CHECK (revision >= 1), CHECK (length(trim(updated_by)) > 0));
-- copy rows from old table "task_network_coordination" to new temporary table "new_task_network_coordination"
INSERT INTO `new_task_network_coordination` (`task_id`, `workspace_id`, `enabled`, `revision`, `updated_at`, `updated_by`) SELECT `task_id`, `workspace_id`, `enabled`, `revision`, `updated_at`, `updated_by` FROM `task_network_coordination`;
-- drop "task_network_coordination" table after copying rows
DROP TABLE `task_network_coordination`;
-- rename temporary table "new_task_network_coordination" to "task_network_coordination"
ALTER TABLE `new_task_network_coordination` RENAME TO `task_network_coordination`;
-- create index "idx_task_network_coordination_workspace" to table: "task_network_coordination"
CREATE INDEX `idx_task_network_coordination_workspace` ON `task_network_coordination` (`workspace_id`, `task_id`);
-- create "new_tool_approval_grants" table
CREATE TABLE `new_tool_approval_grants` (`id` text NOT NULL, `workspace_id` text NOT NULL, `agent_name` text NOT NULL DEFAULT '', `tool_id` text NOT NULL, `input_digest` text NOT NULL DEFAULT '', `decision` text NOT NULL, `created_at` text NOT NULL, `last_used_at` text NOT NULL, PRIMARY KEY (`id`), CHECK (trim(id) <> ''), CHECK (trim(tool_id) <> ''), CHECK (input_digest = '' OR input_digest LIKE 'sha256:%'), CHECK (decision IN ('allow', 'reject')));
-- copy rows from old table "tool_approval_grants" to new temporary table "new_tool_approval_grants"
INSERT INTO `new_tool_approval_grants` (`id`, `workspace_id`, `agent_name`, `tool_id`, `input_digest`, `decision`, `created_at`, `last_used_at`) SELECT `id`, `workspace_id`, `agent_name`, `tool_id`, `input_digest`, `decision`, `created_at`, `last_used_at` FROM `tool_approval_grants`;
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
CREATE TABLE `new_automation_suggestions` (`id` text NULL, `workspace_id` text NULL, `source` text NOT NULL, `dedup_key` text NOT NULL, `status` text NOT NULL, `payload` text NOT NULL, `created_at` text NOT NULL, `resolved_at` text NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (source IN ('catalog', 'usage', 'integration')), CHECK (status IN ('pending', 'accepted', 'dismissed')), CHECK (json_valid(payload) AND json_type(payload) = 'object'), CHECK (
			(status = 'pending' AND resolved_at IS NULL) OR
			(status IN ('accepted', 'dismissed') AND resolved_at IS NOT NULL)
		));
-- copy rows from old table "automation_suggestions" to new temporary table "new_automation_suggestions"
INSERT INTO `new_automation_suggestions` (`id`, `workspace_id`, `source`, `dedup_key`, `status`, `payload`, `created_at`, `resolved_at`) SELECT `id`, `workspace_id`, `source`, `dedup_key`, `status`, `payload`, `created_at`, `resolved_at` FROM `automation_suggestions`;
-- drop "automation_suggestions" table after copying rows
DROP TABLE `automation_suggestions`;
-- rename temporary table "new_automation_suggestions" to "automation_suggestions"
ALTER TABLE `new_automation_suggestions` RENAME TO `automation_suggestions`;
-- create index "idx_automation_suggestions_workspace_status" to table: "automation_suggestions"
CREATE INDEX `idx_automation_suggestions_workspace_status` ON `automation_suggestions` (`workspace_id`, `status`, `created_at`, `id`);
-- create index "automation_suggestions_workspace_id_dedup_key" to table: "automation_suggestions"
CREATE UNIQUE INDEX `automation_suggestions_workspace_id_dedup_key` ON `automation_suggestions` (`workspace_id`, `dedup_key`);
-- create "new_dead_entities" table
CREATE TABLE `new_dead_entities` (`workspace_id` text NOT NULL, `kind` text NOT NULL, `entity_id` text NOT NULL, `reason` text NOT NULL, `marked_at` text NOT NULL, PRIMARY KEY (`workspace_id`, `kind`, `entity_id`), CHECK (kind IN ('extension', 'bridge', 'mcp_sidecar', 'loop_target')), CHECK (trim(entity_id) <> ''), CHECK (trim(reason) <> ''));
-- copy rows from old table "dead_entities" to new temporary table "new_dead_entities"
INSERT INTO `new_dead_entities` (`workspace_id`, `kind`, `entity_id`, `reason`, `marked_at`) SELECT `workspace_id`, `kind`, `entity_id`, `reason`, `marked_at` FROM `dead_entities`;
-- drop "dead_entities" table after copying rows
DROP TABLE `dead_entities`;
-- rename temporary table "new_dead_entities" to "dead_entities"
ALTER TABLE `new_dead_entities` RENAME TO `dead_entities`;
-- create "new_network_wake_events" table
CREATE TABLE `new_network_wake_events` (`sequence` integer NULL PRIMARY KEY AUTOINCREMENT, `workspace_id` text NOT NULL, `wake_id` text NOT NULL, `task_run_id` text NOT NULL, `owner_key` text NOT NULL, `target_session_id` text NOT NULL, `event_type` text NOT NULL, `state` text NOT NULL, `claim_token_hash` text NOT NULL DEFAULT '', `usage_state` text NOT NULL DEFAULT '', `actual_wall_ms` integer NULL, `input_tokens` integer NULL, `output_tokens` integer NULL, `reason` text NOT NULL DEFAULT '', `actor_kind` text NOT NULL, `actor_ref` text NOT NULL, `timestamp` text NOT NULL, CONSTRAINT `0` FOREIGN KEY (`workspace_id`, `target_session_id`) REFERENCES `sessions` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`workspace_id`, `wake_id`, `owner_key`) REFERENCES `network_live_wakes` (`workspace_id`, `wake_id`, `owner_key`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `2` FOREIGN KEY (`task_run_id`) REFERENCES `task_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (length(trim(owner_key)) > 0), CHECK (
		event_type IN ('admitted', 'claimed', 'heartbeat', 'released', 'recovered', 'settled')
	), CHECK (usage_state IN ('', 'actual', 'usage_unavailable')), CHECK (actual_wall_ms IS NULL OR actual_wall_ms >= 0), CHECK (input_tokens IS NULL OR input_tokens >= 0), CHECK (output_tokens IS NULL OR output_tokens >= 0));
-- copy rows from old table "network_wake_events" to new temporary table "new_network_wake_events"
INSERT INTO `new_network_wake_events` (`sequence`, `workspace_id`, `wake_id`, `task_run_id`, `owner_key`, `target_session_id`, `event_type`, `state`, `claim_token_hash`, `usage_state`, `actual_wall_ms`, `input_tokens`, `output_tokens`, `reason`, `actor_kind`, `actor_ref`, `timestamp`) SELECT `sequence`, `workspace_id`, `wake_id`, `task_run_id`, `owner_key`, `target_session_id`, `event_type`, `state`, `claim_token_hash`, `usage_state`, `actual_wall_ms`, `input_tokens`, `output_tokens`, `reason`, `actor_kind`, `actor_ref`, `timestamp` FROM `network_wake_events`;
-- drop "network_wake_events" table after copying rows
DROP TABLE `network_wake_events`;
-- rename temporary table "new_network_wake_events" to "network_wake_events"
ALTER TABLE `new_network_wake_events` RENAME TO `network_wake_events`;
-- create index "idx_network_wake_events_wake_sequence" to table: "network_wake_events"
CREATE INDEX `idx_network_wake_events_wake_sequence` ON `network_wake_events` (`workspace_id`, `wake_id`, `sequence`);
-- create "new_task_runs" table
CREATE TABLE `new_task_runs` (`id` text NULL, `task_id` text NULL, `workspace_id` text NULL, `worktree_id` text NULL, `status` text NOT NULL, `attempt` integer NOT NULL, `recovery_count` integer NOT NULL DEFAULT 0, `previous_run_id` text NULL, `failure_kind` text NOT NULL DEFAULT '', `claimed_by_kind` text NULL, `claimed_by_ref` text NULL, `session_id` text NULL, `origin_kind` text NOT NULL, `origin_ref` text NOT NULL, `idempotency_key` text NULL, `network_spec_json` text NOT NULL DEFAULT '{"version":"network-participation/v1","mode":"local","source":"built_in_local"}', `network_mode` text NOT NULL DEFAULT 'local', `network_channel` text NULL, `network_source` text NOT NULL DEFAULT 'built_in_local', `designation_group_id` text NOT NULL DEFAULT '', `resolved_worktree_mode` text NOT NULL DEFAULT '', `resolved_worktree_ref` text NOT NULL DEFAULT '', `queued_at` text NOT NULL, `claimed_at` text NULL, `started_at` text NULL, `ended_at` text NULL, `error` text NULL, `metadata_json` text NULL, `result_json` text NULL, `summary` text NOT NULL DEFAULT '', `claimed_agent_name` text NOT NULL DEFAULT '', `claimed_peer_id` text NOT NULL DEFAULT '', `terminalized_by_session_id` text NOT NULL DEFAULT '', `terminalized_by_agent_name` text NOT NULL DEFAULT '', `terminalized_by_peer_id` text NOT NULL DEFAULT '', `terminalized_by_actor_kind` text NOT NULL DEFAULT '', `terminalized_by_actor_ref` text NOT NULL DEFAULT '', `review_required` boolean NOT NULL DEFAULT 0, `review_request_round` integer NOT NULL DEFAULT 0, `review_policy_snapshot` text NOT NULL DEFAULT '', `review_request_id` text NULL, `parent_run_id` text NULL, `review_id` text NULL, `review_round` integer NOT NULL DEFAULT 0, `continuation_reason` text NOT NULL DEFAULT '', `missing_work_json` text NOT NULL DEFAULT '[]', `next_round_guidance` text NOT NULL DEFAULT '', `claim_token` text NULL, `claim_token_hash` text NULL, `lease_until` text NULL, `heartbeat_at` text NULL, `run_kind` text NOT NULL DEFAULT 'worker', `loop_run_id` text NULL, `tokens_used` integer NOT NULL DEFAULT 0, `network_wake_id` text NULL, `network_target_session_id` text NULL, `network_owner_key` text NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`, `worktree_id`) REFERENCES `worktrees` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `1` FOREIGN KEY (`network_target_session_id`) REFERENCES `sessions` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `2` FOREIGN KEY (`review_id`) REFERENCES `task_run_reviews` (`review_id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `3` FOREIGN KEY (`parent_run_id`) REFERENCES `task_runs` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `4` FOREIGN KEY (`review_request_id`) REFERENCES `task_run_reviews` (`review_id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `5` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `6` FOREIGN KEY (`task_id`) REFERENCES `tasks` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (attempt > 0), CHECK (recovery_count >= 0), CHECK (
			failure_kind = '' OR failure_kind IN ('operator_forced')
		), CHECK (
			claimed_by_kind IS NULL OR claimed_by_kind IN (
				'human', 'agent_session', 'automation', 'extension', 'network_peer', 'daemon'
			)
		), CHECK (
			origin_kind IN (
				'cli', 'web', 'uds', 'http', 'automation', 'extension', 'network', 'agent_session', 'daemon'
			)
		), CHECK (json_valid(network_spec_json)), CHECK (network_mode IN ('local', 'live')), CHECK (
			network_source IN (
				'explicit_request', 'task_profile', 'workspace_coordination',
				'loop_definition', 'automation_job', 'built_in_local'
			)
		), CHECK (
			resolved_worktree_mode IN ('', 'none', 'ref', 'per_run')
		), CHECK (review_required IN (0, 1)), CHECK (review_request_round >= 0), CHECK (
			review_policy_snapshot = '' OR
			review_policy_snapshot IN ('none', 'on_success', 'on_failure', 'always')
		), CHECK (review_round >= 0), CHECK (run_kind IN ('worker', 'coordinator', 'network_wake')), CHECK (tokens_used >= 0), CHECK (
			(claimed_by_kind IS NULL AND claimed_by_ref IS NULL) OR
			(claimed_by_kind IS NOT NULL AND claimed_by_ref IS NOT NULL)
		), CHECK (status <> 'queued' OR session_id IS NULL), CHECK (run_kind = 'network_wake' OR task_id IS NOT NULL), CHECK (run_kind <> 'network_wake' OR task_id IS NULL), CHECK (
			(resolved_worktree_mode = 'ref') = (resolved_worktree_ref <> '')
		), CHECK (
			(run_kind = 'network_wake' AND network_wake_id IS NOT NULL
				AND network_target_session_id IS NOT NULL AND network_owner_key IS NOT NULL) OR
			(run_kind <> 'network_wake' AND network_wake_id IS NULL
				AND network_target_session_id IS NULL AND network_owner_key IS NULL)
		));
-- copy rows from old table "task_runs" to new temporary table "new_task_runs"
INSERT INTO `new_task_runs` (`id`, `task_id`, `workspace_id`, `worktree_id`, `status`, `attempt`, `recovery_count`, `previous_run_id`, `failure_kind`, `claimed_by_kind`, `claimed_by_ref`, `session_id`, `origin_kind`, `origin_ref`, `idempotency_key`, `network_spec_json`, `network_mode`, `network_channel`, `network_source`, `designation_group_id`, `resolved_worktree_mode`, `resolved_worktree_ref`, `queued_at`, `claimed_at`, `started_at`, `ended_at`, `error`, `metadata_json`, `result_json`, `summary`, `claimed_agent_name`, `claimed_peer_id`, `terminalized_by_session_id`, `terminalized_by_agent_name`, `terminalized_by_peer_id`, `terminalized_by_actor_kind`, `terminalized_by_actor_ref`, `review_required`, `review_request_round`, `review_policy_snapshot`, `review_request_id`, `parent_run_id`, `review_id`, `review_round`, `continuation_reason`, `missing_work_json`, `next_round_guidance`, `claim_token`, `claim_token_hash`, `lease_until`, `heartbeat_at`, `run_kind`, `loop_run_id`, `tokens_used`, `network_wake_id`, `network_target_session_id`, `network_owner_key`) SELECT `id`, `task_id`, `workspace_id`, `worktree_id`, `status`, `attempt`, `recovery_count`, `previous_run_id`, `failure_kind`, `claimed_by_kind`, `claimed_by_ref`, `session_id`, `origin_kind`, `origin_ref`, `idempotency_key`, `network_spec_json`, `network_mode`, `network_channel`, `network_source`, `designation_group_id`, `resolved_worktree_mode`, `resolved_worktree_ref`, `queued_at`, `claimed_at`, `started_at`, `ended_at`, `error`, `metadata_json`, `result_json`, `summary`, `claimed_agent_name`, `claimed_peer_id`, `terminalized_by_session_id`, `terminalized_by_agent_name`, `terminalized_by_peer_id`, `terminalized_by_actor_kind`, `terminalized_by_actor_ref`, `review_required`, `review_request_round`, `review_policy_snapshot`, `review_request_id`, `parent_run_id`, `review_id`, `review_round`, `continuation_reason`, `missing_work_json`, `next_round_guidance`, `claim_token`, `claim_token_hash`, `lease_until`, `heartbeat_at`, `run_kind`, `loop_run_id`, `tokens_used`, `network_wake_id`, `network_target_session_id`, `network_owner_key` FROM `task_runs`;
-- drop "task_runs" table after copying rows
DROP TABLE `task_runs`;
-- rename temporary table "new_task_runs" to "task_runs"
ALTER TABLE `new_task_runs` RENAME TO `task_runs`;
-- create index "task_runs_workspace_id_id" to table: "task_runs"
CREATE UNIQUE INDEX `task_runs_workspace_id_id` ON `task_runs` (`workspace_id`, `id`);
-- create index "idx_task_runs_active_lease_recovery" to table: "task_runs"
CREATE INDEX `idx_task_runs_active_lease_recovery` ON `task_runs` (`status`, `lease_until`, `heartbeat_at`, `id`);
-- create index "idx_task_runs_channel" to table: "task_runs"
CREATE INDEX `idx_task_runs_channel` ON `task_runs` (`network_channel`);
-- create index "idx_task_runs_designation_group" to table: "task_runs"
CREATE INDEX `idx_task_runs_designation_group` ON `task_runs` (`task_id`, `designation_group_id`);
-- create index "idx_task_runs_parent_run" to table: "task_runs"
CREATE INDEX `idx_task_runs_parent_run` ON `task_runs` (`parent_run_id`);
-- create index "idx_task_runs_pending_claim" to table: "task_runs"
CREATE INDEX `idx_task_runs_pending_claim` ON `task_runs` (`status`, `lease_until`, `queued_at`, `id`);
-- create index "idx_task_runs_previous" to table: "task_runs"
CREATE INDEX `idx_task_runs_previous` ON `task_runs` (`previous_run_id`);
-- create index "idx_task_runs_review_request" to table: "task_runs"
CREATE INDEX `idx_task_runs_review_request` ON `task_runs` (`review_request_id`) WHERE review_request_id IS NOT NULL;
-- create index "idx_task_runs_session" to table: "task_runs"
CREATE INDEX `idx_task_runs_session` ON `task_runs` (`session_id`);
-- create index "idx_task_runs_session_status" to table: "task_runs"
CREATE INDEX `idx_task_runs_session_status` ON `task_runs` (`session_id`, `status`, `lease_until`);
-- create index "idx_task_runs_worktree" to table: "task_runs"
CREATE INDEX `idx_task_runs_worktree` ON `task_runs` (`worktree_id`) WHERE worktree_id IS NOT NULL;
-- create index "idx_task_runs_target_session" to table: "task_runs"
CREATE INDEX `idx_task_runs_target_session` ON `task_runs` (`network_target_session_id`, `status`, `queued_at`, `id`) WHERE run_kind = 'network_wake';
-- create index "idx_task_runs_status" to table: "task_runs"
CREATE INDEX `idx_task_runs_status` ON `task_runs` (`status`);
-- create index "idx_task_runs_task" to table: "task_runs"
CREATE INDEX `idx_task_runs_task` ON `task_runs` (`task_id`, `queued_at` DESC, `id` DESC);
-- create index "idx_task_runs_task_review_round" to table: "task_runs"
CREATE INDEX `idx_task_runs_task_review_round` ON `task_runs` (`task_id`, `review_round`) WHERE review_round > 0;
-- create index "idx_task_runs_task_status" to table: "task_runs"
CREATE INDEX `idx_task_runs_task_status` ON `task_runs` (`task_id`, `status`, `queued_at` DESC, `id` DESC);
-- create index "idx_task_runs_workspace_active" to table: "task_runs"
CREATE INDEX `idx_task_runs_workspace_active` ON `task_runs` (`workspace_id`, `status`, `lease_until`) WHERE workspace_id IS NOT NULL AND run_kind IN ('worker', 'coordinator');
-- create index "uq_task_runs_active_loop_coordinator" to table: "task_runs"
CREATE UNIQUE INDEX `uq_task_runs_active_loop_coordinator` ON `task_runs` (`loop_run_id`) WHERE run_kind = 'coordinator' AND status IN ('queued', 'claimed', 'starting', 'running');
-- create index "uq_task_runs_review_id" to table: "task_runs"
CREATE UNIQUE INDEX `uq_task_runs_review_id` ON `task_runs` (`review_id`) WHERE review_id IS NOT NULL;
-- create "new_sessions" table
CREATE TABLE `new_sessions` (`id` text NULL, `name` text NULL, `agent_name` text NOT NULL, `provider` text NOT NULL DEFAULT '', `model` text NOT NULL DEFAULT '', `reasoning_effort` text NOT NULL DEFAULT '', `speed` text NOT NULL DEFAULT '', `speed_resolution_json` text NOT NULL DEFAULT '', `runtime_status` text NOT NULL DEFAULT 'unbound', `runtime_transition` text NOT NULL DEFAULT '', `runtime_failure` text NOT NULL DEFAULT '', `runtime_generation` integer NOT NULL DEFAULT 0, `runtime_recovery_json` text NOT NULL DEFAULT '', `selected_provider` text NOT NULL DEFAULT '', `selected_model` text NOT NULL DEFAULT '', `selected_reasoning_effort` text NOT NULL DEFAULT '', `selected_speed` text NOT NULL DEFAULT '', `runtime_selection_revision` integer NOT NULL DEFAULT 0, `workspace_id` text NOT NULL, `scope` text NOT NULL DEFAULT 'workspace', `worktree_id` text NULL, `session_type` text NOT NULL DEFAULT 'user', `state` text NOT NULL, `archived_at` text NULL, `acp_session_id` text NULL, `stop_reason` text NULL, `stop_detail` text NULL, `subprocess_pid` integer NOT NULL DEFAULT 0, `subprocess_started_at` text NULL, `last_update_at` text NULL, `stall_state` text NOT NULL DEFAULT '', `stall_reason` text NOT NULL DEFAULT '', `activity_json` text NOT NULL DEFAULT '', `attached_to` text NOT NULL DEFAULT '', `attach_expires_at` text NULL, `transcript_epoch` integer NOT NULL DEFAULT 0, `pending_permission_count` integer NOT NULL DEFAULT 0, `pending_clarify_count` integer NOT NULL DEFAULT 0, `attention_revision` integer NOT NULL DEFAULT 0, `last_settled_revision` integer NOT NULL DEFAULT 0, `last_seen_revision` integer NOT NULL DEFAULT 0, `last_seen_at` text NULL, `attention_changed_at` text NULL, `sandbox_id` text NOT NULL DEFAULT '', `sandbox_backend` text NOT NULL DEFAULT 'local', `sandbox_profile` text NOT NULL DEFAULT '', `sandbox_instance_id` text NOT NULL DEFAULT '', `sandbox_state` text NOT NULL DEFAULT '', `sandbox_provider_state_json` text NOT NULL DEFAULT '', `sandbox_last_sync_at` text NULL, `sandbox_last_sync_error` text NOT NULL DEFAULT '', `created_at` text NOT NULL, `updated_at` text NOT NULL, `failure_kind` text NULL, `failure_summary` text NOT NULL DEFAULT '', `crash_bundle_path` text NOT NULL DEFAULT '', `parent_session_id` text NULL, `root_session_id` text NULL, `spawn_depth` integer NOT NULL DEFAULT 0, `spawn_role` text NULL, `ttl_expires_at` text NULL, `auto_stop_on_parent` boolean NOT NULL DEFAULT 0, `notify_creator` boolean NOT NULL DEFAULT 1, `spawn_budget_json` text NOT NULL DEFAULT '{}', `permission_policy_json` text NOT NULL DEFAULT '{}', `soul_snapshot_id` text NULL, `soul_digest` text NOT NULL DEFAULT '', `parent_soul_digest` text NOT NULL DEFAULT '', `input_generation` integer NOT NULL DEFAULT 0, `creation_digest` text NULL, `policy_spec_digest` text NULL, `creation_profile_ref` text NULL, `network_spec_json` text NOT NULL DEFAULT '{"version":"network-participation/v1","mode":"local","source":"built_in_local"}', `network_mode` text NOT NULL DEFAULT 'local', `network_channel` text NULL, `network_source` text NOT NULL DEFAULT 'built_in_local', PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`, `worktree_id`) REFERENCES `worktrees` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `1` FOREIGN KEY (`soul_snapshot_id`) REFERENCES `agent_soul_snapshots` (`id`) ON UPDATE NO ACTION ON DELETE SET NULL, CHECK (scope IN ('global', 'workspace')), CHECK (runtime_generation >= 0), CHECK (runtime_recovery_json = '' OR json_valid(runtime_recovery_json)), CHECK (pending_permission_count >= 0), CHECK (pending_clarify_count >= 0), CHECK (attention_revision >= 0), CHECK (last_settled_revision >= 0), CHECK (last_seen_revision >= 0), CHECK (creation_digest IS NULL OR length(trim(creation_digest)) > 0), CHECK (policy_spec_digest IS NULL OR length(trim(policy_spec_digest)) > 0), CHECK (creation_profile_ref IS NULL OR length(trim(creation_profile_ref)) > 0), CHECK (json_valid(network_spec_json)), CHECK (network_mode IN ('local', 'live')), CHECK (network_source IN (
					'explicit_request', 'task_profile', 'workspace_coordination',
					'loop_definition', 'automation_job', 'built_in_local'
				)), CHECK ((scope = 'workspace') = (workspace_id <> '')));
-- copy rows from old table "sessions" to new temporary table "new_sessions"
INSERT INTO `new_sessions` (`id`, `name`, `agent_name`, `provider`, `model`, `reasoning_effort`, `speed`, `speed_resolution_json`, `runtime_status`, `runtime_transition`, `runtime_failure`, `runtime_generation`, `runtime_recovery_json`, `selected_provider`, `selected_model`, `selected_reasoning_effort`, `selected_speed`, `runtime_selection_revision`, `workspace_id`, `worktree_id`, `session_type`, `state`, `archived_at`, `acp_session_id`, `stop_reason`, `stop_detail`, `subprocess_pid`, `subprocess_started_at`, `last_update_at`, `stall_state`, `stall_reason`, `activity_json`, `attached_to`, `attach_expires_at`, `transcript_epoch`, `pending_permission_count`, `pending_clarify_count`, `attention_revision`, `last_settled_revision`, `last_seen_revision`, `last_seen_at`, `attention_changed_at`, `sandbox_id`, `sandbox_backend`, `sandbox_profile`, `sandbox_instance_id`, `sandbox_state`, `sandbox_provider_state_json`, `sandbox_last_sync_at`, `sandbox_last_sync_error`, `created_at`, `updated_at`, `failure_kind`, `failure_summary`, `crash_bundle_path`, `parent_session_id`, `root_session_id`, `spawn_depth`, `spawn_role`, `ttl_expires_at`, `auto_stop_on_parent`, `notify_creator`, `spawn_budget_json`, `permission_policy_json`, `soul_snapshot_id`, `soul_digest`, `parent_soul_digest`, `input_generation`, `creation_digest`, `policy_spec_digest`, `creation_profile_ref`, `network_spec_json`, `network_mode`, `network_channel`, `network_source`) SELECT `id`, `name`, `agent_name`, `provider`, `model`, `reasoning_effort`, `speed`, `speed_resolution_json`, `runtime_status`, `runtime_transition`, `runtime_failure`, `runtime_generation`, `runtime_recovery_json`, `selected_provider`, `selected_model`, `selected_reasoning_effort`, `selected_speed`, `runtime_selection_revision`, `workspace_id`, `worktree_id`, `session_type`, `state`, `archived_at`, `acp_session_id`, `stop_reason`, `stop_detail`, `subprocess_pid`, `subprocess_started_at`, `last_update_at`, `stall_state`, `stall_reason`, `activity_json`, `attached_to`, `attach_expires_at`, `transcript_epoch`, `pending_permission_count`, `pending_clarify_count`, `attention_revision`, `last_settled_revision`, `last_seen_revision`, `last_seen_at`, `attention_changed_at`, `sandbox_id`, `sandbox_backend`, `sandbox_profile`, `sandbox_instance_id`, `sandbox_state`, `sandbox_provider_state_json`, `sandbox_last_sync_at`, `sandbox_last_sync_error`, `created_at`, `updated_at`, `failure_kind`, `failure_summary`, `crash_bundle_path`, `parent_session_id`, `root_session_id`, `spawn_depth`, `spawn_role`, `ttl_expires_at`, `auto_stop_on_parent`, `notify_creator`, `spawn_budget_json`, `permission_policy_json`, `soul_snapshot_id`, `soul_digest`, `parent_soul_digest`, `input_generation`, `creation_digest`, `policy_spec_digest`, `creation_profile_ref`, `network_spec_json`, `network_mode`, `network_channel`, `network_source` FROM `sessions`;
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
CREATE TABLE `new_tool_approval_pending` (`approval_id` text NOT NULL, `workspace_id` text NULL, `invocation_id` text NOT NULL, `target_kind` text NOT NULL, `tool_id` text NULL, `target_json` text NOT NULL DEFAULT '{}', `command_id` text NULL, `args_json` text NOT NULL, `approval_status` text NOT NULL, `execution_status` text NULL, `result_json` text NULL, `error_json` text NULL, `requested_at` integer NOT NULL, `expires_at` integer NOT NULL, `resolved_at` integer NULL, `executed_at` integer NULL, `resume_fence` integer NOT NULL DEFAULT 0, PRIMARY KEY (`approval_id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (approval_id LIKE 'apr_%'), CHECK (trim(invocation_id) <> ''), CHECK (target_kind IN ('tool', 'client_op', 'navigate', 'view')), CHECK (json_valid(target_json)), CHECK (json_valid(args_json)), CHECK (
		approval_status IN ('pending', 'approved', 'denied', 'timeout', 'canceled')
	), CHECK (
		execution_status IS NULL OR execution_status IN ('dispatching', 'completed', 'failed', 'uncertain')
	), CHECK (result_json IS NULL OR json_valid(result_json)), CHECK (error_json IS NULL OR json_valid(error_json)), CHECK (expires_at > requested_at), CHECK (resume_fence IN (0, 1)), CHECK ((target_kind = 'tool' AND trim(coalesce(tool_id, '')) <> '') OR target_kind <> 'tool'), CHECK ((approval_status = 'pending' AND resolved_at IS NULL) OR (approval_status <> 'pending' AND resolved_at IS NOT NULL)), CHECK ((execution_status IS NULL AND executed_at IS NULL) OR execution_status IS NOT NULL));
-- copy rows from old table "tool_approval_pending" to new temporary table "new_tool_approval_pending"
INSERT INTO `new_tool_approval_pending` (`approval_id`, `workspace_id`, `invocation_id`, `target_kind`, `tool_id`, `target_json`, `command_id`, `args_json`, `approval_status`, `execution_status`, `result_json`, `error_json`, `requested_at`, `expires_at`, `resolved_at`, `executed_at`, `resume_fence`) SELECT `approval_id`, `workspace_id`, `invocation_id`, `target_kind`, `tool_id`, `target_json`, `command_id`, `args_json`, `approval_status`, `execution_status`, `result_json`, `error_json`, `requested_at`, `expires_at`, `resolved_at`, `executed_at`, `resume_fence` FROM `tool_approval_pending`;
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

-- Preserve identity-bearing aggregate rows before re-keying the synthetic home scope.
INSERT INTO cmd_palette_usage (
	workspace_id, command_id, use_count, frecency_weight, last_used_at, updated_at
)
SELECT '', command_id, use_count, frecency_weight, last_used_at, updated_at
FROM cmd_palette_usage
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces)
ON CONFLICT (workspace_id, command_id) DO UPDATE SET
	use_count = cmd_palette_usage.use_count + excluded.use_count,
	frecency_weight = cmd_palette_usage.frecency_weight + excluded.frecency_weight,
	last_used_at = max(cmd_palette_usage.last_used_at, excluded.last_used_at),
	updated_at = max(cmd_palette_usage.updated_at, excluded.updated_at);
DELETE FROM cmd_palette_usage
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);

INSERT INTO cmd_palette_query_hits (
	workspace_id, query, command_id, weight, last_used_at
)
SELECT '', query, command_id, weight, last_used_at
FROM cmd_palette_query_hits
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces)
ON CONFLICT (workspace_id, query, command_id) DO UPDATE SET
	weight = cmd_palette_query_hits.weight + excluded.weight,
	last_used_at = max(cmd_palette_query_hits.last_used_at, excluded.last_used_at);
DELETE FROM cmd_palette_query_hits
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);

INSERT INTO cmd_palette_pins (workspace_id, command_id, pinned_at)
SELECT '', command_id, pinned_at
FROM cmd_palette_pins
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces)
ON CONFLICT (workspace_id, command_id) DO UPDATE SET
	pinned_at = min(cmd_palette_pins.pinned_at, excluded.pinned_at);
DELETE FROM cmd_palette_pins
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);

INSERT INTO token_usage_daily (
	day, workspace_id, agent_name, input_tokens, output_tokens, total_tokens,
	total_cost, cost_currency, cost_status, cost_source, turn_count, updated_at
)
SELECT
	day, '', agent_name, input_tokens, output_tokens, total_tokens,
	total_cost, cost_currency, cost_status, cost_source, turn_count, updated_at
FROM token_usage_daily
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces)
ON CONFLICT (day, workspace_id, agent_name) DO UPDATE SET
	input_tokens = token_usage_daily.input_tokens + excluded.input_tokens,
	output_tokens = token_usage_daily.output_tokens + excluded.output_tokens,
	total_tokens = token_usage_daily.total_tokens + excluded.total_tokens,
	total_cost = CASE
		WHEN token_usage_daily.total_cost IS NULL AND excluded.total_cost IS NULL THEN NULL
		ELSE coalesce(token_usage_daily.total_cost, 0) + coalesce(excluded.total_cost, 0)
	END,
	cost_currency = coalesce(token_usage_daily.cost_currency, excluded.cost_currency),
	cost_status = CASE
		WHEN excluded.updated_at >= token_usage_daily.updated_at THEN excluded.cost_status
		ELSE token_usage_daily.cost_status
	END,
	cost_source = CASE
		WHEN excluded.updated_at >= token_usage_daily.updated_at THEN excluded.cost_source
		ELSE token_usage_daily.cost_source
	END,
	turn_count = token_usage_daily.turn_count + excluded.turn_count,
	updated_at = max(token_usage_daily.updated_at, excluded.updated_at);
DELETE FROM token_usage_daily
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);

INSERT INTO notification_cursors (
	scope_kind, workspace_id, consumer_id, stream_name, subject_id,
	last_sequence, last_delivery_id, last_delivered_at, last_error, updated_at
)
SELECT
	'global', '', consumer_id, stream_name, subject_id,
	last_sequence, last_delivery_id, last_delivered_at, last_error, updated_at
FROM notification_cursors
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces)
ON CONFLICT (scope_kind, workspace_id, consumer_id, stream_name, subject_id) DO UPDATE SET
	last_sequence = max(notification_cursors.last_sequence, excluded.last_sequence),
	last_delivery_id = CASE
		WHEN excluded.last_sequence >= notification_cursors.last_sequence THEN excluded.last_delivery_id
		ELSE notification_cursors.last_delivery_id
	END,
	last_delivered_at = CASE
		WHEN excluded.last_sequence >= notification_cursors.last_sequence THEN excluded.last_delivered_at
		ELSE notification_cursors.last_delivered_at
	END,
	last_error = CASE
		WHEN excluded.updated_at >= notification_cursors.updated_at THEN excluded.last_error
		ELSE notification_cursors.last_error
	END,
	updated_at = max(notification_cursors.updated_at, excluded.updated_at);
DELETE FROM notification_cursors
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);

-- Shape 1: nullable workspace ownership.
UPDATE tasks
SET scope = 'global', workspace_id = NULL
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE task_blocks
SET workspace_id = NULL
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE task_runs
SET workspace_id = NULL
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE task_run_terminal_commands
SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);

UPDATE automation_jobs
SET scope = 'global', workspace_id = NULL
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE automation_jobs
SET loop_workspace_id = NULL
WHERE loop_workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE automation_triggers
SET scope = 'global', workspace_id = NULL
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE automation_triggers
SET loop_workspace_id = NULL
WHERE loop_workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE automation_suggestions
SET workspace_id = NULL
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE automation_job_catalog_entries
SET scope = 'global', workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE automation_trigger_catalog_entries
SET scope = 'global', workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE automation_watch_events
SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);

UPDATE bridge_instances
SET scope = 'global', workspace_id = NULL
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE bridge_routes
SET scope = 'global', workspace_id = NULL
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE bridge_task_subscriptions
SET scope = 'global', workspace_id = NULL
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE bridge_deliveries
SET scope = 'global', workspace_id = NULL
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE bridge_delivery_metrics
SET scope = 'global', workspace_id = NULL
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE tool_approval_pending
SET workspace_id = NULL
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);

-- Shape 2: sessions carry an explicit scope discriminator.
UPDATE sessions
SET scope = 'global', workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE session_prompt_admissions
SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE session_health
SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);

-- Shape 3: identity-bearing rows retain a non-null aggregate key.
UPDATE agent_heartbeat_revisions SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE agent_heartbeat_snapshots SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE agent_heartbeat_wake_events SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE agent_heartbeat_wake_state SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE agent_soul_revisions SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE agent_soul_snapshots SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE tool_approval_grants SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE dead_entities SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE event_summaries SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);

UPDATE loop_config SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE loop_definition_snapshots SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE loop_gate_decisions SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE loop_requests SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE loop_timetravel_ops SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE loop_node_amendments SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE loop_node_lane_pauses SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE loop_admission_claims SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE loop_goal_session_cleanup SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE loop_goal_session_outbox SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE loop_run_events SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE loop_runs SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE loop_session_bindings SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE loop_ui_annotations SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);

UPDATE network_audit_log SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE network_channel_kind_counts SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE network_channel_participants SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE network_channel_stats SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE network_channels SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE network_direct_rooms SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE network_subscriptions SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE network_task_thread_origins SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE network_task_status_projections SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE network_thread_participants SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE network_thread_session_token_stats SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE network_threads SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE network_timeline_log SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE network_work SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE task_network_coordination SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE network_coordination_invitations
SET workspace_id = '', scope_id = CASE WHEN scope_kind = 'workspace' THEN '' ELSE scope_id END
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE network_message_dispositions SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE network_live_wakes SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE network_wake_sources SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE network_participation_budgets SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
UPDATE network_wake_events SET workspace_id = ''
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);

-- Scope-bearing denormalized rows use their existing aggregate discriminator.
INSERT INTO mcp_auth_tokens (
	scope, workspace_id, server_name, definition_fingerprint, issuer, client_id,
	scopes_json, access_token_ref, refresh_token_ref, token_type, expires_at,
	obtained_at, updated_at
)
SELECT
	'global', '', server_name, definition_fingerprint, issuer, client_id,
	scopes_json, access_token_ref, refresh_token_ref, token_type, expires_at,
	obtained_at, updated_at
FROM mcp_auth_tokens
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces)
ON CONFLICT (scope, workspace_id, server_name) DO UPDATE SET
	definition_fingerprint = excluded.definition_fingerprint,
	issuer = excluded.issuer,
	client_id = excluded.client_id,
	scopes_json = excluded.scopes_json,
	access_token_ref = excluded.access_token_ref,
	refresh_token_ref = excluded.refresh_token_ref,
	token_type = excluded.token_type,
	expires_at = excluded.expires_at,
	obtained_at = excluded.obtained_at,
	updated_at = excluded.updated_at
WHERE excluded.updated_at >= mcp_auth_tokens.updated_at;
DELETE FROM mcp_auth_tokens
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);

INSERT INTO mcp_oauth_registrations (
	scope, workspace_id, server_name, definition_fingerprint, resource_url, issuer,
	client_id, token_endpoint_auth_method, client_secret_ref,
	registration_access_token_ref, registration_client_uri, client_id_issued_at,
	client_secret_expires_at, redirect_uri, scopes_json, updated_at
)
SELECT
	'global', '', server_name, definition_fingerprint, resource_url, issuer,
	client_id, token_endpoint_auth_method, client_secret_ref,
	registration_access_token_ref, registration_client_uri, client_id_issued_at,
	client_secret_expires_at, redirect_uri, scopes_json, updated_at
FROM mcp_oauth_registrations
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces)
ON CONFLICT (scope, workspace_id, server_name) DO UPDATE SET
	definition_fingerprint = excluded.definition_fingerprint,
	resource_url = excluded.resource_url,
	issuer = excluded.issuer,
	client_id = excluded.client_id,
	token_endpoint_auth_method = excluded.token_endpoint_auth_method,
	client_secret_ref = excluded.client_secret_ref,
	registration_access_token_ref = excluded.registration_access_token_ref,
	registration_client_uri = excluded.registration_client_uri,
	client_id_issued_at = excluded.client_id_issued_at,
	client_secret_expires_at = excluded.client_secret_expires_at,
	redirect_uri = excluded.redirect_uri,
	scopes_json = excluded.scopes_json,
	updated_at = excluded.updated_at
WHERE excluded.updated_at >= mcp_oauth_registrations.updated_at;
DELETE FROM mcp_oauth_registrations
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);

UPDATE gateway_ingress_bindings
SET scope_kind = 'global', workspace_id = NULL
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);

-- Rows with no meaning outside a registered workspace are removed.
DELETE FROM workspace_network_coordination
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
DELETE FROM extension_env_bindings
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);
DELETE FROM extension_dev_links
WHERE workspace_id IN (SELECT id FROM phase0_home_workspaces);

DELETE FROM workspaces
WHERE id IN (SELECT id FROM phase0_home_workspaces);
DROP TABLE phase0_home_workspaces;
DROP TABLE phase0_operator_home_context;
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
-- recreate trigger "agent_heartbeat_revisions_workspace_insert_guard" after rebuilding table "agent_heartbeat_revisions"
-- +goose StatementBegin
CREATE TRIGGER agent_heartbeat_revisions_workspace_insert_guard
BEFORE INSERT ON agent_heartbeat_revisions
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "agent_heartbeat_revisions_workspace_update_guard" after rebuilding table "agent_heartbeat_revisions"
-- +goose StatementBegin
CREATE TRIGGER agent_heartbeat_revisions_workspace_update_guard
BEFORE UPDATE OF workspace_id ON agent_heartbeat_revisions
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "agent_heartbeat_snapshots_workspace_insert_guard" after rebuilding table "agent_heartbeat_snapshots"
-- +goose StatementBegin
CREATE TRIGGER agent_heartbeat_snapshots_workspace_insert_guard
BEFORE INSERT ON agent_heartbeat_snapshots
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "agent_heartbeat_snapshots_workspace_update_guard" after rebuilding table "agent_heartbeat_snapshots"
-- +goose StatementBegin
CREATE TRIGGER agent_heartbeat_snapshots_workspace_update_guard
BEFORE UPDATE OF workspace_id ON agent_heartbeat_snapshots
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "agent_heartbeat_wake_events_workspace_insert_guard" after rebuilding table "agent_heartbeat_wake_events"
-- +goose StatementBegin
CREATE TRIGGER agent_heartbeat_wake_events_workspace_insert_guard
BEFORE INSERT ON agent_heartbeat_wake_events
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "agent_heartbeat_wake_events_workspace_update_guard" after rebuilding table "agent_heartbeat_wake_events"
-- +goose StatementBegin
CREATE TRIGGER agent_heartbeat_wake_events_workspace_update_guard
BEFORE UPDATE OF workspace_id ON agent_heartbeat_wake_events
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "agent_heartbeat_wake_state_workspace_insert_guard" after rebuilding table "agent_heartbeat_wake_state"
-- +goose StatementBegin
CREATE TRIGGER agent_heartbeat_wake_state_workspace_insert_guard
BEFORE INSERT ON agent_heartbeat_wake_state
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "agent_heartbeat_wake_state_workspace_update_guard" after rebuilding table "agent_heartbeat_wake_state"
-- +goose StatementBegin
CREATE TRIGGER agent_heartbeat_wake_state_workspace_update_guard
BEFORE UPDATE OF workspace_id ON agent_heartbeat_wake_state
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "agent_soul_revisions_workspace_insert_guard" after rebuilding table "agent_soul_revisions"
-- +goose StatementBegin
CREATE TRIGGER agent_soul_revisions_workspace_insert_guard
BEFORE INSERT ON agent_soul_revisions
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "agent_soul_revisions_workspace_update_guard" after rebuilding table "agent_soul_revisions"
-- +goose StatementBegin
CREATE TRIGGER agent_soul_revisions_workspace_update_guard
BEFORE UPDATE OF workspace_id ON agent_soul_revisions
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "agent_soul_snapshots_workspace_insert_guard" after rebuilding table "agent_soul_snapshots"
-- +goose StatementBegin
CREATE TRIGGER agent_soul_snapshots_workspace_insert_guard
BEFORE INSERT ON agent_soul_snapshots
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "agent_soul_snapshots_workspace_update_guard" after rebuilding table "agent_soul_snapshots"
-- +goose StatementBegin
CREATE TRIGGER agent_soul_snapshots_workspace_update_guard
BEFORE UPDATE OF workspace_id ON agent_soul_snapshots
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
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
-- recreate trigger "network_coordination_invitations_workspace_insert_guard" after rebuilding table "network_coordination_invitations"
-- +goose StatementBegin
CREATE TRIGGER network_coordination_invitations_workspace_insert_guard
BEFORE INSERT ON network_coordination_invitations
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "network_coordination_invitations_workspace_update_guard" after rebuilding table "network_coordination_invitations"
-- +goose StatementBegin
CREATE TRIGGER network_coordination_invitations_workspace_update_guard
BEFORE UPDATE OF workspace_id ON network_coordination_invitations
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "network_live_wakes_workspace_insert_guard" after rebuilding table "network_live_wakes"
-- +goose StatementBegin
CREATE TRIGGER network_live_wakes_workspace_insert_guard
BEFORE INSERT ON network_live_wakes
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "network_live_wakes_workspace_update_guard" after rebuilding table "network_live_wakes"
-- +goose StatementBegin
CREATE TRIGGER network_live_wakes_workspace_update_guard
BEFORE UPDATE OF workspace_id ON network_live_wakes
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "network_message_dispositions_workspace_insert_guard" after rebuilding table "network_message_dispositions"
-- +goose StatementBegin
CREATE TRIGGER network_message_dispositions_workspace_insert_guard
BEFORE INSERT ON network_message_dispositions
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "network_message_dispositions_workspace_update_guard" after rebuilding table "network_message_dispositions"
-- +goose StatementBegin
CREATE TRIGGER network_message_dispositions_workspace_update_guard
BEFORE UPDATE OF workspace_id ON network_message_dispositions
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "network_participation_budgets_workspace_insert_guard" after rebuilding table "network_participation_budgets"
-- +goose StatementBegin
CREATE TRIGGER network_participation_budgets_workspace_insert_guard
BEFORE INSERT ON network_participation_budgets
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "network_participation_budgets_workspace_update_guard" after rebuilding table "network_participation_budgets"
-- +goose StatementBegin
CREATE TRIGGER network_participation_budgets_workspace_update_guard
BEFORE UPDATE OF workspace_id ON network_participation_budgets
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "network_wake_events_workspace_insert_guard" after rebuilding table "network_wake_events"
-- +goose StatementBegin
CREATE TRIGGER network_wake_events_workspace_insert_guard
BEFORE INSERT ON network_wake_events
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "network_wake_events_workspace_update_guard" after rebuilding table "network_wake_events"
-- +goose StatementBegin
CREATE TRIGGER network_wake_events_workspace_update_guard
BEFORE UPDATE OF workspace_id ON network_wake_events
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "network_wake_sources_workspace_insert_guard" after rebuilding table "network_wake_sources"
-- +goose StatementBegin
CREATE TRIGGER network_wake_sources_workspace_insert_guard
BEFORE INSERT ON network_wake_sources
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "network_wake_sources_workspace_update_guard" after rebuilding table "network_wake_sources"
-- +goose StatementBegin
CREATE TRIGGER network_wake_sources_workspace_update_guard
BEFORE UPDATE OF workspace_id ON network_wake_sources
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
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
-- recreate trigger "session_health_workspace_insert_guard" after rebuilding table "session_health"
-- +goose StatementBegin
CREATE TRIGGER session_health_workspace_insert_guard
BEFORE INSERT ON session_health
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "session_health_workspace_update_guard" after rebuilding table "session_health"
-- +goose StatementBegin
CREATE TRIGGER session_health_workspace_update_guard
BEFORE UPDATE OF workspace_id ON session_health
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
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
-- recreate trigger "task_network_coordination_workspace_insert_guard" after rebuilding table "task_network_coordination"
-- +goose StatementBegin
CREATE TRIGGER task_network_coordination_workspace_insert_guard
BEFORE INSERT ON task_network_coordination
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "task_network_coordination_workspace_update_guard" after rebuilding table "task_network_coordination"
-- +goose StatementBegin
CREATE TRIGGER task_network_coordination_workspace_update_guard
BEFORE UPDATE OF workspace_id ON task_network_coordination
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
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
-- recreate trigger "trg_task_runs_terminal_command_delete_guard" after rebuilding table "task_runs"
-- +goose StatementBegin
CREATE TRIGGER trg_task_runs_terminal_command_delete_guard
	BEFORE DELETE ON task_runs
	WHEN EXISTS (
		SELECT 1
		FROM task_run_terminal_commands
		WHERE run_id = OLD.id
	)
	BEGIN
		SELECT RAISE(ABORT, 'task run terminal command in progress');
	END;
-- +goose StatementEnd
-- recreate trigger "trg_task_runs_terminal_command_guard" after rebuilding table "task_runs"
-- +goose StatementBegin
CREATE TRIGGER trg_task_runs_terminal_command_guard
	BEFORE UPDATE ON task_runs
	WHEN EXISTS (
		SELECT 1
		FROM task_run_terminal_commands
		WHERE run_id = OLD.id
	)
	BEGIN
		SELECT RAISE(ABORT, 'task run terminal command in progress');
	END;
-- +goose StatementEnd
