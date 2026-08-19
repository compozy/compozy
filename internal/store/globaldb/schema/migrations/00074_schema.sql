-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_loop_generation_outputs" table
CREATE TABLE `new_loop_generation_outputs` (`loop_run_id` text NOT NULL, `generation` integer NOT NULL, `node_id` text NOT NULL, `item_index` integer NOT NULL DEFAULT 0, `status` text NOT NULL, `output_ref` text NULL, `task_run_id` text NULL, `child_loop_run_id` text NULL, `resolved_runtime_json` text NULL, `attempt` integer NOT NULL DEFAULT 1, `next_attempt_at` timestamp NULL, `first_scheduled_at` timestamp NULL, `epoch` integer NOT NULL DEFAULT 0, `goal_status` text NULL, `goal_turns_used` integer NULL, `goal_turn_limit` integer NULL, PRIMARY KEY (`loop_run_id`, `generation`, `node_id`, `item_index`), CONSTRAINT `0` FOREIGN KEY (`loop_run_id`) REFERENCES `loop_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (status IN (
				'pending','enqueued','running','retrying','waiting','paused','awaiting_child',
				'control_pending','awaiting_goal','succeeded','partial','failed','canceled','quarantined'
			)), CHECK (resolved_runtime_json IS NULL OR json_valid(resolved_runtime_json)), CHECK (attempt >= 1));
-- copy rows from old table "loop_generation_outputs" to new temporary table "new_loop_generation_outputs"
INSERT INTO `new_loop_generation_outputs` (`loop_run_id`, `generation`, `node_id`, `item_index`, `status`, `output_ref`, `task_run_id`, `child_loop_run_id`, `resolved_runtime_json`, `attempt`, `next_attempt_at`, `first_scheduled_at`, `epoch`, `goal_status`, `goal_turns_used`, `goal_turn_limit`) SELECT `loop_run_id`, `generation`, `node_id`, `item_index`, `status`, `output_ref`, `task_run_id`, `child_loop_run_id`, `resolved_runtime_json`, `attempt`, `next_attempt_at`, `first_scheduled_at`, `epoch`, `goal_status`, `goal_turns_used`, `goal_turn_limit` FROM `loop_generation_outputs`;
-- drop trigger "automation_watch_events_after_insert" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `automation_watch_events_after_insert`;
-- drop trigger "automation_watch_events_after_terminal_update" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `automation_watch_events_after_terminal_update`;
-- drop "loop_generation_outputs" table after copying rows
DROP TABLE `loop_generation_outputs`;
-- rename temporary table "new_loop_generation_outputs" to "loop_generation_outputs"
ALTER TABLE `new_loop_generation_outputs` RENAME TO `loop_generation_outputs`;
-- create index "idx_loop_generation_outputs_retry_due" to table: "loop_generation_outputs"
CREATE INDEX `idx_loop_generation_outputs_retry_due` ON `loop_generation_outputs` (`next_attempt_at`, `loop_run_id`, `generation`, `node_id`, `item_index`) WHERE status = 'retrying' AND next_attempt_at IS NOT NULL;
-- create index "idx_loop_generation_outputs_output_ref" to table: "loop_generation_outputs"
CREATE INDEX `idx_loop_generation_outputs_output_ref` ON `loop_generation_outputs` (`output_ref`);
-- create "new_loop_runs" table
CREATE TABLE `new_loop_runs` (`id` text NULL, `workspace_id` text NOT NULL, `loop_name` text NOT NULL, `status` text NOT NULL, `completion_state` text NOT NULL DEFAULT 'complete', `generation` integer NOT NULL DEFAULT 0, `reattempt_strategy` text NOT NULL DEFAULT 'failed_only', `last_progress_at` timestamp NOT NULL, `budget_tokens` integer NOT NULL DEFAULT 0, `budget_wall_sec` integer NOT NULL DEFAULT 0, `budget_on_exceeded` text NOT NULL DEFAULT 'halt', `tokens_used` integer NOT NULL DEFAULT 0, `parent_loop_run_id` text NULL, `pause_requested` integer NOT NULL DEFAULT 0, `inputs_json` text NOT NULL, `created_at` text NOT NULL DEFAULT '1970-01-01T00:00:00.000000000Z', `iteration_cap` integer NOT NULL DEFAULT 0, `started_by_kind` text NOT NULL DEFAULT '', `started_by_ref` text NOT NULL DEFAULT '', `started_origin_kind` text NOT NULL DEFAULT '', `started_origin_ref` text NOT NULL DEFAULT '', `started_at` text NOT NULL DEFAULT '1970-01-01T00:00:00.000000000Z', `definition_version` integer NOT NULL DEFAULT 0, `definition_digest` text NOT NULL DEFAULT '', `active_gate_id` text NOT NULL DEFAULT '', `active_human_criteria_json` text NOT NULL DEFAULT '[]', `budget_approval_seq` integer NOT NULL DEFAULT 0, `start_metadata_json` text NOT NULL DEFAULT '{}', `origin_kind` text NOT NULL DEFAULT 'catalog', `origin_session_id` text NULL, `goal_cleared_at` timestamp NULL, `budget_version` integer NOT NULL DEFAULT 0, `goal_context_nudge_ratio` real NOT NULL DEFAULT 0.8, `control_actor_kind` text NULL, `control_actor_id` text NULL, `control_requested_at` timestamp NULL, `origin_creation_profile_ref` text NULL, `origin_policy_spec_digest` text NULL, `origin_creation_digest` text NULL, `network_spec_json` text NOT NULL DEFAULT '{"version":"network-participation/v1","mode":"local","source":"built_in_local"}', `network_mode` text NOT NULL DEFAULT 'local', `network_channel` text NULL, `network_source` text NOT NULL DEFAULT 'built_in_local', `best_generation` integer NULL, `best_score` real NULL, `cancel_requested` integer NOT NULL DEFAULT 0, `cancel_kind` text NOT NULL DEFAULT '', PRIMARY KEY (`id`), CHECK (completion_state IN ('complete','partial')), CHECK (budget_version >= 0), CHECK (goal_context_nudge_ratio >= 0.0 AND goal_context_nudge_ratio <= 1.0), CHECK (origin_creation_profile_ref IS NULL OR length(trim(origin_creation_profile_ref)) > 0), CHECK (origin_policy_spec_digest IS NULL OR length(trim(origin_policy_spec_digest)) > 0), CHECK (origin_creation_digest IS NULL OR length(trim(origin_creation_digest)) > 0), CHECK (json_valid(network_spec_json)), CHECK (network_mode IN ('local', 'live')), CHECK (network_source IN (
						'explicit_request', 'task_profile', 'workspace_coordination',
						'loop_definition', 'automation_job', 'built_in_local'
					)), CHECK (cancel_kind IN ('', 'cancel', 'kill')), CHECK (
						(best_generation IS NULL AND best_score IS NULL)
						OR (best_generation IS NOT NULL AND best_score IS NOT NULL
							AND best_generation >= 1 AND best_generation <= generation)
					));
-- copy rows from old table "loop_runs" to new temporary table "new_loop_runs"
INSERT INTO `new_loop_runs` (`id`, `workspace_id`, `loop_name`, `status`, `generation`, `reattempt_strategy`, `last_progress_at`, `budget_tokens`, `budget_wall_sec`, `budget_on_exceeded`, `tokens_used`, `parent_loop_run_id`, `pause_requested`, `inputs_json`, `created_at`, `iteration_cap`, `started_by_kind`, `started_by_ref`, `started_origin_kind`, `started_origin_ref`, `started_at`, `definition_version`, `definition_digest`, `active_gate_id`, `active_human_criteria_json`, `budget_approval_seq`, `start_metadata_json`, `origin_kind`, `origin_session_id`, `goal_cleared_at`, `budget_version`, `goal_context_nudge_ratio`, `control_actor_kind`, `control_actor_id`, `control_requested_at`, `origin_creation_profile_ref`, `origin_policy_spec_digest`, `origin_creation_digest`, `network_spec_json`, `network_mode`, `network_channel`, `network_source`, `best_generation`, `best_score`, `cancel_requested`, `cancel_kind`) SELECT `id`, `workspace_id`, `loop_name`, `status`, `generation`, `reattempt_strategy`, `last_progress_at`, `budget_tokens`, `budget_wall_sec`, `budget_on_exceeded`, `tokens_used`, `parent_loop_run_id`, `pause_requested`, `inputs_json`, `created_at`, `iteration_cap`, `started_by_kind`, `started_by_ref`, `started_origin_kind`, `started_origin_ref`, `started_at`, `definition_version`, `definition_digest`, `active_gate_id`, `active_human_criteria_json`, `budget_approval_seq`, `start_metadata_json`, `origin_kind`, `origin_session_id`, `goal_cleared_at`, `budget_version`, `goal_context_nudge_ratio`, `control_actor_kind`, `control_actor_id`, `control_requested_at`, `origin_creation_profile_ref`, `origin_policy_spec_digest`, `origin_creation_digest`, `network_spec_json`, `network_mode`, `network_channel`, `network_source`, `best_generation`, `best_score`, `cancel_requested`, `cancel_kind` FROM `loop_runs`;
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
-- create "new_loop_run_events" table
CREATE TABLE `new_loop_run_events` (`watch_seq` integer NOT NULL PRIMARY KEY AUTOINCREMENT, `id` text NOT NULL, `loop_run_id` text NOT NULL, `workspace_id` text NOT NULL, `seq` integer NOT NULL, `kind` text NOT NULL, `payload_json` text NOT NULL, `at` timestamp NOT NULL, `delivery_key` text NULL, CHECK (kind IN (
				'node_running','node_succeeded','node_failed','node_quarantined','node_requeued',
				'node_paused','node_resumed','node_wait_started','node_wait_resumed',
				'duplicate_suppressed','node_canceled','node_killed','node_attention_flagged',
				'node_attention_cleared','target_breaker_transition','gate_verdict',
				'generation_started','channel_msg','token_tick','needs_approval','status_changed',
				'goal_turn_started','goal_turn_completed','goal_status_changed','runtime_applied',
				'predicate_diagnostic','route_taken','node_retry_scheduled','stale_schedule_dropped',
				'late_arrival','effect_results','custom_event','request_opened','request_answered',
				'request_expired','request_canceled','node_amended','branch_pruned'
			)), CHECK (json_valid(payload_json)));
-- copy rows from old table "loop_run_events" to new temporary table "new_loop_run_events"
INSERT INTO `new_loop_run_events` (`watch_seq`, `id`, `loop_run_id`, `workspace_id`, `seq`, `kind`, `payload_json`, `at`, `delivery_key`) SELECT `watch_seq`, `id`, `loop_run_id`, `workspace_id`, `seq`, `kind`, `payload_json`, `at`, `delivery_key` FROM `loop_run_events`;
-- drop "loop_run_events" table after copying rows
DROP TABLE `loop_run_events`;
-- rename temporary table "new_loop_run_events" to "loop_run_events"
ALTER TABLE `new_loop_run_events` RENAME TO `loop_run_events`;
-- create index "loop_run_events_id" to table: "loop_run_events"
CREATE UNIQUE INDEX `loop_run_events_id` ON `loop_run_events` (`id`);
-- create index "idx_loop_run_events_run_seq" to table: "loop_run_events"
CREATE INDEX `idx_loop_run_events_run_seq` ON `loop_run_events` (`loop_run_id`, `seq`);
-- create index "idx_loop_run_events_watch_stream" to table: "loop_run_events"
CREATE INDEX `idx_loop_run_events_watch_stream` ON `loop_run_events` (`workspace_id`, `watch_seq`);
-- create index "uq_loop_run_events_delivery" to table: "loop_run_events"
CREATE UNIQUE INDEX `uq_loop_run_events_delivery` ON `loop_run_events` (`loop_run_id`, `delivery_key`) WHERE delivery_key IS NOT NULL;
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
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
