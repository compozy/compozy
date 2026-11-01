-- +goose Up
-- create "session_monitors" table
CREATE TABLE `session_monitors` (`id` text NOT NULL, `session_id` text NOT NULL, `workspace_id` text NOT NULL, `profile_id` text NOT NULL, `kind` text NOT NULL, `description` text NOT NULL, `terminal_id` text NOT NULL DEFAULT '', `until` text NOT NULL DEFAULT '', `pattern` text NOT NULL DEFAULT '', `timeout_ms` integer NOT NULL DEFAULT 0, `status` text NOT NULL, `created_at` text NOT NULL, `deadline_at` text NOT NULL, `fired_at` text NOT NULL DEFAULT '', `reason` text NOT NULL DEFAULT '', `exit_code` integer NULL, `owner_binding` text NOT NULL DEFAULT '', `version` integer NOT NULL, PRIMARY KEY (`id`), CHECK (kind IN ('terminal', 'timer')), CHECK (status IN ('active', 'delivering', 'fired', 'canceled', 'undelivered', 'interrupted')));
-- create index "idx_session_monitors_session" to table: "session_monitors"
CREATE INDEX `idx_session_monitors_session` ON `session_monitors` (`session_id`);
-- create index "idx_session_monitors_status" to table: "session_monitors"
CREATE INDEX `idx_session_monitors_status` ON `session_monitors` (`status`, `fired_at`);
