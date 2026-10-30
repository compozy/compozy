-- +goose Up
-- create "workspace_folders" table
CREATE TABLE `workspace_folders` (`id` text NULL, `profile_id` text NOT NULL, `name` text NOT NULL, `position` integer NOT NULL, `created_at` text NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (length(trim(id)) > 0), CHECK (length(trim(name)) > 0 AND length(name) <= 120), CHECK (position >= 0));
-- create index "workspace_folders_profile_id_id" to table: "workspace_folders"
CREATE UNIQUE INDEX `workspace_folders_profile_id_id` ON `workspace_folders` (`profile_id`, `id`);
-- create index "idx_workspace_folders_profile" to table: "workspace_folders"
CREATE INDEX `idx_workspace_folders_profile` ON `workspace_folders` (`profile_id`, `position`);
-- create "workspace_placements" table
CREATE TABLE `workspace_placements` (`profile_id` text NOT NULL, `workspace_id` text NOT NULL, `folder_id` text NULL, `position` integer NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`profile_id`, `workspace_id`), CONSTRAINT `0` FOREIGN KEY (`profile_id`, `folder_id`) REFERENCES `workspace_folders` (`profile_id`, `id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (position >= 0));
-- create index "idx_workspace_placements_container" to table: "workspace_placements"
CREATE INDEX `idx_workspace_placements_container` ON `workspace_placements` (`profile_id`, `folder_id`, `position`);
-- drop trigger "workspace_scope_cleanup_after_delete" before applying its declarative change
DROP TRIGGER IF EXISTS `workspace_scope_cleanup_after_delete`;
-- apply declarative trigger "workspace_scope_cleanup_after_delete" on table "workspaces"
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
-- apply declarative trigger "workspace_placements_workspace_insert_guard" on table "workspace_placements"
-- +goose StatementBegin
CREATE TRIGGER workspace_placements_workspace_insert_guard
BEFORE INSERT ON workspace_placements
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- apply declarative trigger "workspace_placements_workspace_update_guard" on table "workspace_placements"
-- +goose StatementBegin
CREATE TRIGGER workspace_placements_workspace_update_guard
BEFORE UPDATE OF workspace_id ON workspace_placements
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
