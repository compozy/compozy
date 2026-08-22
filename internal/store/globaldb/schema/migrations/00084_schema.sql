-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- add column "lifecycle_token" to table: "extensions"
ALTER TABLE `extensions` ADD COLUMN `lifecycle_token` text NOT NULL DEFAULT '';
-- create "new_loop_runs" table
CREATE TABLE `new_loop_runs` (`id` text NULL, `workspace_id` text NOT NULL, `loop_name` text NOT NULL, `status` text NOT NULL, `historical` integer NOT NULL DEFAULT 0, `completion_state` text NOT NULL DEFAULT 'complete', `forked_from_run_id` text NULL, `forked_from_generation` integer NULL, `generation` integer NOT NULL DEFAULT 0, `reattempt_strategy` text NOT NULL DEFAULT 'failed_only', `last_progress_at` timestamp NOT NULL, `budget_tokens` integer NOT NULL DEFAULT 0, `budget_wall_sec` integer NOT NULL DEFAULT 0, `budget_on_exceeded` text NOT NULL DEFAULT 'halt', `tokens_used` integer NOT NULL DEFAULT 0, `parent_loop_run_id` text NULL, `pause_requested` integer NOT NULL DEFAULT 0, `inputs_json` text NOT NULL, `created_at` text NOT NULL DEFAULT '1970-01-01T00:00:00.000000000Z', `iteration_cap` integer NOT NULL DEFAULT 0, `started_by_kind` text NOT NULL DEFAULT '', `started_by_ref` text NOT NULL DEFAULT '', `started_origin_kind` text NOT NULL DEFAULT '', `started_origin_ref` text NOT NULL DEFAULT '', `started_at` text NOT NULL DEFAULT '1970-01-01T00:00:00.000000000Z', `definition_version` integer NOT NULL DEFAULT 0, `definition_digest` text NOT NULL DEFAULT '', `active_gate_id` text NOT NULL DEFAULT '', `active_human_criteria_json` text NOT NULL DEFAULT '[]', `budget_approval_seq` integer NOT NULL DEFAULT 0, `start_metadata_json` text NOT NULL DEFAULT '{}', `origin_kind` text NOT NULL DEFAULT 'catalog', `origin_session_id` text NULL, `goal_cleared_at` timestamp NULL, `budget_version` integer NOT NULL DEFAULT 0, `goal_context_nudge_ratio` real NOT NULL DEFAULT 0.8, `control_actor_kind` text NULL, `control_actor_id` text NULL, `control_requested_at` timestamp NULL, `origin_creation_profile_ref` text NULL, `origin_policy_spec_digest` text NULL, `origin_creation_digest` text NULL, `network_spec_json` text NOT NULL DEFAULT '{"version":"network-participation/v1","mode":"local","source":"built_in_local"}', `network_mode` text NOT NULL DEFAULT 'local', `network_channel` text NULL, `network_source` text NOT NULL DEFAULT 'built_in_local', `best_generation` integer NULL, `best_score` real NULL, `cancel_requested` integer NOT NULL DEFAULT 0, `cancel_kind` text NOT NULL DEFAULT '', PRIMARY KEY (`id`), CHECK (historical IN (0, 1)), CHECK (completion_state IN ('complete','partial')), CHECK (budget_version >= 0), CHECK (goal_context_nudge_ratio >= 0.0 AND goal_context_nudge_ratio <= 1.0), CHECK (origin_creation_profile_ref IS NULL OR length(trim(origin_creation_profile_ref)) > 0), CHECK (origin_policy_spec_digest IS NULL OR length(trim(origin_policy_spec_digest)) > 0), CHECK (origin_creation_digest IS NULL OR length(trim(origin_creation_digest)) > 0), CHECK (json_valid(network_spec_json)), CHECK (network_mode IN ('local', 'live')), CHECK (network_source IN (
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
INSERT INTO `new_loop_runs` (`id`, `workspace_id`, `loop_name`, `status`, `historical`, `completion_state`, `forked_from_run_id`, `forked_from_generation`, `generation`, `reattempt_strategy`, `last_progress_at`, `budget_tokens`, `budget_wall_sec`, `budget_on_exceeded`, `tokens_used`, `parent_loop_run_id`, `pause_requested`, `inputs_json`, `created_at`, `iteration_cap`, `started_by_kind`, `started_by_ref`, `started_origin_kind`, `started_origin_ref`, `started_at`, `definition_version`, `definition_digest`, `active_gate_id`, `active_human_criteria_json`, `budget_approval_seq`, `start_metadata_json`, `origin_kind`, `origin_session_id`, `goal_cleared_at`, `budget_version`, `goal_context_nudge_ratio`, `control_actor_kind`, `control_actor_id`, `control_requested_at`, `origin_creation_profile_ref`, `origin_policy_spec_digest`, `origin_creation_digest`, `network_spec_json`, `network_mode`, `network_channel`, `network_source`, `best_generation`, `best_score`, `cancel_requested`, `cancel_kind`) SELECT `id`, `workspace_id`, `loop_name`, `status`, `historical`, `completion_state`, `forked_from_run_id`, `forked_from_generation`, `generation`, `reattempt_strategy`, `last_progress_at`, `budget_tokens`, `budget_wall_sec`, `budget_on_exceeded`, `tokens_used`, `parent_loop_run_id`, `pause_requested`, `inputs_json`, `created_at`, `iteration_cap`, `started_by_kind`, `started_by_ref`, `started_origin_kind`, `started_origin_ref`, `started_at`, `definition_version`, `definition_digest`, `active_gate_id`, `active_human_criteria_json`, `budget_approval_seq`, `start_metadata_json`, `origin_kind`, `origin_session_id`, `goal_cleared_at`, `budget_version`, `goal_context_nudge_ratio`, `control_actor_kind`, `control_actor_id`, `control_requested_at`, `origin_creation_profile_ref`, `origin_policy_spec_digest`, `origin_creation_digest`, `network_spec_json`, `network_mode`, `network_channel`, `network_source`, `best_generation`, `best_score`, `cancel_requested`, `cancel_kind` FROM `loop_runs`;
-- drop trigger "automation_watch_events_after_insert" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `automation_watch_events_after_insert`;
-- drop trigger "automation_watch_events_after_terminal_update" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `automation_watch_events_after_terminal_update`;
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
			  AND historical = 0
			  AND status IN ('queued','running','watching','needs-approval','paused');
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
