-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_task_runs" table
CREATE TABLE `new_task_runs` (`id` text NULL, `task_id` text NULL, `workspace_id` text NULL, `worktree_id` text NULL, `status` text NOT NULL, `attempt` integer NOT NULL, `recovery_count` integer NOT NULL DEFAULT 0, `previous_run_id` text NULL, `failure_kind` text NOT NULL DEFAULT '', `claimed_by_kind` text NULL, `claimed_by_ref` text NULL, `session_id` text NULL, `origin_kind` text NOT NULL, `origin_ref` text NOT NULL, `idempotency_key` text NULL, `network_spec_json` text NOT NULL DEFAULT '{"version":"network-participation/v1","mode":"local","source":"built_in_local"}', `network_mode` text NOT NULL DEFAULT 'local', `network_channel` text NULL, `network_source` text NOT NULL DEFAULT 'built_in_local', `designation_group_id` text NOT NULL DEFAULT '', `resolved_worktree_mode` text NOT NULL DEFAULT '', `resolved_worktree_ref` text NOT NULL DEFAULT '', `queued_at` text NOT NULL, `claimed_at` text NULL, `started_at` text NULL, `ended_at` text NULL, `error` text NULL, `metadata_json` text NULL, `result_json` text NULL, `expect_digest` text NULL, `result_budget_bytes` integer NULL, `result_overflow` text NULL, `summary` text NOT NULL DEFAULT '', `claimed_agent_name` text NOT NULL DEFAULT '', `claimed_peer_id` text NOT NULL DEFAULT '', `terminalized_by_session_id` text NOT NULL DEFAULT '', `terminalized_by_agent_name` text NOT NULL DEFAULT '', `terminalized_by_peer_id` text NOT NULL DEFAULT '', `terminalized_by_actor_kind` text NOT NULL DEFAULT '', `terminalized_by_actor_ref` text NOT NULL DEFAULT '', `review_required` boolean NOT NULL DEFAULT 0, `review_request_round` integer NOT NULL DEFAULT 0, `review_policy_snapshot` text NOT NULL DEFAULT '', `review_request_id` text NULL, `parent_run_id` text NULL, `review_id` text NULL, `review_round` integer NOT NULL DEFAULT 0, `continuation_reason` text NOT NULL DEFAULT '', `missing_work_json` text NOT NULL DEFAULT '[]', `next_round_guidance` text NOT NULL DEFAULT '', `claim_token` text NULL, `claim_token_hash` text NULL, `lease_until` text NULL, `heartbeat_at` text NULL, `run_kind` text NOT NULL DEFAULT 'worker', `loop_run_id` text NULL, `tokens_used` integer NOT NULL DEFAULT 0, `network_wake_id` text NULL, `network_target_session_id` text NULL, `network_owner_key` text NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`, `worktree_id`) REFERENCES `worktrees` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `1` FOREIGN KEY (`network_target_session_id`) REFERENCES `sessions` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `2` FOREIGN KEY (`review_id`) REFERENCES `task_run_reviews` (`review_id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `3` FOREIGN KEY (`parent_run_id`) REFERENCES `task_runs` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `4` FOREIGN KEY (`review_request_id`) REFERENCES `task_run_reviews` (`review_id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `5` FOREIGN KEY (`expect_digest`) REFERENCES `contract_schemas` (`digest`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `6` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `7` FOREIGN KEY (`task_id`) REFERENCES `tasks` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (attempt > 0), CHECK (recovery_count >= 0), CHECK (
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
		), CHECK (result_budget_bytes IS NULL OR result_budget_bytes > 0), CHECK (result_overflow IS NULL OR result_overflow IN ('store', 'reject')), CHECK (review_required IN (0, 1)), CHECK (review_request_round >= 0), CHECK (
			review_policy_snapshot = '' OR
			review_policy_snapshot IN ('none', 'on_success', 'on_failure', 'always')
		), CHECK (review_round >= 0), CHECK (
			run_kind IN ('worker', 'coordinator', 'network_wake', 'call_activation')
		), CHECK (tokens_used >= 0), CHECK (
			(claimed_by_kind IS NULL AND claimed_by_ref IS NULL) OR
			(claimed_by_kind IS NOT NULL AND claimed_by_ref IS NOT NULL)
		), CHECK (status <> 'queued' OR session_id IS NULL), CHECK (run_kind IN ('network_wake', 'call_activation') OR task_id IS NOT NULL), CHECK (run_kind NOT IN ('network_wake', 'call_activation') OR task_id IS NULL), CHECK (
			(expect_digest IS NULL AND result_budget_bytes IS NULL AND result_overflow IS NULL) OR
			(expect_digest IS NOT NULL AND result_budget_bytes IS NOT NULL AND result_overflow IS NOT NULL)
		), CHECK (
			(resolved_worktree_mode = 'ref') = (resolved_worktree_ref <> '')
		), CHECK (
			(run_kind = 'network_wake' AND network_wake_id IS NOT NULL
				AND network_target_session_id IS NOT NULL AND network_owner_key IS NOT NULL) OR
			(run_kind <> 'network_wake' AND network_wake_id IS NULL
				AND network_target_session_id IS NULL AND network_owner_key IS NULL)
		));
-- copy rows from old table "task_runs" to new temporary table "new_task_runs"
INSERT INTO `new_task_runs` (`id`, `task_id`, `workspace_id`, `worktree_id`, `status`, `attempt`, `recovery_count`, `previous_run_id`, `failure_kind`, `claimed_by_kind`, `claimed_by_ref`, `session_id`, `origin_kind`, `origin_ref`, `idempotency_key`, `network_spec_json`, `network_mode`, `network_channel`, `network_source`, `designation_group_id`, `resolved_worktree_mode`, `resolved_worktree_ref`, `queued_at`, `claimed_at`, `started_at`, `ended_at`, `error`, `metadata_json`, `result_json`, `summary`, `claimed_agent_name`, `claimed_peer_id`, `terminalized_by_session_id`, `terminalized_by_agent_name`, `terminalized_by_peer_id`, `terminalized_by_actor_kind`, `terminalized_by_actor_ref`, `review_required`, `review_request_round`, `review_policy_snapshot`, `review_request_id`, `parent_run_id`, `review_id`, `review_round`, `continuation_reason`, `missing_work_json`, `next_round_guidance`, `claim_token`, `claim_token_hash`, `lease_until`, `heartbeat_at`, `run_kind`, `loop_run_id`, `tokens_used`, `network_wake_id`, `network_target_session_id`, `network_owner_key`) SELECT `id`, `task_id`, `workspace_id`, `worktree_id`, `status`, `attempt`, `recovery_count`, `previous_run_id`, `failure_kind`, `claimed_by_kind`, `claimed_by_ref`, `session_id`, `origin_kind`, `origin_ref`, `idempotency_key`, `network_spec_json`, `network_mode`, `network_channel`, `network_source`, `designation_group_id`, `resolved_worktree_mode`, `resolved_worktree_ref`, `queued_at`, `claimed_at`, `started_at`, `ended_at`, `error`, `metadata_json`, `result_json`, `summary`, `claimed_agent_name`, `claimed_peer_id`, `terminalized_by_session_id`, `terminalized_by_agent_name`, `terminalized_by_peer_id`, `terminalized_by_actor_kind`, `terminalized_by_actor_ref`, `review_required`, `review_request_round`, `review_policy_snapshot`, `review_request_id`, `parent_run_id`, `review_id`, `review_round`, `continuation_reason`, `missing_work_json`, `next_round_guidance`, `claim_token`, `claim_token_hash`, `lease_until`, `heartbeat_at`, `run_kind`, `loop_run_id`, `tokens_used`, `network_wake_id`, `network_target_session_id`, `network_owner_key` FROM `task_runs`;
-- drop trigger "tasks_profile_owner_active" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `tasks_profile_owner_active`;
-- drop trigger "tasks_profile_owner_immutable" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `tasks_profile_owner_immutable`;
-- drop trigger "trg_task_runs_terminal_command_delete_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `trg_task_runs_terminal_command_delete_guard`;
-- drop trigger "trg_task_runs_terminal_command_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `trg_task_runs_terminal_command_guard`;
-- drop trigger "trg_tasks_terminal_command_delete_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `trg_tasks_terminal_command_delete_guard`;
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
-- create "new_tasks" table
CREATE TABLE `new_tasks` (`id` text NULL, `profile_id` text NOT NULL, `identifier` text NULL, `scope` text NOT NULL, `workspace_id` text NULL, `parent_task_id` text NULL, `title` text NOT NULL, `description` text NULL, `priority` text NOT NULL DEFAULT 'medium', `max_attempts` integer NOT NULL DEFAULT 3, `status` text NOT NULL, `approval_policy` text NOT NULL DEFAULT 'none', `approval_state` text NOT NULL DEFAULT 'not_required', `owner_kind` text NULL, `owner_ref` text NULL, `created_by_kind` text NOT NULL, `created_by_ref` text NOT NULL, `origin_kind` text NOT NULL, `origin_ref` text NOT NULL, `created_at` text NOT NULL, `updated_at` text NOT NULL, `closed_at` text NULL, `metadata_json` text NULL, `current_run_id` text NULL, `paused` integer NOT NULL DEFAULT 0, `paused_by` text NOT NULL DEFAULT '', `paused_at` text NULL, `paused_reason` text NOT NULL DEFAULT '', `max_runtime_seconds` integer NOT NULL DEFAULT 0, `spawn_failure_count` integer NOT NULL DEFAULT 0, `last_spawn_error` text NOT NULL DEFAULT '', `review_policy` text NOT NULL DEFAULT 'none', `review_max_rounds` integer NOT NULL DEFAULT 3, `review_round` integer NOT NULL DEFAULT 0, `last_review_id` text NULL, `last_review_outcome` text NULL, `review_circuit_opened_at` text NULL, `review_circuit_reason` text NULL, `auto_enqueue_on_ready` integer NOT NULL DEFAULT 0, `needs_attention_reason` text NULL, `needs_attention_at` text NULL, `needs_attention_by_kind` text NULL, `needs_attention_by_ref` text NULL, `wake_creator` integer NOT NULL DEFAULT 1, `expect_digest` text NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`expect_digest`) REFERENCES `contract_schemas` (`digest`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `1` FOREIGN KEY (`current_run_id`) REFERENCES `task_runs` (`id`) ON UPDATE NO ACTION ON DELETE SET NULL, CONSTRAINT `2` FOREIGN KEY (`parent_task_id`) REFERENCES `tasks` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `3` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `4` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (scope IN ('global', 'workspace')), CHECK (
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
INSERT INTO `new_tasks` (`id`, `profile_id`, `identifier`, `scope`, `workspace_id`, `parent_task_id`, `title`, `description`, `priority`, `max_attempts`, `status`, `approval_policy`, `approval_state`, `owner_kind`, `owner_ref`, `created_by_kind`, `created_by_ref`, `origin_kind`, `origin_ref`, `created_at`, `updated_at`, `closed_at`, `metadata_json`, `current_run_id`, `paused`, `paused_by`, `paused_at`, `paused_reason`, `max_runtime_seconds`, `spawn_failure_count`, `last_spawn_error`, `review_policy`, `review_max_rounds`, `review_round`, `last_review_id`, `last_review_outcome`, `review_circuit_opened_at`, `review_circuit_reason`, `auto_enqueue_on_ready`, `needs_attention_reason`, `needs_attention_at`, `needs_attention_by_kind`, `needs_attention_by_ref`, `wake_creator`) SELECT `id`, `profile_id`, `identifier`, `scope`, `workspace_id`, `parent_task_id`, `title`, `description`, `priority`, `max_attempts`, `status`, `approval_policy`, `approval_state`, `owner_kind`, `owner_ref`, `created_by_kind`, `created_by_ref`, `origin_kind`, `origin_ref`, `created_at`, `updated_at`, `closed_at`, `metadata_json`, `current_run_id`, `paused`, `paused_by`, `paused_at`, `paused_reason`, `max_runtime_seconds`, `spawn_failure_count`, `last_spawn_error`, `review_policy`, `review_max_rounds`, `review_round`, `last_review_id`, `last_review_outcome`, `review_circuit_opened_at`, `review_circuit_reason`, `auto_enqueue_on_ready`, `needs_attention_reason`, `needs_attention_at`, `needs_attention_by_kind`, `needs_attention_by_ref`, `wake_creator` FROM `tasks`;
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
-- add column "parked_at" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `parked_at` text NULL;
-- add column "idle_expires_at" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `idle_expires_at` text NULL;
-- add column "draining_at" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `draining_at` text NULL;
-- create index "idx_sessions_idle_expiry" to table: "sessions"
CREATE INDEX `idx_sessions_idle_expiry` ON `sessions` (`idle_expires_at`) WHERE parked_at IS NOT NULL AND idle_expires_at IS NOT NULL;
-- create index "idx_sessions_draining" to table: "sessions"
CREATE INDEX `idx_sessions_draining` ON `sessions` (`draining_at`) WHERE draining_at IS NOT NULL;
-- create "contract_schemas" table
CREATE TABLE `contract_schemas` (`digest` text NULL, `schema` text NOT NULL, `created_at` text NOT NULL, PRIMARY KEY (`digest`), CHECK (digest GLOB 'sha256:[0-9a-f]*' AND length(digest) = 71), CHECK (json_valid(schema)));
-- create "payload_blobs" table
CREATE TABLE `payload_blobs` (`workspace_id` text NOT NULL DEFAULT '', `ref` text NOT NULL, `bytes` blob NOT NULL, `byte_size` integer NOT NULL, `created_at` text NOT NULL, `last_used_at` text NOT NULL, PRIMARY KEY (`workspace_id`, `ref`), CHECK (ref GLOB 'sha256:[0-9a-f]*' AND length(ref) = 71), CHECK (byte_size >= 0), CHECK (byte_size = length(bytes)));
-- create "calls" table
CREATE TABLE `calls` (`call_id` text NULL, `profile_id` text NOT NULL, `scope` text NOT NULL, `workspace_id` text NOT NULL DEFAULT '', `caller_kind` text NOT NULL, `caller_id` text NOT NULL, `actor_kind` text NOT NULL, `actor_id` text NOT NULL, `activation_run_id` text NULL, `parent_session_id` text NULL, `agent_name` text NULL, `child_session_id` text NULL, `governed_root_id` text NOT NULL, `depth` integer NOT NULL, `state` text NOT NULL, `verdict` text NULL, `expect_digest` text NULL, `prompt_ref` text NOT NULL, `result_ref` text NULL, `result_bytes` integer NULL, `result_budget_bytes` integer NOT NULL, `result_overflow` text NOT NULL, `strict` integer NOT NULL DEFAULT 0, `idle_ttl_seconds` integer NOT NULL, `runtime_provider` text NOT NULL DEFAULT '', `runtime_model` text NOT NULL DEFAULT '', `runtime_reasoning_effort` text NOT NULL DEFAULT '', `runtime_speed` text NOT NULL DEFAULT '', `failure_code` text NULL, `failure_detail` text NULL, `repair_attempts` integer NOT NULL DEFAULT 0, `first_issue_text` text NOT NULL DEFAULT '', `second_issue_text` text NOT NULL DEFAULT '', `final_prose_preview` text NOT NULL DEFAULT '', `superseded_ref` text NULL, `idempotency_key` text NULL, `request_digest` text NOT NULL, `batch_id` text NULL, `deadline_at` text NULL, `created_at` text NOT NULL, `started_at` text NULL, `settled_at` text NULL, `updated_at` text NOT NULL, PRIMARY KEY (`call_id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`, `superseded_ref`) REFERENCES `payload_blobs` (`workspace_id`, `ref`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `1` FOREIGN KEY (`workspace_id`, `result_ref`) REFERENCES `payload_blobs` (`workspace_id`, `ref`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `2` FOREIGN KEY (`workspace_id`, `prompt_ref`) REFERENCES `payload_blobs` (`workspace_id`, `ref`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `3` FOREIGN KEY (`expect_digest`) REFERENCES `contract_schemas` (`digest`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `4` FOREIGN KEY (`child_session_id`) REFERENCES `sessions` (`id`) ON UPDATE NO ACTION ON DELETE SET NULL, CONSTRAINT `5` FOREIGN KEY (`parent_session_id`) REFERENCES `sessions` (`id`) ON UPDATE NO ACTION ON DELETE SET NULL, CONSTRAINT `6` FOREIGN KEY (`activation_run_id`) REFERENCES `task_runs` (`id`) ON UPDATE NO ACTION ON DELETE SET NULL, CONSTRAINT `7` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (call_id LIKE 'call_%'), CHECK (scope IN ('global', 'workspace')), CHECK (
		caller_kind IN ('session', 'task_run', 'loop_run', 'automation_run')
	), CHECK (trim(caller_id) <> ''), CHECK (
		actor_kind IN ('human', 'agent_session', 'automation', 'extension', 'network_peer', 'daemon')
	), CHECK (trim(actor_id) <> ''), CHECK (trim(governed_root_id) <> ''), CHECK (depth >= 0), CHECK (
		state IN (
			'queued', 'running', 'completed', 'invalid-result', 'completed-without-result',
			'failed', 'canceled', 'timeout', 'expired'
		)
	), CHECK (verdict IS NULL OR verdict IN ('returned', 'extracted', 'repaired')), CHECK (result_bytes IS NULL OR result_bytes >= 0), CHECK (result_budget_bytes > 0), CHECK (result_overflow IN ('store', 'reject')), CHECK (strict IN (0, 1)), CHECK (idle_ttl_seconds > 0), CHECK (failure_detail IS NULL OR length(CAST(failure_detail AS BLOB)) <= 2048), CHECK (repair_attempts IN (0, 1)), CHECK (length(CAST(first_issue_text AS BLOB)) <= 4096), CHECK (length(CAST(second_issue_text AS BLOB)) <= 4096), CHECK (length(CAST(final_prose_preview AS BLOB)) <= 4096), CHECK (trim(request_digest) <> ''), CHECK ((scope = 'global' AND workspace_id = '') OR (scope = 'workspace' AND workspace_id <> '')), CHECK ((agent_name IS NULL) <> (child_session_id IS NULL) OR state <> 'queued'), CHECK ((state = 'completed') = (result_ref IS NOT NULL)), CHECK ((result_ref IS NULL AND result_bytes IS NULL) OR (result_ref IS NOT NULL AND result_bytes IS NOT NULL)));
-- create index "uq_calls_idempotency" to table: "calls"
CREATE UNIQUE INDEX `uq_calls_idempotency` ON `calls` (`profile_id`, `scope`, `workspace_id`, `caller_kind`, `caller_id`, `idempotency_key`) WHERE idempotency_key IS NOT NULL;
-- create index "idx_calls_owner_state" to table: "calls"
CREATE INDEX `idx_calls_owner_state` ON `calls` (`profile_id`, `scope`, `workspace_id`, `state`, `created_at` DESC, `call_id` DESC);
-- create index "idx_calls_caller_state" to table: "calls"
CREATE INDEX `idx_calls_caller_state` ON `calls` (`caller_kind`, `caller_id`, `state`, `created_at` DESC);
-- create index "idx_calls_child_state" to table: "calls"
CREATE INDEX `idx_calls_child_state` ON `calls` (`child_session_id`, `state`, `created_at` DESC);
-- create index "idx_calls_root_state" to table: "calls"
CREATE INDEX `idx_calls_root_state` ON `calls` (`governed_root_id`, `state`);
-- create index "idx_calls_deadline" to table: "calls"
CREATE INDEX `idx_calls_deadline` ON `calls` (`deadline_at`, `call_id`) WHERE deadline_at IS NOT NULL AND state IN ('queued', 'running');
-- create index "idx_calls_activation_run" to table: "calls"
CREATE INDEX `idx_calls_activation_run` ON `calls` (`activation_run_id`) WHERE activation_run_id IS NOT NULL;
-- create "call_permission_atoms" table
CREATE TABLE `call_permission_atoms` (`call_id` text NOT NULL, `atom` text NOT NULL, PRIMARY KEY (`call_id`, `atom`), CONSTRAINT `0` FOREIGN KEY (`call_id`) REFERENCES `calls` (`call_id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (trim(atom) <> ''));
-- create "call_activation_runs" table
CREATE TABLE `call_activation_runs` (`run_id` text NULL, `call_id` text NOT NULL, `workspace_id` text NOT NULL DEFAULT '', `governed_root_id` text NOT NULL, `activation_kind` text NOT NULL, `parent_session_id` text NULL, `target_session_id` text NULL, `agent_name` text NULL, `depth` integer NOT NULL, `idle_ttl_seconds` integer NOT NULL, `runtime_provider` text NOT NULL DEFAULT '', `runtime_model` text NOT NULL DEFAULT '', `runtime_reasoning_effort` text NOT NULL DEFAULT '', `runtime_speed` text NOT NULL DEFAULT '', `created_at` text NOT NULL, PRIMARY KEY (`run_id`), CONSTRAINT `0` FOREIGN KEY (`target_session_id`) REFERENCES `sessions` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`parent_session_id`) REFERENCES `sessions` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `2` FOREIGN KEY (`call_id`) REFERENCES `calls` (`call_id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `3` FOREIGN KEY (`run_id`) REFERENCES `task_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (trim(governed_root_id) <> ''), CHECK (activation_kind IN ('spawn', 'revive')), CHECK (depth >= 0), CHECK (idle_ttl_seconds > 0), CHECK (
		(activation_kind = 'spawn' AND agent_name IS NOT NULL AND target_session_id IS NULL) OR
		(activation_kind = 'revive' AND agent_name IS NULL AND target_session_id IS NOT NULL)
	));
-- create index "call_activation_runs_call_id" to table: "call_activation_runs"
CREATE UNIQUE INDEX `call_activation_runs_call_id` ON `call_activation_runs` (`call_id`);
-- create index "idx_call_activation_runs_root" to table: "call_activation_runs"
CREATE INDEX `idx_call_activation_runs_root` ON `call_activation_runs` (`governed_root_id`, `run_id`);
-- create "operator_caller_sessions" table
CREATE TABLE `operator_caller_sessions` (`profile_id` text NOT NULL, `scope` text NOT NULL, `workspace_id` text NOT NULL DEFAULT '', `session_id` text NOT NULL, `created_at` text NOT NULL, PRIMARY KEY (`profile_id`, `scope`, `workspace_id`), CONSTRAINT `0` FOREIGN KEY (`session_id`) REFERENCES `sessions` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (scope IN ('global', 'workspace')), CHECK ((scope = 'global' AND workspace_id = '') OR (scope = 'workspace' AND workspace_id <> '')));
-- create index "operator_caller_sessions_session_id" to table: "operator_caller_sessions"
CREATE UNIQUE INDEX `operator_caller_sessions_session_id` ON `operator_caller_sessions` (`session_id`);
-- create "call_messages" table
CREATE TABLE `call_messages` (`message_id` text NULL, `profile_id` text NOT NULL, `scope` text NOT NULL, `workspace_id` text NOT NULL DEFAULT '', `from_kind` text NOT NULL, `from_id` text NOT NULL, `to_session_id` text NOT NULL, `call_id` text NULL, `body` text NOT NULL, `dedup_hash` text NOT NULL, `created_at` text NOT NULL, PRIMARY KEY (`message_id`), CONSTRAINT `0` FOREIGN KEY (`call_id`) REFERENCES `calls` (`call_id`) ON UPDATE NO ACTION ON DELETE SET NULL, CONSTRAINT `1` FOREIGN KEY (`to_session_id`) REFERENCES `sessions` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `2` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (message_id LIKE 'msg_%'), CHECK (scope IN ('global', 'workspace')), CHECK (from_kind IN ('session', 'operator')), CHECK (trim(from_id) <> ''), CHECK (trim(dedup_hash) <> ''), CHECK ((scope = 'global' AND workspace_id = '') OR (scope = 'workspace' AND workspace_id <> '')));
-- create index "idx_call_messages_owner_time" to table: "call_messages"
CREATE INDEX `idx_call_messages_owner_time` ON `call_messages` (`profile_id`, `scope`, `workspace_id`, `created_at` DESC, `message_id` DESC);
-- create index "idx_call_messages_recipient_time" to table: "call_messages"
CREATE INDEX `idx_call_messages_recipient_time` ON `call_messages` (`to_session_id`, `created_at`, `message_id`);
-- create index "idx_call_messages_sender_dedup" to table: "call_messages"
CREATE INDEX `idx_call_messages_sender_dedup` ON `call_messages` (`from_kind`, `from_id`, `dedup_hash`, `created_at` DESC);
-- create "call_deliveries" table
CREATE TABLE `call_deliveries` (`delivery_id` text NULL, `kind` text NOT NULL, `subject_id` text NOT NULL, `recipient_session_id` text NOT NULL, `owner_key` text NOT NULL, `wake_event_id` text NOT NULL, `state` text NOT NULL, `reason` text NOT NULL DEFAULT '', `attempts` integer NOT NULL DEFAULT 0, `created_at` text NOT NULL, `updated_at` text NOT NULL, `delivered_at` text NULL, PRIMARY KEY (`delivery_id`), CONSTRAINT `0` FOREIGN KEY (`recipient_session_id`) REFERENCES `sessions` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (delivery_id LIKE 'delivery_%'), CHECK (kind IN ('completion', 'message', 'repair')), CHECK (trim(subject_id) <> ''), CHECK (trim(owner_key) <> ''), CHECK (trim(wake_event_id) <> ''), CHECK (state IN ('pending', 'injected', 'woken', 'failed')), CHECK (attempts >= 0));
-- create index "call_deliveries_wake_event_id" to table: "call_deliveries"
CREATE UNIQUE INDEX `call_deliveries_wake_event_id` ON `call_deliveries` (`wake_event_id`);
-- create index "call_deliveries_kind_subject_id_recipient_session_id" to table: "call_deliveries"
CREATE UNIQUE INDEX `call_deliveries_kind_subject_id_recipient_session_id` ON `call_deliveries` (`kind`, `subject_id`, `recipient_session_id`);
-- create index "idx_call_deliveries_pending" to table: "call_deliveries"
CREATE INDEX `idx_call_deliveries_pending` ON `call_deliveries` (`state`, `created_at`, `delivery_id`);
-- create index "idx_call_deliveries_recipient" to table: "call_deliveries"
CREATE INDEX `idx_call_deliveries_recipient` ON `call_deliveries` (`recipient_session_id`, `state`, `created_at`);
-- create "call_publications" table
CREATE TABLE `call_publications` (`call_id` text NOT NULL, `channel` text NOT NULL, `thread_id` text NOT NULL DEFAULT '', `network_message_id` text NOT NULL, `created_at` text NOT NULL, PRIMARY KEY (`call_id`, `channel`, `thread_id`), CONSTRAINT `0` FOREIGN KEY (`call_id`) REFERENCES `calls` (`call_id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (trim(channel) <> ''), CHECK (trim(network_message_id) <> ''));
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
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
