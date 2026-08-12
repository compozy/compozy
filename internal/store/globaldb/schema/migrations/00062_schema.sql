-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- add column "worktree_id" to table: "event_summaries"
ALTER TABLE `event_summaries` ADD COLUMN `worktree_id` text NOT NULL DEFAULT '';
-- create index "idx_summaries_worktree" to table: "event_summaries"
CREATE INDEX `idx_summaries_worktree` ON `event_summaries` (`worktree_id`);
-- create "new_sessions" table
CREATE TABLE `new_sessions` (`id` text NULL, `name` text NULL, `agent_name` text NOT NULL, `provider` text NOT NULL DEFAULT '', `model` text NOT NULL DEFAULT '', `reasoning_effort` text NOT NULL DEFAULT '', `speed` text NOT NULL DEFAULT '', `speed_resolution_json` text NOT NULL DEFAULT '', `runtime_status` text NOT NULL DEFAULT 'unbound', `runtime_transition` text NOT NULL DEFAULT '', `runtime_failure` text NOT NULL DEFAULT '', `selected_provider` text NOT NULL DEFAULT '', `selected_model` text NOT NULL DEFAULT '', `selected_reasoning_effort` text NOT NULL DEFAULT '', `selected_speed` text NOT NULL DEFAULT '', `runtime_selection_revision` integer NOT NULL DEFAULT 0, `workspace_id` text NOT NULL, `worktree_id` text NULL, `session_type` text NOT NULL DEFAULT 'user', `state` text NOT NULL, `archived_at` text NULL, `acp_session_id` text NULL, `stop_reason` text NULL, `stop_detail` text NULL, `subprocess_pid` integer NOT NULL DEFAULT 0, `subprocess_started_at` text NULL, `last_update_at` text NULL, `stall_state` text NOT NULL DEFAULT '', `stall_reason` text NOT NULL DEFAULT '', `activity_json` text NOT NULL DEFAULT '', `attached_to` text NOT NULL DEFAULT '', `attach_expires_at` text NULL, `transcript_epoch` integer NOT NULL DEFAULT 0, `sandbox_id` text NOT NULL DEFAULT '', `sandbox_backend` text NOT NULL DEFAULT 'local', `sandbox_profile` text NOT NULL DEFAULT '', `sandbox_instance_id` text NOT NULL DEFAULT '', `sandbox_state` text NOT NULL DEFAULT '', `sandbox_provider_state_json` text NOT NULL DEFAULT '', `sandbox_last_sync_at` text NULL, `sandbox_last_sync_error` text NOT NULL DEFAULT '', `created_at` text NOT NULL, `updated_at` text NOT NULL, `failure_kind` text NULL, `failure_summary` text NOT NULL DEFAULT '', `crash_bundle_path` text NOT NULL DEFAULT '', `parent_session_id` text NULL, `root_session_id` text NULL, `spawn_depth` integer NOT NULL DEFAULT 0, `spawn_role` text NULL, `ttl_expires_at` text NULL, `auto_stop_on_parent` boolean NOT NULL DEFAULT 0, `spawn_budget_json` text NOT NULL DEFAULT '{}', `permission_policy_json` text NOT NULL DEFAULT '{}', `soul_snapshot_id` text NULL, `soul_digest` text NOT NULL DEFAULT '', `parent_soul_digest` text NOT NULL DEFAULT '', `input_generation` integer NOT NULL DEFAULT 0, `creation_digest` text NULL, `policy_spec_digest` text NULL, `creation_profile_ref` text NULL, `network_spec_json` text NOT NULL DEFAULT '{"version":"network-participation/v1","mode":"local","source":"built_in_local"}', `network_mode` text NOT NULL DEFAULT 'local', `network_channel` text NULL, `network_source` text NOT NULL DEFAULT 'built_in_local', PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`, `worktree_id`) REFERENCES `worktrees` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `1` FOREIGN KEY (`soul_snapshot_id`) REFERENCES `agent_soul_snapshots` (`id`) ON UPDATE NO ACTION ON DELETE SET NULL, CONSTRAINT `2` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (creation_digest IS NULL OR length(trim(creation_digest)) > 0), CHECK (policy_spec_digest IS NULL OR length(trim(policy_spec_digest)) > 0), CHECK (creation_profile_ref IS NULL OR length(trim(creation_profile_ref)) > 0), CHECK (json_valid(network_spec_json)), CHECK (network_mode IN ('local', 'live')), CHECK (network_source IN (
					'explicit_request', 'task_profile', 'workspace_coordination',
					'loop_definition', 'automation_job', 'built_in_local'
				)));
-- copy rows from old table "sessions" to new temporary table "new_sessions"
INSERT INTO `new_sessions` (`id`, `name`, `agent_name`, `provider`, `model`, `reasoning_effort`, `speed`, `speed_resolution_json`, `runtime_status`, `runtime_transition`, `runtime_failure`, `selected_provider`, `selected_model`, `selected_reasoning_effort`, `selected_speed`, `runtime_selection_revision`, `workspace_id`, `session_type`, `state`, `archived_at`, `acp_session_id`, `stop_reason`, `stop_detail`, `subprocess_pid`, `subprocess_started_at`, `last_update_at`, `stall_state`, `stall_reason`, `activity_json`, `attached_to`, `attach_expires_at`, `transcript_epoch`, `sandbox_id`, `sandbox_backend`, `sandbox_profile`, `sandbox_instance_id`, `sandbox_state`, `sandbox_provider_state_json`, `sandbox_last_sync_at`, `sandbox_last_sync_error`, `created_at`, `updated_at`, `failure_kind`, `failure_summary`, `crash_bundle_path`, `parent_session_id`, `root_session_id`, `spawn_depth`, `spawn_role`, `ttl_expires_at`, `auto_stop_on_parent`, `spawn_budget_json`, `permission_policy_json`, `soul_snapshot_id`, `soul_digest`, `parent_soul_digest`, `input_generation`, `creation_digest`, `policy_spec_digest`, `creation_profile_ref`, `network_spec_json`, `network_mode`, `network_channel`, `network_source`) SELECT `id`, `name`, `agent_name`, `provider`, `model`, `reasoning_effort`, `speed`, `speed_resolution_json`, `runtime_status`, `runtime_transition`, `runtime_failure`, `selected_provider`, `selected_model`, `selected_reasoning_effort`, `selected_speed`, `runtime_selection_revision`, `workspace_id`, `session_type`, `state`, `archived_at`, `acp_session_id`, `stop_reason`, `stop_detail`, `subprocess_pid`, `subprocess_started_at`, `last_update_at`, `stall_state`, `stall_reason`, `activity_json`, `attached_to`, `attach_expires_at`, `transcript_epoch`, `sandbox_id`, `sandbox_backend`, `sandbox_profile`, `sandbox_instance_id`, `sandbox_state`, `sandbox_provider_state_json`, `sandbox_last_sync_at`, `sandbox_last_sync_error`, `created_at`, `updated_at`, `failure_kind`, `failure_summary`, `crash_bundle_path`, `parent_session_id`, `root_session_id`, `spawn_depth`, `spawn_role`, `ttl_expires_at`, `auto_stop_on_parent`, `spawn_budget_json`, `permission_policy_json`, `soul_snapshot_id`, `soul_digest`, `parent_soul_digest`, `input_generation`, `creation_digest`, `policy_spec_digest`, `creation_profile_ref`, `network_spec_json`, `network_mode`, `network_channel`, `network_source` FROM `sessions`;
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
-- create "new_task_execution_profiles" table
CREATE TABLE `new_task_execution_profiles` (`task_id` text NULL, `coordinator_mode` text NOT NULL DEFAULT 'inherit', `coordinator_agent_name` text NOT NULL DEFAULT '', `coordinator_provider` text NOT NULL DEFAULT '', `coordinator_model` text NOT NULL DEFAULT '', `coordinator_guidance` text NOT NULL DEFAULT '', `worker_mode` text NOT NULL DEFAULT 'inherit', `worker_agent_name` text NOT NULL DEFAULT '', `worker_provider` text NOT NULL DEFAULT '', `worker_model` text NOT NULL DEFAULT '', `review_agent_name` text NOT NULL DEFAULT '', `review_provider` text NOT NULL DEFAULT '', `review_model` text NOT NULL DEFAULT '', `sandbox_mode` text NOT NULL DEFAULT 'inherit', `sandbox_ref` text NOT NULL DEFAULT '', `worktree_mode` text NOT NULL DEFAULT 'inherit', `worktree_ref` text NOT NULL DEFAULT '', `created_at` text NOT NULL, `updated_at` text NOT NULL, `runtime_mode` text NOT NULL DEFAULT 'default', `network_mode` text NOT NULL DEFAULT '', `network_channel_strategy` text NULL, `network_channel` text NULL, `network_bounds_json` text NULL, PRIMARY KEY (`task_id`), CONSTRAINT `0` FOREIGN KEY (`task_id`) REFERENCES `tasks` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (
				coordinator_mode IN ('inherit', 'guided')
			), CHECK (
				worker_mode IN ('inherit', 'select')
			), CHECK (
				sandbox_mode IN ('inherit', 'none', 'ref')
			), CHECK (
				worktree_mode IN ('inherit', 'none', 'ref', 'per_run')
			), CHECK (
				runtime_mode IN ('default', 'evidence')
			), CHECK (network_mode IN ('', 'local', 'live')), CHECK (
				network_channel_strategy IS NULL OR network_channel_strategy IN ('named', 'run', 'loop_run')
			), CHECK (
				network_bounds_json IS NULL OR json_valid(network_bounds_json)
			), CHECK (
				(sandbox_mode = 'ref' AND sandbox_ref <> '') OR
				(sandbox_mode <> 'ref' AND sandbox_ref = '')
			), CHECK (
				(worktree_mode = 'ref') = (worktree_ref <> '')
			));
-- copy rows from old table "task_execution_profiles" to new temporary table "new_task_execution_profiles"
INSERT INTO `new_task_execution_profiles` (`task_id`, `coordinator_mode`, `coordinator_agent_name`, `coordinator_provider`, `coordinator_model`, `coordinator_guidance`, `worker_mode`, `worker_agent_name`, `worker_provider`, `worker_model`, `review_agent_name`, `review_provider`, `review_model`, `sandbox_mode`, `sandbox_ref`, `created_at`, `updated_at`, `runtime_mode`, `network_mode`, `network_channel_strategy`, `network_channel`, `network_bounds_json`) SELECT `task_id`, `coordinator_mode`, `coordinator_agent_name`, `coordinator_provider`, `coordinator_model`, `coordinator_guidance`, `worker_mode`, `worker_agent_name`, `worker_provider`, `worker_model`, `review_agent_name`, `review_provider`, `review_model`, `sandbox_mode`, `sandbox_ref`, `created_at`, `updated_at`, `runtime_mode`, `network_mode`, `network_channel_strategy`, `network_channel`, `network_bounds_json` FROM `task_execution_profiles`;
-- drop "task_execution_profiles" table after copying rows
DROP TABLE `task_execution_profiles`;
-- rename temporary table "new_task_execution_profiles" to "task_execution_profiles"
ALTER TABLE `new_task_execution_profiles` RENAME TO `task_execution_profiles`;
-- create index "task_execution_profiles_task_id_idx" to table: "task_execution_profiles"
CREATE INDEX `task_execution_profiles_task_id_idx` ON `task_execution_profiles` (`task_id`);
-- create "new_task_runs" table
CREATE TABLE `new_task_runs` (`id` text NULL, `task_id` text NULL, `workspace_id` text NULL, `worktree_id` text NULL, `status` text NOT NULL, `attempt` integer NOT NULL, `recovery_count` integer NOT NULL DEFAULT 0, `previous_run_id` text NULL, `failure_kind` text NOT NULL DEFAULT '', `claimed_by_kind` text NULL, `claimed_by_ref` text NULL, `session_id` text NULL, `origin_kind` text NOT NULL, `origin_ref` text NOT NULL, `idempotency_key` text NULL, `network_spec_json` text NOT NULL DEFAULT '{"version":"network-participation/v1","mode":"local","source":"built_in_local"}', `network_mode` text NOT NULL DEFAULT 'local', `network_channel` text NULL, `network_source` text NOT NULL DEFAULT 'built_in_local', `designation_group_id` text NOT NULL DEFAULT '', `resolved_worktree_mode` text NOT NULL DEFAULT '', `resolved_worktree_ref` text NOT NULL DEFAULT '', `queued_at` text NOT NULL, `claimed_at` text NULL, `started_at` text NULL, `ended_at` text NULL, `error` text NULL, `metadata_json` text NULL, `result_json` text NULL, `summary` text NOT NULL DEFAULT '', `claimed_agent_name` text NOT NULL DEFAULT '', `claimed_peer_id` text NOT NULL DEFAULT '', `terminalized_by_session_id` text NOT NULL DEFAULT '', `terminalized_by_agent_name` text NOT NULL DEFAULT '', `terminalized_by_peer_id` text NOT NULL DEFAULT '', `terminalized_by_actor_kind` text NOT NULL DEFAULT '', `terminalized_by_actor_ref` text NOT NULL DEFAULT '', `review_required` boolean NOT NULL DEFAULT 0, `review_request_round` integer NOT NULL DEFAULT 0, `review_policy_snapshot` text NOT NULL DEFAULT '', `review_request_id` text NULL, `parent_run_id` text NULL, `review_id` text NULL, `review_round` integer NOT NULL DEFAULT 0, `continuation_reason` text NOT NULL DEFAULT '', `missing_work_json` text NOT NULL DEFAULT '[]', `next_round_guidance` text NOT NULL DEFAULT '', `claim_token` text NULL, `claim_token_hash` text NULL, `lease_until` text NULL, `heartbeat_at` text NULL, `run_kind` text NOT NULL DEFAULT 'worker', `loop_run_id` text NULL, `tokens_used` integer NOT NULL DEFAULT 0, `network_wake_id` text NULL, `network_target_session_id` text NULL, `network_owner_key` text NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`, `worktree_id`) REFERENCES `worktrees` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `1` FOREIGN KEY (`workspace_id`, `network_target_session_id`) REFERENCES `sessions` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `2` FOREIGN KEY (`review_id`) REFERENCES `task_run_reviews` (`review_id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `3` FOREIGN KEY (`parent_run_id`) REFERENCES `task_runs` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `4` FOREIGN KEY (`review_request_id`) REFERENCES `task_run_reviews` (`review_id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `5` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `6` FOREIGN KEY (`task_id`) REFERENCES `tasks` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (attempt > 0), CHECK (recovery_count >= 0), CHECK (
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
		), CHECK (status <> 'queued' OR session_id IS NULL), CHECK (run_kind = 'network_wake' OR task_id IS NOT NULL), CHECK (run_kind <> 'network_wake' OR task_id IS NULL), CHECK (run_kind <> 'network_wake' OR workspace_id IS NOT NULL), CHECK (
			(resolved_worktree_mode = 'ref') = (resolved_worktree_ref <> '')
		), CHECK (
			(run_kind = 'network_wake' AND network_wake_id IS NOT NULL
				AND network_target_session_id IS NOT NULL AND network_owner_key IS NOT NULL) OR
			(run_kind <> 'network_wake' AND network_wake_id IS NULL
				AND network_target_session_id IS NULL AND network_owner_key IS NULL)
		));
