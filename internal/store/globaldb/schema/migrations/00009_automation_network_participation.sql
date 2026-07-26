-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- drop automation watch triggers before rebuilding their referenced owner tables
DROP TRIGGER automation_watch_events_after_insert;
DROP TRIGGER automation_watch_events_after_terminal_update;
-- create "new_automation_jobs" table
CREATE TABLE `new_automation_jobs` (`id` text NULL, `scope` text NOT NULL, `name` text NOT NULL, `agent_name` text NOT NULL, `workspace_id` text NULL, `prompt` text NOT NULL, `schedule` text NULL, `task` text NULL, `enabled` boolean NOT NULL DEFAULT 1, `retry` text NOT NULL, `fire_limit` text NOT NULL, `source` text NOT NULL DEFAULT 'dynamic', `target_kind` text NOT NULL DEFAULT 'agent', `loop_workspace_id` text NULL, `loop_name` text NULL, `loop_inputs` text NULL, `loop_input_mapping` text NULL, `loop_network_participation` text NULL, `created_at` text NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`loop_workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (scope IN ('global', 'workspace')), CHECK (target_kind IN ('agent', 'loop')), CHECK (
			loop_network_participation IS NULL OR json_valid(loop_network_participation)
		), CHECK (
			(scope = 'global' AND workspace_id IS NULL) OR
			(scope = 'workspace' AND workspace_id IS NOT NULL)
		));
-- copy rows from old table "automation_jobs" to new temporary table "new_automation_jobs"
INSERT INTO `new_automation_jobs` (`id`, `scope`, `name`, `agent_name`, `workspace_id`, `prompt`, `schedule`, `task`, `enabled`, `retry`, `fire_limit`, `source`, `target_kind`, `loop_workspace_id`, `loop_name`, `loop_inputs`, `loop_input_mapping`, `created_at`, `updated_at`) SELECT `id`, `scope`, `name`, `agent_name`, `workspace_id`, `prompt`, `schedule`, `task`, `enabled`, `retry`, `fire_limit`, `source`, `target_kind`, `loop_workspace_id`, `loop_name`, `loop_inputs`, `loop_input_mapping`, `created_at`, `updated_at` FROM `automation_jobs`;
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
-- create "new_automation_runs" table
CREATE TABLE `new_automation_runs` (`id` text NULL, `job_id` text NULL, `trigger_id` text NULL, `session_id` text NULL, `task_id` text NULL, `task_run_id` text NULL, `status` text NOT NULL, `attempt` integer NOT NULL DEFAULT 1, `started_at` text NULL, `ended_at` text NULL, `error` text NULL, `loop_run_id` text NULL, `fire_id` text NULL, `scheduled_at` text NULL, `delivery_error` text NULL, `delivery_error_at` text NULL, `network_participation` text NULL, `metadata_json` text NOT NULL DEFAULT '{}', PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`loop_run_id`) REFERENCES `loop_runs` (`id`) ON UPDATE NO ACTION ON DELETE SET NULL, CHECK (
		network_participation IS NULL OR json_valid(network_participation)
	  ));
-- copy rows from old table "automation_runs" to new temporary table "new_automation_runs"
INSERT INTO `new_automation_runs` (`id`, `job_id`, `trigger_id`, `session_id`, `task_id`, `task_run_id`, `status`, `attempt`, `started_at`, `ended_at`, `error`, `loop_run_id`, `fire_id`, `scheduled_at`, `delivery_error`, `delivery_error_at`, `metadata_json`) SELECT `id`, `job_id`, `trigger_id`, `session_id`, `task_id`, `task_run_id`, `status`, `attempt`, `started_at`, `ended_at`, `error`, `loop_run_id`, `fire_id`, `scheduled_at`, `delivery_error`, `delivery_error_at`, `metadata_json` FROM `automation_runs`;
-- drop "automation_runs" table after copying rows
DROP TABLE `automation_runs`;
-- rename temporary table "new_automation_runs" to "automation_runs"
ALTER TABLE `new_automation_runs` RENAME TO `automation_runs`;
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
-- create "new_automation_triggers" table
CREATE TABLE `new_automation_triggers` (`id` text NULL, `scope` text NOT NULL, `name` text NOT NULL, `agent_name` text NOT NULL, `workspace_id` text NULL, `prompt` text NOT NULL, `event` text NOT NULL, `filter` text NULL, `enabled` boolean NOT NULL DEFAULT 1, `retry` text NOT NULL, `fire_limit` text NOT NULL, `source` text NOT NULL DEFAULT 'dynamic', `webhook_id` text NULL, `endpoint_slug` text NULL, `webhook_secret_ref` text NULL, `target_kind` text NOT NULL DEFAULT 'agent', `loop_workspace_id` text NULL, `loop_name` text NULL, `loop_inputs` text NULL, `loop_input_mapping` text NULL, `loop_network_participation` text NULL, `created_at` text NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`loop_workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (scope IN ('global', 'workspace')), CHECK (target_kind IN ('agent', 'loop')), CHECK (
			loop_network_participation IS NULL OR json_valid(loop_network_participation)
		), CHECK (
			(scope = 'global' AND workspace_id IS NULL) OR
			(scope = 'workspace' AND workspace_id IS NOT NULL)
		));
