-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_session_derivations" table
CREATE TABLE `new_session_derivations` (`workspace_id` text NOT NULL, `idempotency_key` text NOT NULL, `profile_id` text NOT NULL, `request_fingerprint` text NOT NULL, `source_session_id` text NOT NULL, `child_session_id` text NOT NULL, `kind` text NOT NULL, `outcome_json` text NOT NULL, `created_at` text NOT NULL, `child_deleted_at` text NULL, CHECK (kind IN ('continue', 'fork')), CHECK (json_valid(outcome_json)));
-- copy rows from old table "session_derivations" to new temporary table "new_session_derivations"
INSERT INTO `new_session_derivations` (`workspace_id`, `idempotency_key`, `profile_id`, `request_fingerprint`, `source_session_id`, `child_session_id`, `kind`, `outcome_json`, `created_at`, `child_deleted_at`) SELECT `workspace_id`, `idempotency_key`, `profile_id`, `request_fingerprint`, `source_session_id`, `child_session_id`, `kind`, `outcome_json`, `created_at`, `child_deleted_at` FROM `session_derivations`;
-- drop trigger "workspace_scope_cleanup_after_delete" before applying its declarative change
DROP TRIGGER IF EXISTS `workspace_scope_cleanup_after_delete`;
-- drop "session_derivations" table after copying rows
DROP TABLE `session_derivations`;
-- rename temporary table "new_session_derivations" to "session_derivations"
ALTER TABLE `new_session_derivations` RENAME TO `session_derivations`;
-- create index "idx_session_derivations_project_key" to table: "session_derivations"
CREATE UNIQUE INDEX `idx_session_derivations_project_key` ON `session_derivations` (`workspace_id`, `idempotency_key`) WHERE workspace_id <> '';
-- create index "idx_session_derivations_user_key" to table: "session_derivations"
CREATE UNIQUE INDEX `idx_session_derivations_user_key` ON `session_derivations` (`profile_id`, `idempotency_key`) WHERE workspace_id = '';
-- create index "idx_session_derivations_source" to table: "session_derivations"
CREATE INDEX `idx_session_derivations_source` ON `session_derivations` (`source_session_id`);
-- create index "idx_session_derivations_child" to table: "session_derivations"
CREATE INDEX `idx_session_derivations_child` ON `session_derivations` (`child_session_id`);
-- create "new_session_folders" table
CREATE TABLE `new_session_folders` (`id` text NULL, `profile_id` text NOT NULL, `workspace_id` text NOT NULL, `name` text NOT NULL, `position` integer NOT NULL, `created_at` text NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (length(trim(id)) > 0), CHECK (length(trim(name)) > 0 AND length(name) <= 120), CHECK (position >= 0));
-- copy rows from old table "session_folders" to new temporary table "new_session_folders"
INSERT INTO `new_session_folders` (`id`, `profile_id`, `workspace_id`, `name`, `position`, `created_at`, `updated_at`) SELECT `id`, `profile_id`, `workspace_id`, `name`, `position`, `created_at`, `updated_at` FROM `session_folders`;
CREATE TEMP TABLE session_placements_backup AS SELECT * FROM session_placements;
DROP TABLE session_placements;
-- drop "session_folders" table after copying rows
DROP TABLE `session_folders`;
-- rename temporary table "new_session_folders" to "session_folders"
ALTER TABLE `new_session_folders` RENAME TO `session_folders`;
-- create index "session_folders_profile_id_workspace_id_id" to table: "session_folders"
CREATE UNIQUE INDEX `session_folders_profile_id_workspace_id_id` ON `session_folders` (`profile_id`, `workspace_id`, `id`);
-- create index "idx_session_folders_scope" to table: "session_folders"
CREATE INDEX `idx_session_folders_scope` ON `session_folders` (`profile_id`, `workspace_id`, `position`);
CREATE TABLE session_placements (
	session_id TEXT PRIMARY KEY,
	profile_id TEXT NOT NULL,
	workspace_id TEXT NOT NULL,
	folder_id TEXT,
	position INTEGER NOT NULL CHECK (position >= 0),
	updated_at TEXT NOT NULL,
	FOREIGN KEY (workspace_id, session_id)
		REFERENCES sessions(workspace_id, id) ON DELETE CASCADE,
	FOREIGN KEY (profile_id, workspace_id, folder_id)
		REFERENCES session_folders(profile_id, workspace_id, id) ON DELETE CASCADE
);

CREATE INDEX idx_session_placements_container
	ON session_placements(profile_id, workspace_id, folder_id, position);

CREATE INDEX idx_session_placements_folder
	ON session_placements(folder_id);

INSERT INTO session_placements SELECT * FROM session_placements_backup;
DROP TABLE session_placements_backup;
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
-- apply declarative trigger "workspace_scope_cleanup_after_delete" on table "workspaces"
-- +goose StatementBegin
CREATE TRIGGER workspace_scope_cleanup_after_delete
AFTER DELETE ON workspaces
BEGIN
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
-- apply declarative trigger "session_folders_workspace_insert_guard" on table "session_folders"
-- +goose StatementBegin
CREATE TRIGGER session_folders_workspace_insert_guard
BEFORE INSERT ON session_folders
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- apply declarative trigger "session_folders_workspace_update_guard" on table "session_folders"
-- +goose StatementBegin
CREATE TRIGGER session_folders_workspace_update_guard
BEFORE UPDATE OF workspace_id ON session_folders
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
