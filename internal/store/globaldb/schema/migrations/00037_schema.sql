-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
DROP TRIGGER automation_watch_events_after_insert;
DROP TRIGGER automation_watch_events_after_terminal_update;
-- Greenfield hard cut: pre-ratchet loop history cannot satisfy provenance and lineage invariants.
DELETE FROM session_input_queue WHERE loop_run_id IS NOT NULL;
UPDATE task_runs SET loop_run_id = NULL WHERE loop_run_id IS NOT NULL;
DELETE FROM loop_run_events;
DELETE FROM loop_goal_session_outbox;
DELETE FROM loop_goal_session_cleanup;
DELETE FROM loop_goal_judge_attempts;
DELETE FROM loop_goal_turns;
DELETE FROM loop_goal_checkpoints;
DELETE FROM loop_goal_binding_retry_witnesses;
DELETE FROM loop_session_bindings;
DELETE FROM loop_generation_outputs;
DELETE FROM loop_gate_decisions;
UPDATE automation_runs SET loop_run_id = NULL WHERE loop_run_id IS NOT NULL;
DELETE FROM loop_runs;
DELETE FROM loop_output_blobs;
-- create "new_loop_runs" table
CREATE TABLE `new_loop_runs` (`id` text NULL, `workspace_id` text NOT NULL, `loop_name` text NOT NULL, `status` text NOT NULL, `generation` integer NOT NULL DEFAULT 0, `reattempt_strategy` text NOT NULL DEFAULT 'failed_only', `last_progress_at` timestamp NOT NULL, `budget_tokens` integer NOT NULL DEFAULT 0, `budget_wall_sec` integer NOT NULL DEFAULT 0, `budget_on_exceeded` text NOT NULL DEFAULT 'halt', `tokens_used` integer NOT NULL DEFAULT 0, `parent_loop_run_id` text NULL, `pause_requested` integer NOT NULL DEFAULT 0, `inputs_json` text NOT NULL, `created_at` text NOT NULL DEFAULT '1970-01-01T00:00:00.000000000Z', `iteration_cap` integer NOT NULL DEFAULT 0, `started_by_kind` text NOT NULL DEFAULT '', `started_by_ref` text NOT NULL DEFAULT '', `started_origin_kind` text NOT NULL DEFAULT '', `started_origin_ref` text NOT NULL DEFAULT '', `started_at` text NOT NULL DEFAULT '1970-01-01T00:00:00.000000000Z', `definition_version` integer NOT NULL DEFAULT 0, `definition_digest` text NOT NULL DEFAULT '', `active_gate_id` text NOT NULL DEFAULT '', `active_human_criteria_json` text NOT NULL DEFAULT '[]', `budget_approval_seq` integer NOT NULL DEFAULT 0, `start_metadata_json` text NOT NULL DEFAULT '{}', `origin_kind` text NOT NULL DEFAULT 'catalog', `origin_session_id` text NULL, `goal_cleared_at` timestamp NULL, `budget_version` integer NOT NULL DEFAULT 0, `goal_context_nudge_ratio` real NOT NULL DEFAULT 0.8, `control_actor_kind` text NULL, `control_actor_id` text NULL, `control_requested_at` timestamp NULL, `origin_creation_profile_ref` text NULL, `origin_policy_spec_digest` text NULL, `origin_creation_digest` text NULL, `network_spec_json` text NOT NULL DEFAULT '{"version":"network-participation/v1","mode":"local","source":"built_in_local"}', `network_mode` text NOT NULL DEFAULT 'local', `network_channel` text NULL, `network_source` text NOT NULL DEFAULT 'built_in_local', `best_generation` integer NULL, `best_score` real NULL, PRIMARY KEY (`id`), CHECK (budget_version >= 0), CHECK (goal_context_nudge_ratio >= 0.0 AND goal_context_nudge_ratio <= 1.0), CHECK (origin_creation_profile_ref IS NULL OR length(trim(origin_creation_profile_ref)) > 0), CHECK (origin_policy_spec_digest IS NULL OR length(trim(origin_policy_spec_digest)) > 0), CHECK (origin_creation_digest IS NULL OR length(trim(origin_creation_digest)) > 0), CHECK (json_valid(network_spec_json)), CHECK (network_mode IN ('local', 'live')), CHECK (network_source IN (
						'explicit_request', 'task_profile', 'workspace_coordination',
						'loop_definition', 'automation_job', 'built_in_local'
					)), CHECK (
						(best_generation IS NULL AND best_score IS NULL)
						OR (best_generation IS NOT NULL AND best_score IS NOT NULL
							AND best_generation >= 1 AND best_generation <= generation)
					));
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
-- create "loop_gate_verdicts" table
CREATE TABLE `loop_gate_verdicts` (`loop_run_id` text NOT NULL, `generation` integer NOT NULL, `gate_id` text NOT NULL, `outcome` text NOT NULL, `score` real NULL, `route_cause_rank` integer NULL, `blocking_issues_json` text NOT NULL DEFAULT '[]', `criteria_json` text NOT NULL DEFAULT '[]', `decided_at` timestamp NOT NULL, PRIMARY KEY (`loop_run_id`, `generation`, `gate_id`), CONSTRAINT `0` FOREIGN KEY (`loop_run_id`) REFERENCES `loop_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (generation >= 1), CHECK (outcome IN (
				'approved','rejected','awaiting_approval','blocked','error','timeout','invalid_output'
			)), CHECK (json_valid(blocking_issues_json)), CHECK (json_valid(criteria_json)));
-- create index "idx_loop_gate_verdicts_route_cause" to table: "loop_gate_verdicts"
CREATE INDEX `idx_loop_gate_verdicts_route_cause` ON `loop_gate_verdicts` (`loop_run_id`, `generation`, `route_cause_rank`) WHERE route_cause_rank IS NOT NULL;
-- create "loop_generations" table
CREATE TABLE `loop_generations` (`loop_run_id` text NOT NULL, `generation` integer NOT NULL, `parent_generation` integer NOT NULL DEFAULT 0, `origin` text NOT NULL, `created_at` timestamp NOT NULL, PRIMARY KEY (`loop_run_id`, `generation`), CONSTRAINT `0` FOREIGN KEY (`loop_run_id`) REFERENCES `loop_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (generation >= 1), CHECK (parent_generation >= 0 AND parent_generation < generation), CHECK (origin IN (
				'initial','stop_when','reattempt','gate_revise','gate_next_generation',
				'dod_retry','ratchet_restore'
			)));
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
