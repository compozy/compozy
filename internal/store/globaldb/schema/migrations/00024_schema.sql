-- +goose NO TRANSACTION
-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_task_runs" table
CREATE TABLE `new_task_runs` (`id` text NULL, `task_id` text NULL, `workspace_id` text NULL, `status` text NOT NULL, `attempt` integer NOT NULL, `recovery_count` integer NOT NULL DEFAULT 0, `previous_run_id` text NULL, `failure_kind` text NOT NULL DEFAULT '', `claimed_by_kind` text NULL, `claimed_by_ref` text NULL, `session_id` text NULL, `origin_kind` text NOT NULL, `origin_ref` text NOT NULL, `idempotency_key` text NULL, `network_spec_json` text NOT NULL DEFAULT '{"version":"network-participation/v1","mode":"local","source":"built_in_local"}', `network_mode` text NOT NULL DEFAULT 'local', `network_channel` text NULL, `network_source` text NOT NULL DEFAULT 'built_in_local', `designation_group_id` text NOT NULL DEFAULT '', `queued_at` text NOT NULL, `claimed_at` text NULL, `started_at` text NULL, `ended_at` text NULL, `error` text NULL, `metadata_json` text NULL, `result_json` text NULL, `summary` text NOT NULL DEFAULT '', `claimed_agent_name` text NOT NULL DEFAULT '', `claimed_peer_id` text NOT NULL DEFAULT '', `terminalized_by_session_id` text NOT NULL DEFAULT '', `terminalized_by_agent_name` text NOT NULL DEFAULT '', `terminalized_by_peer_id` text NOT NULL DEFAULT '', `terminalized_by_actor_kind` text NOT NULL DEFAULT '', `terminalized_by_actor_ref` text NOT NULL DEFAULT '', `review_required` boolean NOT NULL DEFAULT 0, `review_request_round` integer NOT NULL DEFAULT 0, `review_policy_snapshot` text NOT NULL DEFAULT '', `review_request_id` text NULL, `parent_run_id` text NULL, `review_id` text NULL, `review_round` integer NOT NULL DEFAULT 0, `continuation_reason` text NOT NULL DEFAULT '', `missing_work_json` text NOT NULL DEFAULT '[]', `next_round_guidance` text NOT NULL DEFAULT '', `claim_token` text NULL, `claim_token_hash` text NULL, `lease_until` text NULL, `heartbeat_at` text NULL, `run_kind` text NOT NULL DEFAULT 'worker', `loop_run_id` text NULL, `tokens_used` integer NOT NULL DEFAULT 0, `network_wake_id` text NULL, `network_target_session_id` text NULL, `network_owner_key` text NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`, `network_target_session_id`) REFERENCES `sessions` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`review_id`) REFERENCES `task_run_reviews` (`review_id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `2` FOREIGN KEY (`parent_run_id`) REFERENCES `task_runs` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `3` FOREIGN KEY (`review_request_id`) REFERENCES `task_run_reviews` (`review_id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `4` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `5` FOREIGN KEY (`task_id`) REFERENCES `tasks` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (attempt > 0), CHECK (recovery_count >= 0), CHECK (
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
		), CHECK (review_required IN (0, 1)), CHECK (review_request_round >= 0), CHECK (
			review_policy_snapshot = '' OR
			review_policy_snapshot IN ('none', 'on_success', 'on_failure', 'always')
		), CHECK (review_round >= 0), CHECK (run_kind IN ('worker', 'coordinator', 'network_wake')), CHECK (tokens_used >= 0), CHECK (
			(claimed_by_kind IS NULL AND claimed_by_ref IS NULL) OR
			(claimed_by_kind IS NOT NULL AND claimed_by_ref IS NOT NULL)
		), CHECK (status <> 'queued' OR session_id IS NULL), CHECK (run_kind = 'network_wake' OR task_id IS NOT NULL), CHECK (run_kind <> 'network_wake' OR task_id IS NULL), CHECK (run_kind <> 'network_wake' OR workspace_id IS NOT NULL), CHECK (
			(run_kind = 'network_wake' AND network_wake_id IS NOT NULL
				AND network_target_session_id IS NOT NULL AND network_owner_key IS NOT NULL) OR
			(run_kind <> 'network_wake' AND network_wake_id IS NULL
				AND network_target_session_id IS NULL AND network_owner_key IS NULL)
		));
-- copy rows from old table "task_runs" to new temporary table "new_task_runs"
INSERT INTO `new_task_runs` (`id`, `task_id`, `workspace_id`, `status`, `attempt`, `previous_run_id`, `failure_kind`, `claimed_by_kind`, `claimed_by_ref`, `session_id`, `origin_kind`, `origin_ref`, `idempotency_key`, `network_spec_json`, `network_mode`, `network_channel`, `network_source`, `designation_group_id`, `queued_at`, `claimed_at`, `started_at`, `ended_at`, `error`, `metadata_json`, `result_json`, `summary`, `claimed_agent_name`, `claimed_peer_id`, `terminalized_by_session_id`, `terminalized_by_agent_name`, `terminalized_by_peer_id`, `terminalized_by_actor_kind`, `terminalized_by_actor_ref`, `review_required`, `review_request_round`, `review_policy_snapshot`, `review_request_id`, `parent_run_id`, `review_id`, `review_round`, `continuation_reason`, `missing_work_json`, `next_round_guidance`, `claim_token`, `claim_token_hash`, `lease_until`, `heartbeat_at`, `run_kind`, `loop_run_id`, `tokens_used`, `network_wake_id`, `network_target_session_id`, `network_owner_key`) SELECT `id`, `task_id`, `workspace_id`, `status`, `attempt`, `previous_run_id`, `failure_kind`, `claimed_by_kind`, `claimed_by_ref`, `session_id`, `origin_kind`, `origin_ref`, `idempotency_key`, `network_spec_json`, `network_mode`, `network_channel`, `network_source`, `designation_group_id`, `queued_at`, `claimed_at`, `started_at`, `ended_at`, `error`, `metadata_json`, `result_json`, `summary`, `claimed_agent_name`, `claimed_peer_id`, `terminalized_by_session_id`, `terminalized_by_agent_name`, `terminalized_by_peer_id`, `terminalized_by_actor_kind`, `terminalized_by_actor_ref`, `review_required`, `review_request_round`, `review_policy_snapshot`, `review_request_id`, `parent_run_id`, `review_id`, `review_round`, `continuation_reason`, `missing_work_json`, `next_round_guidance`, `claim_token`, `claim_token_hash`, `lease_until`, `heartbeat_at`, `run_kind`, `loop_run_id`, `tokens_used`, `network_wake_id`, `network_target_session_id`, `network_owner_key` FROM `task_runs`;
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
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
