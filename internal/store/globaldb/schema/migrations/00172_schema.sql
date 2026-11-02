-- +goose Up
-- A snapshot table carries its unique key either as a named index, on homes that passed 00085,
-- or as an autoindex, on homes bootstrapped from definitions. Only a rebuild converges both.
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- drop triggers that reference the rebuilt tables
DROP TRIGGER IF EXISTS `workspace_scope_cleanup_after_delete`;
DROP TRIGGER IF EXISTS `agent_heartbeat_snapshots_workspace_insert_guard`;
DROP TRIGGER IF EXISTS `agent_heartbeat_snapshots_workspace_update_guard`;
DROP TRIGGER IF EXISTS `agent_soul_snapshots_workspace_insert_guard`;
DROP TRIGGER IF EXISTS `agent_soul_snapshots_workspace_update_guard`;
-- add column "profile_id" to table: "agent_heartbeat_revisions"
ALTER TABLE `agent_heartbeat_revisions` ADD COLUMN `profile_id` text NOT NULL DEFAULT '00000000000000000000000000';
-- drop index "idx_agent_heartbeat_revisions_agent_created" from table: "agent_heartbeat_revisions"
DROP INDEX `idx_agent_heartbeat_revisions_agent_created`;
-- create index "idx_agent_heartbeat_revisions_agent_created" to table: "agent_heartbeat_revisions"
CREATE INDEX `idx_agent_heartbeat_revisions_agent_created` ON `agent_heartbeat_revisions` (`profile_id`, `workspace_id`, `agent_name`, `created_at` DESC, `id` DESC);
-- create "new_agent_heartbeat_snapshots" table
CREATE TABLE `new_agent_heartbeat_snapshots` (`id` text NULL, `workspace_id` text NOT NULL, `agent_name` text NOT NULL, `source_path` text NOT NULL, `schema_version` integer NOT NULL DEFAULT 1, `digest` text NOT NULL, `config_digest` text NOT NULL, `body` text NOT NULL, `frontmatter_json` text NOT NULL, `resolved_json` text NOT NULL, `diagnostics_json` text NOT NULL, `created_at` text NOT NULL, `profile_id` text NOT NULL DEFAULT '00000000000000000000000000', PRIMARY KEY (`id`));
-- copy rows from old table "agent_heartbeat_snapshots" to new temporary table "new_agent_heartbeat_snapshots"
INSERT INTO `new_agent_heartbeat_snapshots` (`id`, `workspace_id`, `agent_name`, `source_path`, `schema_version`, `digest`, `config_digest`, `body`, `frontmatter_json`, `resolved_json`, `diagnostics_json`, `created_at`) SELECT `id`, `workspace_id`, `agent_name`, `source_path`, `schema_version`, `digest`, `config_digest`, `body`, `frontmatter_json`, `resolved_json`, `diagnostics_json`, `created_at` FROM `agent_heartbeat_snapshots`;
-- drop "agent_heartbeat_snapshots" table after copying rows
DROP TABLE `agent_heartbeat_snapshots`;
-- rename temporary table "new_agent_heartbeat_snapshots" to "agent_heartbeat_snapshots"
ALTER TABLE `new_agent_heartbeat_snapshots` RENAME TO `agent_heartbeat_snapshots`;
-- create index "idx_agent_heartbeat_snapshots_agent_created" to table: "agent_heartbeat_snapshots"
CREATE INDEX `idx_agent_heartbeat_snapshots_agent_created` ON `agent_heartbeat_snapshots` (`profile_id`, `workspace_id`, `agent_name`, `created_at` DESC, `id` DESC);
-- create index "agent_heartbeat_snapshots_profile_id_workspace_id_agent_name_digest" to table: "agent_heartbeat_snapshots"
CREATE UNIQUE INDEX `agent_heartbeat_snapshots_profile_id_workspace_id_agent_name_digest` ON `agent_heartbeat_snapshots` (`profile_id`, `workspace_id`, `agent_name`, `digest`);
-- add column "profile_id" to table: "agent_soul_revisions"
ALTER TABLE `agent_soul_revisions` ADD COLUMN `profile_id` text NOT NULL DEFAULT '00000000000000000000000000';
-- drop index "idx_agent_soul_revisions_agent" from table: "agent_soul_revisions"
DROP INDEX `idx_agent_soul_revisions_agent`;
-- create index "idx_agent_soul_revisions_agent" to table: "agent_soul_revisions"
CREATE INDEX `idx_agent_soul_revisions_agent` ON `agent_soul_revisions` (`profile_id`, `workspace_id`, `agent_name`, `created_at` DESC);
-- create "new_agent_soul_snapshots" table
CREATE TABLE `new_agent_soul_snapshots` (`id` text NULL, `workspace_id` text NOT NULL, `agent_name` text NOT NULL, `source_path` text NOT NULL, `digest` text NOT NULL, `profile_json` text NOT NULL DEFAULT '{}', `body` text NOT NULL DEFAULT '', `truncated` integer NOT NULL DEFAULT 0, `created_at` text NOT NULL, `profile_id` text NOT NULL DEFAULT '00000000000000000000000000', PRIMARY KEY (`id`), CHECK (truncated IN (0, 1)));
-- copy rows from old table "agent_soul_snapshots" to new temporary table "new_agent_soul_snapshots"
INSERT INTO `new_agent_soul_snapshots` (`id`, `workspace_id`, `agent_name`, `source_path`, `digest`, `profile_json`, `body`, `truncated`, `created_at`) SELECT `id`, `workspace_id`, `agent_name`, `source_path`, `digest`, `profile_json`, `body`, `truncated`, `created_at` FROM `agent_soul_snapshots`;
-- drop "agent_soul_snapshots" table after copying rows
DROP TABLE `agent_soul_snapshots`;
-- rename temporary table "new_agent_soul_snapshots" to "agent_soul_snapshots"
ALTER TABLE `new_agent_soul_snapshots` RENAME TO `agent_soul_snapshots`;
-- create index "idx_agent_soul_snapshots_agent" to table: "agent_soul_snapshots"
CREATE INDEX `idx_agent_soul_snapshots_agent` ON `agent_soul_snapshots` (`profile_id`, `workspace_id`, `agent_name`, `created_at` DESC);
-- create index "agent_soul_snapshots_profile_id_workspace_id_agent_name_digest" to table: "agent_soul_snapshots"
CREATE UNIQUE INDEX `agent_soul_snapshots_profile_id_workspace_id_agent_name_digest` ON `agent_soul_snapshots` (`profile_id`, `workspace_id`, `agent_name`, `digest`);

