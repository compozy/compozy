-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_agent_instance_wake_state" table
CREATE TABLE `new_agent_instance_wake_state` (`instance_id` text NULL, `next_wake_at` text NOT NULL DEFAULT '', `last_wake_at` text NOT NULL DEFAULT '', `last_digest` text NOT NULL DEFAULT '', `last_digest_at` text NOT NULL DEFAULT '', `automation_cursor` text NOT NULL DEFAULT '', `unattended_turns` integer NOT NULL DEFAULT 0, `engaged_at` text NOT NULL DEFAULT '', `quiet_streak` integer NOT NULL DEFAULT 0, `failure_streak` integer NOT NULL DEFAULT 0, `occurrence_id` text NOT NULL DEFAULT '', `triage_session_id` text NOT NULL DEFAULT '', `occurrence_started_at` text NOT NULL DEFAULT '', `rotation_due_session` text NOT NULL DEFAULT '', `occurrence_cancelled` integer NOT NULL DEFAULT 0, PRIMARY KEY (`instance_id`), CONSTRAINT `0` FOREIGN KEY (`instance_id`) REFERENCES `agent_instances` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (unattended_turns >= 0), CHECK (quiet_streak >= 0), CHECK (failure_streak >= 0), CHECK (occurrence_cancelled IN (0, 1)));
-- copy rows from old table "agent_instance_wake_state" to new temporary table "new_agent_instance_wake_state"
INSERT INTO `new_agent_instance_wake_state` (`instance_id`, `next_wake_at`, `last_wake_at`, `last_digest`, `last_digest_at`, `automation_cursor`, `quiet_streak`, `failure_streak`, `occurrence_id`, `triage_session_id`, `occurrence_started_at`, `rotation_due_session`, `occurrence_cancelled`) SELECT `instance_id`, `next_wake_at`, `last_wake_at`, `last_digest`, `last_digest_at`, `automation_cursor`, `quiet_streak`, `failure_streak`, `occurrence_id`, `triage_session_id`, `occurrence_started_at`, `rotation_due_session`, `occurrence_cancelled` FROM `agent_instance_wake_state`;
-- drop "agent_instance_wake_state" table after copying rows
DROP TABLE `agent_instance_wake_state`;
-- rename temporary table "new_agent_instance_wake_state" to "agent_instance_wake_state"
ALTER TABLE `new_agent_instance_wake_state` RENAME TO `agent_instance_wake_state`;
-- create index "idx_agent_instance_wake_state_due" to table: "agent_instance_wake_state"
CREATE INDEX `idx_agent_instance_wake_state_due` ON `agent_instance_wake_state` (`next_wake_at`, `instance_id`);
-- create "agent_instance_paused_runs" table
CREATE TABLE `agent_instance_paused_runs` (`instance_id` text NOT NULL, `run_kind` text NOT NULL, `run_id` text NOT NULL, `paused_at` text NOT NULL, PRIMARY KEY (`instance_id`, `run_kind`, `run_id`), CONSTRAINT `0` FOREIGN KEY (`instance_id`) REFERENCES `agent_instances` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (run_kind IN ('goal', 'loop')));
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
