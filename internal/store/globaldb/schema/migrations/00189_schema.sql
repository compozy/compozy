-- +goose Up
-- create "event_summary_export" table
CREATE TABLE `event_summary_export` (`seq` integer NULL PRIMARY KEY AUTOINCREMENT, `id` text NOT NULL, `profile_id` text NOT NULL, `session_id` text NOT NULL DEFAULT '', `workspace_id` text NOT NULL DEFAULT '', `worktree_id` text NOT NULL DEFAULT '', `type` text NOT NULL, `agent_name` text NOT NULL DEFAULT '', `content_json` text NOT NULL DEFAULT '', `task_id` text NOT NULL DEFAULT '', `run_id` text NOT NULL DEFAULT '', `workflow_id` text NOT NULL DEFAULT '', `claim_token_hash` text NOT NULL DEFAULT '', `lease_until` text NOT NULL DEFAULT '', `coordinator_session_id` text NOT NULL DEFAULT '', `scheduler_reason` text NOT NULL DEFAULT '', `hook_event` text NOT NULL DEFAULT '', `hook_name` text NOT NULL DEFAULT '', `actor_kind` text NOT NULL DEFAULT '', `actor_id` text NOT NULL DEFAULT '', `release_reason` text NOT NULL DEFAULT '', `parent_session_id` text NOT NULL DEFAULT '', `root_session_id` text NOT NULL DEFAULT '', `spawn_depth` integer NOT NULL DEFAULT 0, `summary` text NULL, `timestamp` text NOT NULL, `provider` text NOT NULL DEFAULT '', `outcome` text NOT NULL DEFAULT 'info', CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "event_summary_export_id" to table: "event_summary_export"
CREATE UNIQUE INDEX `event_summary_export_id` ON `event_summary_export` (`id`);
-- create "telemetry_deleted_scopes" table
CREATE TABLE `telemetry_deleted_scopes` (`kind` text NOT NULL, `scope_id` text NOT NULL, PRIMARY KEY (`kind`, `scope_id`), CHECK (kind IN ('profile', 'workspace')));
-- drop trigger "event_summaries_profile_owner_active" before applying its declarative change
DROP TRIGGER IF EXISTS `event_summaries_profile_owner_active`;
-- drop trigger "event_summaries_profile_owner_immutable" before applying its declarative change
DROP TRIGGER IF EXISTS `event_summaries_profile_owner_immutable`;
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
	INSERT OR IGNORE INTO telemetry_deleted_scopes(kind, scope_id) VALUES ('workspace', OLD.id);
	DELETE FROM tool_approval_grants WHERE workspace_id = OLD.id;
	DELETE FROM dead_entities WHERE workspace_id = OLD.id;
	DELETE FROM notification_cursors WHERE workspace_id = OLD.id;
	DELETE FROM skill_exposures WHERE workspace_id = OLD.id;
END;
-- +goose StatementEnd
-- apply declarative trigger "telemetry_profile_deleted" on table "profiles"
-- +goose StatementBegin
CREATE TRIGGER telemetry_profile_deleted AFTER DELETE ON profiles BEGIN
    INSERT OR IGNORE INTO telemetry_deleted_scopes(kind, scope_id) VALUES ('profile', OLD.id);
END;
-- +goose StatementEnd
