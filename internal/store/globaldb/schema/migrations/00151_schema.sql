-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_tool_approval_pending" table
CREATE TABLE `new_tool_approval_pending` (`approval_id` text NOT NULL, `profile_id` text NOT NULL, `workspace_id` text NULL, `invocation_id` text NOT NULL, `target_kind` text NOT NULL, `tool_id` text NULL, `target_json` text NOT NULL DEFAULT '{}', `command_id` text NULL, `args_json` text NOT NULL, `approval_status` text NOT NULL, `execution_status` text NULL, `result_json` text NULL, `error_json` text NULL, `requested_at` integer NOT NULL, `expires_at` integer NOT NULL, `resolved_at` integer NULL, `executed_at` integer NULL, `resume_fence` integer NOT NULL DEFAULT 0, PRIMARY KEY (`approval_id`), CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (approval_id LIKE 'apr_%'), CHECK (trim(invocation_id) <> ''), CHECK (target_kind IN ('tool', 'client_op', 'navigate', 'view')), CHECK (json_valid(target_json)), CHECK (json_valid(args_json)), CHECK (
		approval_status IN ('pending', 'approved', 'denied', 'timeout', 'canceled')
	), CHECK (
		execution_status IS NULL OR execution_status IN ('dispatching', 'completed', 'failed', 'uncertain')
	), CHECK (result_json IS NULL OR json_valid(result_json)), CHECK (error_json IS NULL OR json_valid(error_json)), CHECK (expires_at > requested_at), CHECK (resume_fence IN (0, 1)), CHECK ((target_kind = 'tool' AND trim(coalesce(tool_id, '')) <> '') OR target_kind <> 'tool'), CHECK ((approval_status = 'pending' AND resolved_at IS NULL) OR (approval_status <> 'pending' AND resolved_at IS NOT NULL)), CHECK ((execution_status IS NULL AND executed_at IS NULL) OR execution_status IS NOT NULL));