-- copy rows from old table "automation_triggers" to new temporary table "new_automation_triggers"
INSERT INTO `new_automation_triggers` (`id`, `scope`, `name`, `agent_name`, `workspace_id`, `prompt`, `event`, `filter`, `enabled`, `retry`, `fire_limit`, `source`, `webhook_id`, `endpoint_slug`, `webhook_secret_ref`, `target_kind`, `loop_workspace_id`, `loop_name`, `loop_inputs`, `loop_input_mapping`, `created_at`, `updated_at`) SELECT `id`, `scope`, `name`, `agent_name`, `workspace_id`, `prompt`, `event`, `filter`, `enabled`, `retry`, `fire_limit`, `source`, `webhook_id`, `endpoint_slug`, `webhook_secret_ref`, `target_kind`, `loop_workspace_id`, `loop_name`, `loop_inputs`, `loop_input_mapping`, `created_at`, `updated_at` FROM `automation_triggers`;
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
-- recreate automation watch triggers after every referenced owner table is stable
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
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;

-- +goose Down
-- reverse: create index "uq_automation_triggers_workspace_name" to table: "automation_triggers"
DROP INDEX `uq_automation_triggers_workspace_name`;
-- reverse: create index "uq_automation_triggers_webhook_id" to table: "automation_triggers"
DROP INDEX `uq_automation_triggers_webhook_id`;
-- reverse: create index "uq_automation_triggers_global_name" to table: "automation_triggers"
DROP INDEX `uq_automation_triggers_global_name`;
-- reverse: create index "idx_automation_triggers_loop_target" to table: "automation_triggers"
DROP INDEX `idx_automation_triggers_loop_target`;
-- reverse: create index "idx_automation_triggers_event" to table: "automation_triggers"
DROP INDEX `idx_automation_triggers_event`;
-- reverse: create index "idx_automation_triggers_enabled" to table: "automation_triggers"
DROP INDEX `idx_automation_triggers_enabled`;
-- reverse: create "new_automation_triggers" table
DROP TABLE `new_automation_triggers`;
-- reverse: create index "uq_automation_runs_fire_id" to table: "automation_runs"
DROP INDEX `uq_automation_runs_fire_id`;
-- reverse: create index "idx_automation_runs_trigger" to table: "automation_runs"
DROP INDEX `idx_automation_runs_trigger`;
-- reverse: create index "idx_automation_runs_status" to table: "automation_runs"
DROP INDEX `idx_automation_runs_status`;
-- reverse: create index "idx_automation_runs_started" to table: "automation_runs"
DROP INDEX `idx_automation_runs_started`;
-- reverse: create index "idx_automation_runs_loop_run" to table: "automation_runs"
DROP INDEX `idx_automation_runs_loop_run`;
-- reverse: create index "idx_automation_runs_job" to table: "automation_runs"
DROP INDEX `idx_automation_runs_job`;
-- reverse: create "new_automation_runs" table
DROP TABLE `new_automation_runs`;
-- reverse: create index "uq_automation_jobs_workspace_name" to table: "automation_jobs"
DROP INDEX `uq_automation_jobs_workspace_name`;
-- reverse: create index "uq_automation_jobs_global_name" to table: "automation_jobs"
DROP INDEX `uq_automation_jobs_global_name`;
-- reverse: create index "idx_automation_jobs_loop_target" to table: "automation_jobs"
DROP INDEX `idx_automation_jobs_loop_target`;
-- reverse: create index "idx_automation_jobs_enabled" to table: "automation_jobs"
DROP INDEX `idx_automation_jobs_enabled`;
-- reverse: create "new_automation_jobs" table
DROP TABLE `new_automation_jobs`;

