-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_automation_runs" table
CREATE TABLE `new_automation_runs` (`id` text NULL, `profile_id` text NULL, `job_id` text NULL, `trigger_id` text NULL, `session_id` text NULL, `task_id` text NULL, `task_run_id` text NULL, `status` text NOT NULL, `attempt` integer NOT NULL DEFAULT 1, `started_at` text NULL, `ended_at` text NULL, `error` text NULL, `loop_run_id` text NULL, `fire_id` text NULL, `scheduled_at` text NULL, `delivery_error` text NULL, `delivery_error_at` text NULL, `network_participation` text NULL, `metadata_json` text NOT NULL DEFAULT '{}', PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`loop_run_id`) REFERENCES `loop_runs` (`id`) ON UPDATE NO ACTION ON DELETE SET NULL, CONSTRAINT `1` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (
		network_participation IS NULL OR json_valid(network_participation)
	  ));
-- copy rows from old table "automation_runs" to new temporary table "new_automation_runs"
INSERT INTO `new_automation_runs` (`id`, `job_id`, `trigger_id`, `session_id`, `task_id`, `task_run_id`, `status`, `attempt`, `started_at`, `ended_at`, `error`, `loop_run_id`, `fire_id`, `scheduled_at`, `delivery_error`, `delivery_error_at`, `network_participation`, `metadata_json`) SELECT `id`, `job_id`, `trigger_id`, `session_id`, `task_id`, `task_run_id`, `status`, `attempt`, `started_at`, `ended_at`, `error`, `loop_run_id`, `fire_id`, `scheduled_at`, `delivery_error`, `delivery_error_at`, `network_participation`, `metadata_json` FROM `automation_runs`;
-- drop trigger "automation_watch_events_after_insert" before applying its declarative change
DROP TRIGGER IF EXISTS `automation_watch_events_after_insert`;
-- drop trigger "automation_watch_events_after_terminal_update" before applying its declarative change
DROP TRIGGER IF EXISTS `automation_watch_events_after_terminal_update`;
-- drop "automation_runs" table after copying rows
DROP TABLE `automation_runs`;
-- rename temporary table "new_automation_runs" to "automation_runs"
ALTER TABLE `new_automation_runs` RENAME TO `automation_runs`;
-- Preserve known ownership without assigning orphan history to a guessed profile.
UPDATE automation_runs SET profile_id = (
  SELECT id FROM profiles WHERE id = COALESCE(
    (SELECT NULLIF(CAST(json_extract(spec_json, '$.profile_id') AS TEXT), '')
       FROM resource_records WHERE kind = 'automation.job' AND id = automation_runs.job_id AND json_valid(spec_json)),
    (SELECT profile_id FROM automation_jobs WHERE id = automation_runs.job_id),
    (SELECT NULLIF(CAST(json_extract(spec_json, '$.profile_id') AS TEXT), '')
       FROM resource_records WHERE kind = 'automation.trigger' AND id = automation_runs.trigger_id AND json_valid(spec_json)),
    (SELECT profile_id FROM automation_triggers WHERE id = automation_runs.trigger_id),
    (SELECT profile_id FROM sessions WHERE id = automation_runs.session_id),
    (SELECT profile_id FROM loop_runs WHERE id = automation_runs.loop_run_id)
  )
);
-- create index "idx_automation_runs_job" to table: "automation_runs"
CREATE INDEX `idx_automation_runs_job` ON `automation_runs` (`job_id`);
-- create index "idx_automation_runs_loop_run" to table: "automation_runs"
CREATE INDEX `idx_automation_runs_loop_run` ON `automation_runs` (`loop_run_id`);
-- create index "idx_automation_runs_started" to table: "automation_runs"
CREATE INDEX `idx_automation_runs_started` ON `automation_runs` (`started_at`);
-- create index "idx_automation_runs_status" to table: "automation_runs"
CREATE INDEX `idx_automation_runs_status` ON `automation_runs` (`status`);
-- create index "idx_automation_runs_trigger" to table: "automation_runs"
CREATE INDEX `idx_automation_runs_trigger` ON `automation_runs` (`trigger_id`);
-- create index "uq_automation_runs_fire_id" to table: "automation_runs"
CREATE UNIQUE INDEX `uq_automation_runs_fire_id` ON `automation_runs` (`fire_id`) WHERE fire_id IS NOT NULL;
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
-- apply declarative trigger "automation_watch_events_after_insert" on table "automation_runs"
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
-- apply declarative trigger "automation_watch_events_after_terminal_update" on table "automation_runs"
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
