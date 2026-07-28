-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_automation_scheduler_state" table
CREATE TABLE `new_automation_scheduler_state` (`job_id` text NULL, `next_run_at` text NULL, `last_run_at` text NULL, `last_scheduled_at` text NULL, `last_fire_id` text NOT NULL DEFAULT '', `schedule_hash` text NOT NULL DEFAULT '', `catch_up_policy` text NOT NULL DEFAULT 'skip_missed', `misfire_grace_seconds` integer NOT NULL DEFAULT 0, `consecutive_resume_failures` integer NOT NULL DEFAULT 0, `last_misfire_at` text NULL, `misfire_count` integer NOT NULL DEFAULT 0, `updated_at` text NOT NULL, PRIMARY KEY (`job_id`), CHECK (catch_up_policy IN ('skip_missed', 'coalesce', 'replay', 'run_once_on_catchup')), CHECK (misfire_grace_seconds >= 0), CHECK (consecutive_resume_failures >= 0), CHECK (misfire_count >= 0));
-- copy rows from old table "automation_scheduler_state" to new temporary table "new_automation_scheduler_state"
INSERT INTO `new_automation_scheduler_state` (`job_id`, `next_run_at`, `last_run_at`, `last_scheduled_at`, `last_fire_id`, `schedule_hash`, `catch_up_policy`, `misfire_grace_seconds`, `consecutive_resume_failures`, `last_misfire_at`, `misfire_count`, `updated_at`) SELECT `job_id`, `next_run_at`, `last_run_at`, `last_scheduled_at`, `last_fire_id`, `schedule_hash`, CASE `catch_up_policy` WHEN 'skip' THEN 'skip_missed' ELSE `catch_up_policy` END AS `catch_up_policy`, `misfire_grace_seconds`, `consecutive_resume_failures`, `last_misfire_at`, `misfire_count`, `updated_at` FROM `automation_scheduler_state`;
-- drop "automation_scheduler_state" table after copying rows
DROP TABLE `automation_scheduler_state`;
-- rename temporary table "new_automation_scheduler_state" to "automation_scheduler_state"
ALTER TABLE `new_automation_scheduler_state` RENAME TO `automation_scheduler_state`;
-- create index "idx_automation_scheduler_misfire" to table: "automation_scheduler_state"
CREATE INDEX `idx_automation_scheduler_misfire` ON `automation_scheduler_state` (`last_misfire_at`);
-- create index "idx_automation_scheduler_next_run" to table: "automation_scheduler_state"
CREATE INDEX `idx_automation_scheduler_next_run` ON `automation_scheduler_state` (`next_run_at`);
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
