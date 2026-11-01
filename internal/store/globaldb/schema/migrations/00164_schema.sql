-- +goose Up
-- add column "instance_id" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `instance_id` text NULL;
-- create "agent_instances" table
CREATE TABLE `agent_instances` (`id` text NULL, `profile_id` text NOT NULL, `agent_name` text NOT NULL, `workspace_id` text NOT NULL DEFAULT '', `state` text NOT NULL, `pause_reason` text NOT NULL DEFAULT '', `main_session_id` text NULL, `grant_ceiling` text NOT NULL DEFAULT '', `permission_ceiling` text NOT NULL DEFAULT '{}', `checks_enabled` integer NOT NULL DEFAULT 1, `depth` integer NOT NULL DEFAULT 0, `created_by_kind` text NOT NULL, `created_by_id` text NOT NULL, `created_at` text NOT NULL, `updated_at` text NOT NULL, `archived_at` text NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`main_session_id`) REFERENCES `sessions` (`id`) ON UPDATE NO ACTION ON DELETE SET NULL, CONSTRAINT `1` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (state IN ('starting', 'active', 'paused')), CHECK (grant_ceiling IN ('', 'unrestricted')), CHECK (json_valid(permission_ceiling)), CHECK (checks_enabled IN (0, 1)), CHECK (depth IN (0, 1)));
-- create index "idx_agent_instances_live_identity" to table: "agent_instances"
CREATE UNIQUE INDEX `idx_agent_instances_live_identity` ON `agent_instances` (`profile_id`, `agent_name`, `workspace_id`) WHERE archived_at IS NULL;
-- create index "idx_agent_instances_main_session" to table: "agent_instances"
CREATE INDEX `idx_agent_instances_main_session` ON `agent_instances` (`main_session_id`);
-- create index "idx_agent_instances_workspace" to table: "agent_instances"
CREATE INDEX `idx_agent_instances_workspace` ON `agent_instances` (`workspace_id`);
-- create "agent_instance_reports" table
CREATE TABLE `agent_instance_reports` (`id` text NULL, `instance_id` text NOT NULL, `source_key` text NOT NULL, `kind` text NOT NULL, `payload` text NOT NULL, `fingerprint` text NOT NULL DEFAULT '', `state` text NOT NULL DEFAULT 'pending', `prompt_admission_id` text NOT NULL DEFAULT '', `event_session_id` text NOT NULL DEFAULT '', `event_seq` integer NOT NULL DEFAULT 0, `created_at` text NOT NULL, `delivered_at` text NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`instance_id`) REFERENCES `agent_instances` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (kind IN ('proposal', 'failure', 'approval_expired', 'interrupted', 'rotation_failed', 'rotation_deferred')), CHECK (json_valid(payload)), CHECK (state IN ('pending', 'reserved', 'delivered', 'indeterminate')));
-- create index "agent_instance_reports_source_key" to table: "agent_instance_reports"
CREATE UNIQUE INDEX `agent_instance_reports_source_key` ON `agent_instance_reports` (`source_key`);
-- create index "idx_agent_instance_reports_history" to table: "agent_instance_reports"
CREATE INDEX `idx_agent_instance_reports_history` ON `agent_instance_reports` (`instance_id`, `created_at`, `id`);
-- create index "idx_agent_instance_reports_pending" to table: "agent_instance_reports"
CREATE INDEX `idx_agent_instance_reports_pending` ON `agent_instance_reports` (`instance_id`, `state`, `created_at`, `id`);
-- apply declarative trigger "agent_instances_workspace_insert_guard" on table "agent_instances"
-- +goose StatementBegin
CREATE TRIGGER agent_instances_workspace_insert_guard
BEFORE INSERT ON agent_instances
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
    SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- apply declarative trigger "agent_instances_workspace_update_guard" on table "agent_instances"
-- +goose StatementBegin
CREATE TRIGGER agent_instances_workspace_update_guard
BEFORE UPDATE OF workspace_id ON agent_instances
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
    SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
