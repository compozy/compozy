-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_agent_instances" table
CREATE TABLE `new_agent_instances` (`id` text NULL, `profile_id` text NOT NULL, `agent_name` text NOT NULL, `workspace_id` text NOT NULL DEFAULT '', `state` text NOT NULL, `pause_reason` text NOT NULL DEFAULT '', `main_session_id` text NULL, `grant_ceiling` text NOT NULL DEFAULT '', `permission_ceiling` text NOT NULL DEFAULT '{}', `checks_enabled` integer NOT NULL DEFAULT 1, `depth` integer NOT NULL DEFAULT 0, `created_by_kind` text NOT NULL, `created_by_id` text NOT NULL, `created_at` text NOT NULL, `updated_at` text NOT NULL, `archived_at` text NULL, `skill_auto_approve` integer NOT NULL DEFAULT 0, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`main_session_id`) REFERENCES `sessions` (`id`) ON UPDATE NO ACTION ON DELETE SET NULL, CONSTRAINT `1` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (state IN ('starting', 'active', 'paused')), CHECK (grant_ceiling IN ('', 'unrestricted')), CHECK (json_valid(permission_ceiling)), CHECK (checks_enabled IN (0, 1)), CHECK (depth IN (0, 1)), CHECK (skill_auto_approve IN (0, 1)));
-- copy rows from old table "agent_instances" to new temporary table "new_agent_instances"
INSERT INTO `new_agent_instances` (`id`, `profile_id`, `agent_name`, `workspace_id`, `state`, `pause_reason`, `main_session_id`, `grant_ceiling`, `permission_ceiling`, `checks_enabled`, `depth`, `created_by_kind`, `created_by_id`, `created_at`, `updated_at`, `archived_at`) SELECT `id`, `profile_id`, `agent_name`, `workspace_id`, `state`, `pause_reason`, `main_session_id`, `grant_ceiling`, `permission_ceiling`, `checks_enabled`, `depth`, `created_by_kind`, `created_by_id`, `created_at`, `updated_at`, `archived_at` FROM `agent_instances`;
-- drop trigger "agent_instances_workspace_insert_guard" before applying its declarative change
DROP TRIGGER IF EXISTS `agent_instances_workspace_insert_guard`;
-- drop trigger "agent_instances_workspace_update_guard" before applying its declarative change
DROP TRIGGER IF EXISTS `agent_instances_workspace_update_guard`;
-- drop "agent_instances" table after copying rows
DROP TABLE `agent_instances`;
-- rename temporary table "new_agent_instances" to "agent_instances"
ALTER TABLE `new_agent_instances` RENAME TO `agent_instances`;
-- create index "idx_agent_instances_live_identity" to table: "agent_instances"
CREATE UNIQUE INDEX `idx_agent_instances_live_identity` ON `agent_instances` (`profile_id`, `agent_name`, `workspace_id`) WHERE archived_at IS NULL;
-- create index "idx_agent_instances_main_session" to table: "agent_instances"
CREATE INDEX `idx_agent_instances_main_session` ON `agent_instances` (`main_session_id`);
-- create index "idx_agent_instances_workspace" to table: "agent_instances"
CREATE INDEX `idx_agent_instances_workspace` ON `agent_instances` (`workspace_id`);
-- create index "idx_agent_instances_activity" to table: "agent_instances"
CREATE INDEX `idx_agent_instances_activity` ON `agent_instances` (`profile_id`, `agent_name`, `workspace_id`);
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
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