-- copy rows from old table "task_runs" to new temporary table "new_task_runs"
INSERT INTO `new_task_runs` (`id`, `task_id`, `workspace_id`, `status`, `attempt`, `recovery_count`, `previous_run_id`, `failure_kind`, `claimed_by_kind`, `claimed_by_ref`, `session_id`, `origin_kind`, `origin_ref`, `idempotency_key`, `network_spec_json`, `network_mode`, `network_channel`, `network_source`, `designation_group_id`, `queued_at`, `claimed_at`, `started_at`, `ended_at`, `error`, `metadata_json`, `result_json`, `summary`, `claimed_agent_name`, `claimed_peer_id`, `terminalized_by_session_id`, `terminalized_by_agent_name`, `terminalized_by_peer_id`, `terminalized_by_actor_kind`, `terminalized_by_actor_ref`, `review_required`, `review_request_round`, `review_policy_snapshot`, `review_request_id`, `parent_run_id`, `review_id`, `review_round`, `continuation_reason`, `missing_work_json`, `next_round_guidance`, `claim_token`, `claim_token_hash`, `lease_until`, `heartbeat_at`, `run_kind`, `loop_run_id`, `tokens_used`, `network_wake_id`, `network_target_session_id`, `network_owner_key`) SELECT `id`, `task_id`, `workspace_id`, `status`, `attempt`, `recovery_count`, `previous_run_id`, `failure_kind`, `claimed_by_kind`, `claimed_by_ref`, `session_id`, `origin_kind`, `origin_ref`, `idempotency_key`, `network_spec_json`, `network_mode`, `network_channel`, `network_source`, `designation_group_id`, `queued_at`, `claimed_at`, `started_at`, `ended_at`, `error`, `metadata_json`, `result_json`, `summary`, `claimed_agent_name`, `claimed_peer_id`, `terminalized_by_session_id`, `terminalized_by_agent_name`, `terminalized_by_peer_id`, `terminalized_by_actor_kind`, `terminalized_by_actor_ref`, `review_required`, `review_request_round`, `review_policy_snapshot`, `review_request_id`, `parent_run_id`, `review_id`, `review_round`, `continuation_reason`, `missing_work_json`, `next_round_guidance`, `claim_token`, `claim_token_hash`, `lease_until`, `heartbeat_at`, `run_kind`, `loop_run_id`, `tokens_used`, `network_wake_id`, `network_target_session_id`, `network_owner_key` FROM `task_runs`;
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
-- create "worktrees" table
CREATE TABLE `worktrees` (`id` text NULL, `workspace_id` text NOT NULL, `name` text NOT NULL, `branch` text NOT NULL DEFAULT '', `path` text NOT NULL, `git_dir` text NOT NULL DEFAULT '', `state` text NOT NULL, `pending_phase` text NOT NULL DEFAULT '', `origin` text NOT NULL, `setup_state` text NOT NULL DEFAULT 'none', `setup_error` text NOT NULL DEFAULT '', `base_ref` text NOT NULL DEFAULT '', `created_branch` integer NOT NULL DEFAULT 0, `run_namespace` text NOT NULL DEFAULT '', `created_head` text NOT NULL DEFAULT '', `run_id` text NOT NULL DEFAULT '', `created_at` text NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (
			state IN ('pending', 'ready', 'failed', 'removing', 'missing', 'removed', 'dismissed')
		), CHECK (
			pending_phase IN ('', 'branch', 'checkout', 'copy', 'setup')
		), CHECK (origin IN ('manual', 'per_run', 'adopted')), CHECK (setup_state IN ('none', 'ok', 'failed')), CHECK (created_branch IN (0, 1)));
