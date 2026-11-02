-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_agent_instance_reports" table
CREATE TABLE `new_agent_instance_reports` (`id` text NULL, `instance_id` text NOT NULL, `source_key` text NOT NULL, `kind` text NOT NULL, `payload` text NOT NULL, `fingerprint` text NOT NULL DEFAULT '', `state` text NOT NULL DEFAULT 'pending', `prompt_admission_id` text NOT NULL DEFAULT '', `event_session_id` text NOT NULL DEFAULT '', `event_seq` integer NOT NULL DEFAULT 0, `created_at` text NOT NULL, `delivered_at` text NULL, `recovery_noted` integer NOT NULL DEFAULT 0, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`instance_id`) REFERENCES `agent_instances` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (kind IN ('proposal', 'failure', 'approval_expired', 'interrupted', 'rotation_failed', 'rotation_deferred')), CHECK (json_valid(payload)), CHECK (state IN ('pending', 'reserved', 'delivered', 'indeterminate')), CHECK (recovery_noted IN (0, 1)));
-- copy rows from old table "agent_instance_reports" to new temporary table "new_agent_instance_reports"
INSERT INTO `new_agent_instance_reports` (`id`, `instance_id`, `source_key`, `kind`, `payload`, `fingerprint`, `state`, `prompt_admission_id`, `event_session_id`, `event_seq`, `created_at`, `delivered_at`) SELECT `id`, `instance_id`, `source_key`, `kind`, `payload`, `fingerprint`, `state`, `prompt_admission_id`, `event_session_id`, `event_seq`, `created_at`, `delivered_at` FROM `agent_instance_reports`;
-- drop "agent_instance_reports" table after copying rows
DROP TABLE `agent_instance_reports`;
-- rename temporary table "new_agent_instance_reports" to "agent_instance_reports"
ALTER TABLE `new_agent_instance_reports` RENAME TO `agent_instance_reports`;
-- create index "agent_instance_reports_source_key" to table: "agent_instance_reports"
CREATE UNIQUE INDEX `agent_instance_reports_source_key` ON `agent_instance_reports` (`source_key`);
-- create index "idx_agent_instance_reports_history" to table: "agent_instance_reports"
CREATE INDEX `idx_agent_instance_reports_history` ON `agent_instance_reports` (`instance_id`, `created_at`, `id`);
-- create index "idx_agent_instance_reports_pending" to table: "agent_instance_reports"
CREATE INDEX `idx_agent_instance_reports_pending` ON `agent_instance_reports` (`instance_id`, `state`, `created_at`, `id`);
-- create index "idx_agent_instance_reports_unnoted" to table: "agent_instance_reports"
CREATE INDEX `idx_agent_instance_reports_unnoted` ON `agent_instance_reports` (`id`) WHERE state = 'indeterminate' AND recovery_noted = 0;
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
