-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_session_monitors" table
CREATE TABLE `new_session_monitors` (`id` text NOT NULL, `session_id` text NOT NULL, `workspace_id` text NOT NULL, `profile_id` text NOT NULL, `kind` text NOT NULL, `description` text NOT NULL, `terminal_id` text NOT NULL DEFAULT '', `until` text NOT NULL DEFAULT '', `pattern` text NOT NULL DEFAULT '', `timeout_ms` integer NOT NULL DEFAULT 0, `status` text NOT NULL, `created_at` text NOT NULL, `deadline_at` text NOT NULL, `fired_at` text NOT NULL DEFAULT '', `reason` text NOT NULL DEFAULT '', `exit_code` integer NULL, `owner_binding` text NOT NULL DEFAULT '', `owner_check_failed_at` text NOT NULL DEFAULT '', `version` integer NOT NULL, `branch` text NOT NULL DEFAULT '', `repo_key` text NOT NULL DEFAULT '', `pr_cursor` text NOT NULL DEFAULT '', `fire_count` integer NOT NULL DEFAULT 0, `last_fired_at` text NOT NULL DEFAULT '', `target_session_ids` text NOT NULL DEFAULT '[]', `states` text NOT NULL DEFAULT '[]', `targets` text NOT NULL DEFAULT '[]', PRIMARY KEY (`id`), CHECK (kind IN ('terminal', 'timer', 'pull_request', 'session')), CHECK (status IN ('active', 'delivering', 'fired', 'canceled', 'undelivered', 'interrupted')));
-- copy rows from old table "session_monitors" to new temporary table "new_session_monitors"
INSERT INTO `new_session_monitors` (`id`, `session_id`, `workspace_id`, `profile_id`, `kind`, `description`, `terminal_id`, `until`, `pattern`, `timeout_ms`, `status`, `created_at`, `deadline_at`, `fired_at`, `reason`, `exit_code`, `owner_binding`, `owner_check_failed_at`, `version`, `target_session_ids`, `states`, `targets`) SELECT `id`, `session_id`, `workspace_id`, `profile_id`, `kind`, `description`, `terminal_id`, `until`, `pattern`, `timeout_ms`, `status`, `created_at`, `deadline_at`, `fired_at`, `reason`, `exit_code`, `owner_binding`, `owner_check_failed_at`, `version`, `target_session_ids`, `states`, `targets` FROM `session_monitors`;
-- drop "session_monitors" table after copying rows
DROP TABLE `session_monitors`;
-- rename temporary table "new_session_monitors" to "session_monitors"
ALTER TABLE `new_session_monitors` RENAME TO `session_monitors`;
-- create index "idx_session_monitors_session" to table: "session_monitors"
CREATE INDEX `idx_session_monitors_session` ON `session_monitors` (`session_id`);
-- create index "idx_session_monitors_status" to table: "session_monitors"
CREATE INDEX `idx_session_monitors_status` ON `session_monitors` (`status`, `fired_at`);
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
