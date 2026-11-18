-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_agent_instance_reports" table
CREATE TABLE `new_agent_instance_reports` (`id` text NULL, `instance_id` text NOT NULL, `source_key` text NOT NULL, `kind` text NOT NULL, `payload` text NOT NULL, `fingerprint` text NOT NULL DEFAULT '', `state` text NOT NULL DEFAULT 'pending', `prompt_admission_id` text NOT NULL DEFAULT '', `event_session_id` text NOT NULL DEFAULT '', `event_seq` integer NOT NULL DEFAULT 0, `created_at` text NOT NULL, `delivered_at` text NULL, `recovery_noted` integer NOT NULL DEFAULT 0, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`instance_id`) REFERENCES `agent_instances` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (kind IN ('proposal', 'failure', 'approval_expired', 'interrupted', 'rotation_failed', 'rotation_deferred', 'activity')), CHECK (json_valid(payload)), CHECK (state IN ('pending', 'reserved', 'delivered', 'indeterminate')), CHECK (recovery_noted IN (0, 1)));
-- copy rows from old table "agent_instance_reports" to new temporary table "new_agent_instance_reports"
INSERT INTO `new_agent_instance_reports` (`id`, `instance_id`, `source_key`, `kind`, `payload`, `fingerprint`, `state`, `prompt_admission_id`, `event_session_id`, `event_seq`, `created_at`, `delivered_at`, `recovery_noted`) SELECT `id`, `instance_id`, `source_key`, `kind`, `payload`, `fingerprint`, `state`, `prompt_admission_id`, `event_session_id`, `event_seq`, `created_at`, `delivered_at`, `recovery_noted` FROM `agent_instance_reports`;
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
-- add column "tools_status" to table: "agent_heartbeat_wake_events"
ALTER TABLE `agent_heartbeat_wake_events` ADD COLUMN `tools_status` text NOT NULL DEFAULT 'unavailable';
-- create "agent_instance_check_tool_calls" table
CREATE TABLE `agent_instance_check_tool_calls` (`instance_id` text NOT NULL, `occurrence_id` text NOT NULL, `seq` integer NOT NULL, `tool_id` text NOT NULL, `backend` text NOT NULL, `mcp_server` text NULL, `access` text NOT NULL, `decision` text NOT NULL, `deny_reason` text NULL, `outcome` text NOT NULL, `effect` text NOT NULL, `error_code` text NULL, `args_preview` text NULL, `duration_ms` integer NULL, `created_at` text NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`occurrence_id`, `seq`), CONSTRAINT `0` FOREIGN KEY (`instance_id`) REFERENCES `agent_instances` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (backend IN ('native','mcp')), CHECK (access IN ('read','write')), CHECK (decision IN ('allowed','denied')), CHECK (outcome IN ('running','ok','error','cancelled','denied','unknown')), CHECK (effect IN ('none','done','maybe')));
-- create index "idx_agent_instance_check_tool_calls_history" to table: "agent_instance_check_tool_calls"
CREATE INDEX `idx_agent_instance_check_tool_calls_history` ON `agent_instance_check_tool_calls` (`instance_id`, `created_at`);
-- create "agent_instance_proposal_picks" table
CREATE TABLE `agent_instance_proposal_picks` (`report_id` text NOT NULL, `proposal_index` integer NOT NULL, `instance_id` text NOT NULL, `action_id` text NULL, `actor` text NOT NULL, `target_session_id` text NOT NULL, `queue_entry_id` text NULL, `picked_at` text NOT NULL, PRIMARY KEY (`report_id`, `proposal_index`), CONSTRAINT `0` FOREIGN KEY (`queue_entry_id`) REFERENCES `session_input_queue` (`id`) ON UPDATE NO ACTION ON DELETE SET NULL, CONSTRAINT `1` FOREIGN KEY (`report_id`) REFERENCES `agent_instance_reports` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (actor IN ('operator','auto_accept')));
-- create index "idx_agent_instance_proposal_picks_daily" to table: "agent_instance_proposal_picks"
CREATE INDEX `idx_agent_instance_proposal_picks_daily` ON `agent_instance_proposal_picks` (`instance_id`, `actor`, `picked_at`);
-- create "heartbeat_authority_approvals" table
CREATE TABLE `heartbeat_authority_approvals` (`notified_sha256` text NOT NULL DEFAULT '', `profile_id` text NOT NULL, `definition_dir` text NOT NULL, `sha256` text NOT NULL, `approved_at` text NOT NULL, `approved_via` text NOT NULL, PRIMARY KEY (`profile_id`, `definition_dir`), CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (approved_via IN ('cli','web','http','uds')));
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