-- create index "worktrees_workspace_id_name" to table: "worktrees"
CREATE UNIQUE INDEX `worktrees_workspace_id_name` ON `worktrees` (`workspace_id`, `name`);
-- create index "worktrees_workspace_id_id" to table: "worktrees"
CREATE UNIQUE INDEX `worktrees_workspace_id_id` ON `worktrees` (`workspace_id`, `id`);
-- create index "idx_worktrees_workspace_state" to table: "worktrees"
CREATE INDEX `idx_worktrees_workspace_state` ON `worktrees` (`workspace_id`, `state`);
-- create index "idx_worktrees_live_path" to table: "worktrees"
CREATE UNIQUE INDEX `idx_worktrees_live_path` ON `worktrees` (`path`) WHERE state IN ('pending', 'ready', 'removing');
-- create "worktree_status" table
CREATE TABLE `worktree_status` (`worktree_id` text NULL, `branch` text NULL, `detached` integer NULL, `head_sha` text NULL, `dirty_files` integer NULL, `insertions` integer NULL, `deletions` integer NULL, `has_upstream` integer NULL, `ahead` integer NULL, `behind` integer NULL, `read_error` text NOT NULL DEFAULT '', `refreshed_at` text NULL, PRIMARY KEY (`worktree_id`), CONSTRAINT `0` FOREIGN KEY (`worktree_id`) REFERENCES `worktrees` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (detached IS NULL OR detached IN (0, 1)), CHECK (dirty_files IS NULL OR dirty_files >= 0), CHECK (insertions IS NULL OR insertions >= 0), CHECK (deletions IS NULL OR deletions >= 0), CHECK (has_upstream IS NULL OR has_upstream IN (0, 1)), CHECK (ahead IS NULL OR ahead >= 0), CHECK (behind IS NULL OR behind >= 0));
-- create "worktree_forge_status" table
CREATE TABLE `worktree_forge_status` (`worktree_id` text NULL, `provider` text NOT NULL DEFAULT '', `pr_number` integer NULL, `pr_state` text NULL, `pr_url` text NOT NULL DEFAULT '', `merged` integer NULL, `fetched_at` text NULL, PRIMARY KEY (`worktree_id`), CONSTRAINT `0` FOREIGN KEY (`worktree_id`) REFERENCES `worktrees` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (pr_number IS NULL OR pr_number > 0), CHECK (pr_state IS NULL OR pr_state IN ('open', 'closed', 'merged')), CHECK (merged IS NULL OR merged IN (0, 1)));
-- create "worktree_exit_ops" table
CREATE TABLE `worktree_exit_ops` (`op_id` text NULL, `workspace_id` text NOT NULL, `worktree_id` text NOT NULL, `action` text NOT NULL, `state` text NOT NULL, `started_at` text NOT NULL, `finished_at` text NULL, PRIMARY KEY (`op_id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`, `worktree_id`) REFERENCES `worktrees` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (action IN ('commit', 'commit_push', 'push', 'open_pr')), CHECK (state IN ('running', 'completed', 'failed', 'canceled')));
-- create index "idx_worktree_exit_ops_active" to table: "worktree_exit_ops"
CREATE UNIQUE INDEX `idx_worktree_exit_ops_active` ON `worktree_exit_ops` (`worktree_id`) WHERE state = 'running';
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
