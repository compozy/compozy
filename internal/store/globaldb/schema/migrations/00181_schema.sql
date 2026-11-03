-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_automation_triggers" table
CREATE TABLE `new_automation_triggers` (`grant_ceiling` text NOT NULL DEFAULT '', `id` text NULL, `profile_id` text NOT NULL, `scope` text NOT NULL, `name` text NOT NULL, `agent_name` text NOT NULL, `workspace_id` text NULL, `prompt` text NOT NULL, `event` text NOT NULL, `filter` text NULL, `enabled` boolean NOT NULL DEFAULT 1, `retry` text NOT NULL, `fire_limit` text NOT NULL, `source` text NOT NULL DEFAULT 'dynamic', `webhook_id` text NULL, `endpoint_slug` text NULL, `webhook_secret_ref` text NULL, `target_kind` text NOT NULL DEFAULT 'agent', `target_instance_scope` text NOT NULL DEFAULT '', `loop_workspace_id` text NULL, `loop_name` text NULL, `loop_inputs` text NULL, `loop_input_mapping` text NULL, `created_at` text NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`loop_workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `2` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (scope IN ('global', 'workspace')), CHECK (target_kind IN ('agent', 'loop', 'agent_instance')), CHECK (
			(scope = 'global' AND workspace_id IS NULL) OR
			(scope = 'workspace' AND workspace_id IS NOT NULL)
		));
-- copy rows from old table "automation_triggers" to new temporary table "new_automation_triggers"
INSERT INTO `new_automation_triggers` (`grant_ceiling`, `id`, `profile_id`, `scope`, `name`, `agent_name`, `workspace_id`, `prompt`, `event`, `filter`, `enabled`, `retry`, `fire_limit`, `source`, `webhook_id`, `endpoint_slug`, `webhook_secret_ref`, `target_kind`, `loop_workspace_id`, `loop_name`, `loop_inputs`, `loop_input_mapping`, `created_at`, `updated_at`) SELECT `grant_ceiling`, `id`, `profile_id`, `scope`, `name`, `agent_name`, `workspace_id`, `prompt`, `event`, `filter`, `enabled`, `retry`, `fire_limit`, `source`, `webhook_id`, `endpoint_slug`, `webhook_secret_ref`, `target_kind`, `loop_workspace_id`, `loop_name`, `loop_inputs`, `loop_input_mapping`, `created_at`, `updated_at` FROM `automation_triggers`;
-- drop trigger "automation_triggers_profile_owner_active" before applying its declarative change
DROP TRIGGER IF EXISTS `automation_triggers_profile_owner_active`;
-- drop trigger "automation_triggers_profile_owner_immutable" before applying its declarative change
DROP TRIGGER IF EXISTS `automation_triggers_profile_owner_immutable`;
-- drop trigger "automation_watch_events_after_insert" before applying its declarative change
DROP TRIGGER IF EXISTS `automation_watch_events_after_insert`;
-- drop trigger "automation_watch_events_after_terminal_update" before applying its declarative change
DROP TRIGGER IF EXISTS `automation_watch_events_after_terminal_update`;
-- drop "automation_triggers" table after copying rows
DROP TABLE `automation_triggers`;
-- rename temporary table "new_automation_triggers" to "automation_triggers"
ALTER TABLE `new_automation_triggers` RENAME TO `automation_triggers`;
-- create index "idx_automation_triggers_enabled" to table: "automation_triggers"
CREATE INDEX `idx_automation_triggers_enabled` ON `automation_triggers` (`enabled`);
-- create index "idx_automation_triggers_event" to table: "automation_triggers"
CREATE INDEX `idx_automation_triggers_event` ON `automation_triggers` (`event`);
-- create index "idx_automation_triggers_profile" to table: "automation_triggers"
CREATE INDEX `idx_automation_triggers_profile` ON `automation_triggers` (`profile_id`, `id`);
-- create index "idx_automation_triggers_loop_target" to table: "automation_triggers"
CREATE INDEX `idx_automation_triggers_loop_target` ON `automation_triggers` (`loop_name`, `loop_workspace_id`) WHERE target_kind = 'loop';
-- create index "uq_automation_triggers_global_name" to table: "automation_triggers"
CREATE UNIQUE INDEX `uq_automation_triggers_global_name` ON `automation_triggers` (`name`) WHERE scope = 'global';
-- create index "uq_automation_triggers_webhook_id" to table: "automation_triggers"
CREATE UNIQUE INDEX `uq_automation_triggers_webhook_id` ON `automation_triggers` (`webhook_id`) WHERE webhook_id IS NOT NULL;
-- create index "uq_automation_triggers_workspace_name" to table: "automation_triggers"
CREATE UNIQUE INDEX `uq_automation_triggers_workspace_name` ON `automation_triggers` (`workspace_id`, `name`) WHERE scope = 'workspace';
-- create "new_agent_instance_wake_state" table
CREATE TABLE `new_agent_instance_wake_state` (`instance_id` text NULL, `next_wake_at` text NOT NULL DEFAULT '', `last_wake_at` text NOT NULL DEFAULT '', `last_digest` text NOT NULL DEFAULT '', `last_digest_at` text NOT NULL DEFAULT '', `automation_cursor` text NOT NULL DEFAULT '', `unattended_turns` integer NOT NULL DEFAULT 0, `engaged_at` text NOT NULL DEFAULT '', `events_omitted` integer NOT NULL DEFAULT 0, `quiet_streak` integer NOT NULL DEFAULT 0, `failure_streak` integer NOT NULL DEFAULT 0, `occurrence_id` text NOT NULL DEFAULT '', `triage_session_id` text NOT NULL DEFAULT '', `occurrence_started_at` text NOT NULL DEFAULT '', `rotation_due_session` text NOT NULL DEFAULT '', `occurrence_cancelled` integer NOT NULL DEFAULT 0, PRIMARY KEY (`instance_id`), CONSTRAINT `0` FOREIGN KEY (`instance_id`) REFERENCES `agent_instances` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (unattended_turns >= 0), CHECK (events_omitted >= 0), CHECK (quiet_streak >= 0), CHECK (failure_streak >= 0), CHECK (occurrence_cancelled IN (0, 1)));
-- copy rows from old table "agent_instance_wake_state" to new temporary table "new_agent_instance_wake_state"
INSERT INTO `new_agent_instance_wake_state` (`instance_id`, `next_wake_at`, `last_wake_at`, `last_digest`, `last_digest_at`, `automation_cursor`, `unattended_turns`, `engaged_at`, `quiet_streak`, `failure_streak`, `occurrence_id`, `triage_session_id`, `occurrence_started_at`, `rotation_due_session`, `occurrence_cancelled`) SELECT `instance_id`, `next_wake_at`, `last_wake_at`, `last_digest`, `last_digest_at`, `automation_cursor`, `unattended_turns`, `engaged_at`, `quiet_streak`, `failure_streak`, `occurrence_id`, `triage_session_id`, `occurrence_started_at`, `rotation_due_session`, `occurrence_cancelled` FROM `agent_instance_wake_state`;
-- drop "agent_instance_wake_state" table after copying rows
DROP TABLE `agent_instance_wake_state`;
-- rename temporary table "new_agent_instance_wake_state" to "agent_instance_wake_state"
ALTER TABLE `new_agent_instance_wake_state` RENAME TO `agent_instance_wake_state`;
-- create index "idx_agent_instance_wake_state_due" to table: "agent_instance_wake_state"
CREATE INDEX `idx_agent_instance_wake_state_due` ON `agent_instance_wake_state` (`next_wake_at`, `instance_id`);
-- create "agent_skill_reviews" table
CREATE TABLE `agent_skill_reviews` (`id` text NULL, `instance_id` text NOT NULL, `profile_id` text NOT NULL, `workspace_id` text NOT NULL DEFAULT '', `agent_name` text NOT NULL DEFAULT '', `skill_name` text NOT NULL, `digest` text NOT NULL, `state` text NOT NULL, `draft_path` text NOT NULL, `source_session_id` text NULL, `decided_by` text NOT NULL DEFAULT '', `created_at` text NOT NULL, `decided_at` text NULL, PRIMARY KEY (`id`), CHECK (state IN ('pending', 'approved', 'rejected')));
-- create index "idx_agent_skill_reviews_instance" to table: "agent_skill_reviews"
CREATE INDEX `idx_agent_skill_reviews_instance` ON `agent_skill_reviews` (`instance_id`, `state`, `created_at`, `id`);
-- create index "idx_agent_skill_reviews_scope" to table: "agent_skill_reviews"
CREATE INDEX `idx_agent_skill_reviews_scope` ON `agent_skill_reviews` (`profile_id`, `workspace_id`, `created_at`, `id`);
-- create "agent_instance_events" table
CREATE TABLE `agent_instance_events` (`id` text NULL, `instance_id` text NOT NULL, `trigger_id` text NOT NULL, `run_id` text NOT NULL, `summary` text NOT NULL, `received_at` text NOT NULL, `consumed_at` text NULL, `occurrence_id` text NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`instance_id`) REFERENCES `agent_instances` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE);
-- create index "agent_instance_events_run_id" to table: "agent_instance_events"
CREATE UNIQUE INDEX `agent_instance_events_run_id` ON `agent_instance_events` (`run_id`);
-- create index "idx_agent_instance_events_unread" to table: "agent_instance_events"
CREATE INDEX `idx_agent_instance_events_unread` ON `agent_instance_events` (`instance_id`, `consumed_at`, `received_at`, `id`);
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
-- apply declarative trigger "automation_triggers_profile_owner_active" on table "automation_triggers"
-- +goose StatementBegin
CREATE TRIGGER automation_triggers_profile_owner_active BEFORE INSERT ON automation_triggers BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd
-- apply declarative trigger "automation_triggers_profile_owner_immutable" on table "automation_triggers"
-- +goose StatementBegin
CREATE TRIGGER automation_triggers_profile_owner_immutable BEFORE UPDATE OF profile_id ON automation_triggers
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
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