-- copy rows from old table "tool_approval_pending" to new temporary table "new_tool_approval_pending"
INSERT INTO `new_tool_approval_pending` (`approval_id`, `profile_id`, `workspace_id`, `invocation_id`, `target_kind`, `tool_id`, `target_json`, `command_id`, `args_json`, `approval_status`, `execution_status`, `result_json`, `error_json`, `requested_at`, `expires_at`, `resolved_at`, `executed_at`, `resume_fence`) SELECT `approval_id`, `profile_id`, `workspace_id`, `invocation_id`, `target_kind`, `tool_id`, `target_json`, `command_id`, `args_json`, `approval_status`, `execution_status`, `result_json`, `error_json`, `requested_at`, `expires_at`, `resolved_at`, `executed_at`, `resume_fence` FROM `tool_approval_pending`;
-- drop trigger "tool_approval_pending_profile_owner_active" before applying its declarative change
DROP TRIGGER IF EXISTS `tool_approval_pending_profile_owner_active`;
-- drop trigger "tool_approval_pending_profile_owner_immutable" before applying its declarative change
DROP TRIGGER IF EXISTS `tool_approval_pending_profile_owner_immutable`;
-- drop trigger "workspace_scope_cleanup_after_delete" before applying its declarative change
DROP TRIGGER IF EXISTS `workspace_scope_cleanup_after_delete`;
-- drop "tool_approval_pending" table after copying rows
DROP TABLE `tool_approval_pending`;
-- rename temporary table "new_tool_approval_pending" to "tool_approval_pending"
ALTER TABLE `new_tool_approval_pending` RENAME TO `tool_approval_pending`;
-- create index "tool_approval_pending_invocation_id" to table: "tool_approval_pending"
CREATE UNIQUE INDEX `tool_approval_pending_invocation_id` ON `tool_approval_pending` (`invocation_id`);
-- create index "idx_tool_approval_pending_workspace_status" to table: "tool_approval_pending"
CREATE INDEX `idx_tool_approval_pending_workspace_status` ON `tool_approval_pending` (`workspace_id`, `approval_status`, `expires_at`, `approval_id`);
-- create index "idx_tool_approval_pending_recovery" to table: "tool_approval_pending"
CREATE INDEX `idx_tool_approval_pending_recovery` ON `tool_approval_pending` (`approval_status`, `execution_status`, `resume_fence`, `expires_at`);
-- create "new_attention_workspace_mutes" table
CREATE TABLE `new_attention_workspace_mutes` (`profile_id` text NOT NULL, `workspace_id` text NOT NULL, PRIMARY KEY (`profile_id`, `workspace_id`), CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE);
-- copy rows from old table "attention_workspace_mutes" to new temporary table "new_attention_workspace_mutes"
INSERT INTO `new_attention_workspace_mutes` (`profile_id`, `workspace_id`) SELECT `profile_id`, `workspace_id` FROM `attention_workspace_mutes`;
-- drop "attention_workspace_mutes" table after copying rows
DROP TABLE `attention_workspace_mutes`;
-- rename temporary table "new_attention_workspace_mutes" to "attention_workspace_mutes"
ALTER TABLE `new_attention_workspace_mutes` RENAME TO `attention_workspace_mutes`;
-- create "user_scope_migrations" table
CREATE TABLE `user_scope_migrations` (`source_workspace_id` text NULL, `source_root` text NOT NULL, `disposition` text NOT NULL, `profile_ids` text NOT NULL, `sql_done_at` text NULL, `journal_state` text NOT NULL DEFAULT 'pending', `layout_state` text NOT NULL DEFAULT 'pending', `registration_removed_at` text NULL, PRIMARY KEY (`source_workspace_id`), CHECK (disposition IN ('migrate', 'keep_project')), CHECK (json_valid(profile_ids)), CHECK (journal_state IN ('pending', 'copied', 'verified', 'skipped')), CHECK (layout_state IN ('pending', 'copied', 'verified', 'skipped')));
-- create "user_scope_migration_archive" table
CREATE TABLE `user_scope_migration_archive` (`source_workspace_id` text NOT NULL, `table_name` text NOT NULL, `row_json` text NOT NULL, `reason` text NOT NULL, `archived_at` text NOT NULL, CHECK (json_valid(row_json)));
-- create "workspace_name_aliases" table
CREATE TABLE `workspace_name_aliases` (`old_name` text NULL, `workspace_id` text NOT NULL, `created_at` text NOT NULL, PRIMARY KEY (`old_name`));
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
-- apply declarative trigger "tool_approval_pending_profile_owner_active" on table "tool_approval_pending"
-- +goose StatementBegin
CREATE TRIGGER tool_approval_pending_profile_owner_active BEFORE INSERT ON tool_approval_pending BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd
-- apply declarative trigger "tool_approval_pending_profile_owner_immutable" on table "tool_approval_pending"
-- +goose StatementBegin
CREATE TRIGGER tool_approval_pending_profile_owner_immutable BEFORE UPDATE OF profile_id ON tool_approval_pending
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
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
-- apply declarative trigger "attention_workspace_mutes_workspace_insert_guard" on table "attention_workspace_mutes"
-- +goose StatementBegin
CREATE TRIGGER attention_workspace_mutes_workspace_insert_guard
BEFORE INSERT ON attention_workspace_mutes
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN SELECT RAISE(ABORT, 'workspace not found'); END;
-- +goose StatementEnd
-- apply declarative trigger "attention_workspace_mutes_workspace_update_guard" on table "attention_workspace_mutes"
-- +goose StatementBegin
CREATE TRIGGER attention_workspace_mutes_workspace_update_guard
BEFORE UPDATE OF workspace_id ON attention_workspace_mutes
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN SELECT RAISE(ABORT, 'workspace not found'); END;
-- +goose StatementEnd
-- apply declarative trigger "tool_approval_pending_workspace_insert_guard" on table "tool_approval_pending"
-- +goose StatementBegin
CREATE TRIGGER tool_approval_pending_workspace_insert_guard
BEFORE INSERT ON tool_approval_pending
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN SELECT RAISE(ABORT, 'workspace not found'); END;
-- +goose StatementEnd
-- apply declarative trigger "tool_approval_pending_workspace_update_guard" on table "tool_approval_pending"
-- +goose StatementBegin
CREATE TRIGGER tool_approval_pending_workspace_update_guard
BEFORE UPDATE OF workspace_id ON tool_approval_pending
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN SELECT RAISE(ABORT, 'workspace not found'); END;
-- +goose StatementEnd