-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;

-- +goose StatementBegin
CREATE TRIGGER agent_heartbeat_snapshots_workspace_insert_guard
BEFORE INSERT ON agent_heartbeat_snapshots
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER agent_heartbeat_snapshots_workspace_update_guard
BEFORE UPDATE OF workspace_id ON agent_heartbeat_snapshots
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER agent_soul_snapshots_workspace_insert_guard
BEFORE INSERT ON agent_soul_snapshots
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER agent_soul_snapshots_workspace_update_guard
BEFORE UPDATE OF workspace_id ON agent_soul_snapshots
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER workspace_scope_cleanup_after_delete
AFTER DELETE ON workspaces
BEGIN
    DELETE FROM attention_workspace_mutes WHERE workspace_id = OLD.id;
    DELETE FROM tool_approval_pending WHERE workspace_id = OLD.id;
	DELETE FROM loop_ui_annotations WHERE workspace_id = OLD.id;
	DELETE FROM loop_session_bindings WHERE workspace_id = OLD.id;
	DELETE FROM loop_run_events WHERE workspace_id = OLD.id;
	DELETE FROM loop_runs WHERE workspace_id = OLD.id;
	DELETE FROM loop_goal_session_outbox WHERE workspace_id = OLD.id;
	DELETE FROM loop_session_cleanup WHERE workspace_id = OLD.id;
	DELETE FROM loop_admission_claims WHERE workspace_id = OLD.id;
	DELETE FROM loop_node_lane_pauses WHERE workspace_id = OLD.id;
	DELETE FROM loop_node_amendments WHERE workspace_id = OLD.id;
	DELETE FROM loop_timetravel_ops WHERE workspace_id = OLD.id;
	DELETE FROM loop_requests WHERE workspace_id = OLD.id;
	DELETE FROM loop_gate_decisions WHERE workspace_id = OLD.id;
	DELETE FROM loop_definition_snapshots WHERE workspace_id = OLD.id;
	DELETE FROM loop_config WHERE workspace_id = OLD.id;
	DELETE FROM agent_heartbeat_wake_events WHERE workspace_id = OLD.id;
	DELETE FROM agent_heartbeat_wake_state WHERE workspace_id = OLD.id;
	DELETE FROM agent_heartbeat_revisions WHERE workspace_id = OLD.id;
	DELETE FROM agent_heartbeat_snapshots WHERE workspace_id = OLD.id;
	DELETE FROM agent_soul_revisions WHERE workspace_id = OLD.id;
	DELETE FROM agent_soul_snapshots WHERE workspace_id = OLD.id;
	DELETE FROM session_folders WHERE workspace_id = OLD.id;
	DELETE FROM workspace_placements WHERE workspace_id = OLD.id;
	DELETE FROM session_health WHERE workspace_id = OLD.id;
	DELETE FROM session_derivations WHERE workspace_id = OLD.id;
	DELETE FROM sessions WHERE workspace_id = OLD.id;
	DELETE FROM token_usage_daily WHERE workspace_id = OLD.id;
	DELETE FROM event_summaries WHERE workspace_id = OLD.id;
	DELETE FROM tool_approval_grants WHERE workspace_id = OLD.id;
	DELETE FROM dead_entities WHERE workspace_id = OLD.id;
	DELETE FROM notification_cursors WHERE workspace_id = OLD.id;
	DELETE FROM skill_exposures WHERE workspace_id = OLD.id;
END;
-- +goose StatementEnd

-- Preserve ownership recorded in profile-specific authored source paths.
UPDATE agent_soul_snapshots
SET profile_id = COALESCE((
  SELECT id FROM profiles
  WHERE instr('/' || replace(agent_soul_snapshots.source_path, char(92), '/'), '/profiles/' || profiles.name || '/agents/') > 0
), profile_id);
UPDATE agent_soul_revisions
SET profile_id = COALESCE((
  SELECT id FROM profiles
  WHERE instr('/' || replace(agent_soul_revisions.source_path, char(92), '/'), '/profiles/' || profiles.name || '/agents/') > 0
), profile_id);
UPDATE agent_heartbeat_snapshots
SET profile_id = COALESCE((
  SELECT id FROM profiles
  WHERE instr('/' || replace(agent_heartbeat_snapshots.source_path, char(92), '/'), '/profiles/' || profiles.name || '/agents/') > 0
), profile_id);
UPDATE agent_heartbeat_revisions
SET profile_id = COALESCE((
  SELECT id FROM profiles
  WHERE instr('/' || replace(agent_heartbeat_revisions.source_path, char(92), '/'), '/profiles/' || profiles.name || '/agents/') > 0
), profile_id);
