-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_network_channels" table
CREATE TABLE `new_network_channels` (`profile_id` text NOT NULL, `workspace_id` text NOT NULL, `channel` text NOT NULL, `purpose` text NOT NULL, `created_by` text NOT NULL DEFAULT '', `created_at` text NOT NULL, `updated_at` text NOT NULL, `fanout_policy` text NOT NULL DEFAULT 'capability_match', `coordinator_peer_id` text NOT NULL DEFAULT '', PRIMARY KEY (`workspace_id`, `channel`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (
					fanout_policy IN ('capability_match', 'coordinator', 'all_members')
				));
-- copy rows from old table "network_channels" to new temporary table "new_network_channels"
INSERT INTO `new_network_channels` (`profile_id`, `workspace_id`, `channel`, `purpose`, `created_by`, `created_at`, `updated_at`, `fanout_policy`, `coordinator_peer_id`) SELECT `profile_id`, `workspace_id`, `channel`, `purpose`, `created_by`, `created_at`, `updated_at`, `fanout_policy`, `coordinator_peer_id` FROM `network_channels`;
-- drop trigger "dead_entities_profile_owner_active" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `dead_entities_profile_owner_active`;
-- drop trigger "dead_entities_profile_owner_immutable" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `dead_entities_profile_owner_immutable`;
-- drop trigger "dead_entities_workspace_insert_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `dead_entities_workspace_insert_guard`;
-- drop trigger "dead_entities_workspace_update_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `dead_entities_workspace_update_guard`;
-- drop trigger "network_channels_profile_owner_active" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `network_channels_profile_owner_active`;
-- drop trigger "network_channels_profile_owner_immutable" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `network_channels_profile_owner_immutable`;
-- drop trigger "network_channels_workspace_insert_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `network_channels_workspace_insert_guard`;
-- drop trigger "network_channels_workspace_update_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `network_channels_workspace_update_guard`;
-- drop trigger "no_workspace_data_workspace_delete" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `no_workspace_data_workspace_delete`;
-- drop trigger "tool_approval_grants_profile_owner_active" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `tool_approval_grants_profile_owner_active`;
-- drop trigger "tool_approval_grants_profile_owner_immutable" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `tool_approval_grants_profile_owner_immutable`;
-- drop trigger "tool_approval_grants_workspace_insert_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `tool_approval_grants_workspace_insert_guard`;
-- drop trigger "tool_approval_grants_workspace_update_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `tool_approval_grants_workspace_update_guard`;
-- drop "network_channels" table after copying rows
DROP TABLE `network_channels`;
-- rename temporary table "new_network_channels" to "network_channels"
ALTER TABLE `new_network_channels` RENAME TO `network_channels`;
-- create index "idx_network_channels_updated_at" to table: "network_channels"
CREATE INDEX `idx_network_channels_updated_at` ON `network_channels` (`updated_at`);
-- create index "idx_network_channels_workspace" to table: "network_channels"
CREATE INDEX `idx_network_channels_workspace` ON `network_channels` (`workspace_id`);
-- create index "idx_network_channels_workspace_updated_at" to table: "network_channels"
CREATE INDEX `idx_network_channels_workspace_updated_at` ON `network_channels` (`workspace_id`, `updated_at` DESC, `channel`);
-- create "new_tool_approval_grants" table
CREATE TABLE `new_tool_approval_grants` (`id` text NOT NULL, `profile_id` text NOT NULL, `workspace_id` text NOT NULL, `agent_name` text NOT NULL DEFAULT '', `tool_id` text NOT NULL, `input_digest` text NOT NULL DEFAULT '', `decision` text NOT NULL, `created_at` text NOT NULL, `last_used_at` text NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (trim(id) <> ''), CHECK (trim(tool_id) <> ''), CHECK (input_digest = '' OR input_digest LIKE 'sha256:%'), CHECK (decision IN ('allow', 'reject')));
-- copy rows from old table "tool_approval_grants" to new temporary table "new_tool_approval_grants"
INSERT INTO `new_tool_approval_grants` (`id`, `profile_id`, `workspace_id`, `agent_name`, `tool_id`, `input_digest`, `decision`, `created_at`, `last_used_at`) SELECT `id`, `profile_id`, `workspace_id`, `agent_name`, `tool_id`, `input_digest`, `decision`, `created_at`, `last_used_at` FROM `tool_approval_grants`;
-- drop "tool_approval_grants" table after copying rows
DROP TABLE `tool_approval_grants`;
-- rename temporary table "new_tool_approval_grants" to "tool_approval_grants"
ALTER TABLE `new_tool_approval_grants` RENAME TO `tool_approval_grants`;
-- create index "tool_approval_grants_profile_id_workspace_id_agent_name_tool_id_input_digest" to table: "tool_approval_grants"
CREATE UNIQUE INDEX `tool_approval_grants_profile_id_workspace_id_agent_name_tool_id_input_digest` ON `tool_approval_grants` (`profile_id`, `workspace_id`, `agent_name`, `tool_id`, `input_digest`);
-- create index "idx_tool_approval_grants_lookup" to table: "tool_approval_grants"
CREATE INDEX `idx_tool_approval_grants_lookup` ON `tool_approval_grants` (`profile_id`, `workspace_id`, `tool_id`, `agent_name`, `input_digest`);
-- create index "idx_tool_approval_grants_list" to table: "tool_approval_grants"
CREATE INDEX `idx_tool_approval_grants_list` ON `tool_approval_grants` (`profile_id`, `workspace_id`, `created_at` DESC, `id`);
-- create "new_dead_entities" table
CREATE TABLE `new_dead_entities` (`profile_id` text NOT NULL, `workspace_id` text NOT NULL, `kind` text NOT NULL, `entity_id` text NOT NULL, `reason` text NOT NULL, `marked_at` text NOT NULL, PRIMARY KEY (`profile_id`, `workspace_id`, `kind`, `entity_id`), CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (kind IN ('extension', 'bridge', 'mcp_sidecar', 'loop_target')), CHECK (trim(entity_id) <> ''), CHECK (trim(reason) <> ''));
-- copy rows from old table "dead_entities" to new temporary table "new_dead_entities"
INSERT INTO `new_dead_entities` (`profile_id`, `workspace_id`, `kind`, `entity_id`, `reason`, `marked_at`) SELECT `profile_id`, `workspace_id`, `kind`, `entity_id`, `reason`, `marked_at` FROM `dead_entities`;
-- drop "dead_entities" table after copying rows
DROP TABLE `dead_entities`;
-- rename temporary table "new_dead_entities" to "dead_entities"
ALTER TABLE `new_dead_entities` RENAME TO `dead_entities`;
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
-- recreate trigger "dead_entities_profile_owner_active" after rebuilding table "dead_entities"
-- +goose StatementBegin
CREATE TRIGGER dead_entities_profile_owner_active BEFORE INSERT ON dead_entities BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd
-- recreate trigger "dead_entities_profile_owner_immutable" after rebuilding table "dead_entities"
-- +goose StatementBegin
CREATE TRIGGER dead_entities_profile_owner_immutable BEFORE UPDATE OF profile_id ON dead_entities
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
-- recreate trigger "dead_entities_workspace_insert_guard" after rebuilding table "dead_entities"
-- +goose StatementBegin
CREATE TRIGGER dead_entities_workspace_insert_guard
BEFORE INSERT ON dead_entities
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "dead_entities_workspace_update_guard" after rebuilding table "dead_entities"
-- +goose StatementBegin
CREATE TRIGGER dead_entities_workspace_update_guard
BEFORE UPDATE OF workspace_id ON dead_entities
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "network_channels_profile_owner_active" after rebuilding table "network_channels"
-- +goose StatementBegin
CREATE TRIGGER network_channels_profile_owner_active BEFORE INSERT ON network_channels BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd
-- recreate trigger "network_channels_profile_owner_immutable" after rebuilding table "network_channels"
-- +goose StatementBegin
CREATE TRIGGER network_channels_profile_owner_immutable BEFORE UPDATE OF profile_id ON network_channels
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
-- recreate trigger "network_channels_workspace_insert_guard" after rebuilding table "network_channels"
-- +goose StatementBegin
CREATE TRIGGER network_channels_workspace_insert_guard
BEFORE INSERT ON network_channels
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "network_channels_workspace_update_guard" after rebuilding table "network_channels"
-- +goose StatementBegin
CREATE TRIGGER network_channels_workspace_update_guard
BEFORE UPDATE OF workspace_id ON network_channels
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "no_workspace_data_workspace_delete" after rebuilding table "workspaces"
-- +goose StatementBegin
CREATE TRIGGER no_workspace_data_workspace_delete
AFTER DELETE ON workspaces
BEGIN
	DELETE FROM network_wake_events WHERE workspace_id = OLD.id;
	DELETE FROM network_wake_sources WHERE workspace_id = OLD.id;
	DELETE FROM network_message_dispositions WHERE workspace_id = OLD.id;
	DELETE FROM network_live_wakes WHERE workspace_id = OLD.id;
	DELETE FROM network_participation_budgets WHERE workspace_id = OLD.id;
	DELETE FROM network_task_status_projections WHERE workspace_id = OLD.id;
	DELETE FROM network_task_thread_origins WHERE workspace_id = OLD.id;
	DELETE FROM network_thread_session_token_stats WHERE workspace_id = OLD.id;
	DELETE FROM network_thread_participants WHERE workspace_id = OLD.id;
	DELETE FROM network_subscriptions WHERE workspace_id = OLD.id;
	DELETE FROM network_work WHERE workspace_id = OLD.id;
	DELETE FROM network_direct_rooms WHERE workspace_id = OLD.id;
	DELETE FROM network_threads WHERE workspace_id = OLD.id;
	DELETE FROM network_channel_participants WHERE workspace_id = OLD.id;
	DELETE FROM network_channel_kind_counts WHERE workspace_id = OLD.id;
	DELETE FROM network_channel_stats WHERE workspace_id = OLD.id;
	DELETE FROM network_timeline_log WHERE workspace_id = OLD.id;
	DELETE FROM network_channels WHERE workspace_id = OLD.id;
	DELETE FROM network_audit_log WHERE workspace_id = OLD.id;
	DELETE FROM network_coordination_invitations WHERE workspace_id = OLD.id;
	DELETE FROM task_network_coordination WHERE workspace_id = OLD.id;
	DELETE FROM loop_ui_annotations WHERE workspace_id = OLD.id;
	DELETE FROM loop_session_bindings WHERE workspace_id = OLD.id;
	DELETE FROM loop_run_events WHERE workspace_id = OLD.id;
	DELETE FROM loop_runs WHERE workspace_id = OLD.id;
	DELETE FROM loop_goal_session_outbox WHERE workspace_id = OLD.id;
	DELETE FROM loop_goal_session_cleanup WHERE workspace_id = OLD.id;
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
	DELETE FROM session_health WHERE workspace_id = OLD.id;
	DELETE FROM sessions WHERE workspace_id = OLD.id;
	DELETE FROM token_usage_daily WHERE workspace_id = OLD.id;
	DELETE FROM event_summaries WHERE workspace_id = OLD.id;
	DELETE FROM tool_approval_grants WHERE workspace_id = OLD.id;
	DELETE FROM dead_entities WHERE workspace_id = OLD.id;
	DELETE FROM notification_cursors WHERE workspace_id = OLD.id;
END;
-- +goose StatementEnd
-- recreate trigger "tool_approval_grants_profile_owner_active" after rebuilding table "tool_approval_grants"
-- +goose StatementBegin
CREATE TRIGGER tool_approval_grants_profile_owner_active BEFORE INSERT ON tool_approval_grants BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd
-- recreate trigger "tool_approval_grants_profile_owner_immutable" after rebuilding table "tool_approval_grants"
-- +goose StatementBegin
CREATE TRIGGER tool_approval_grants_profile_owner_immutable BEFORE UPDATE OF profile_id ON tool_approval_grants
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
-- recreate trigger "tool_approval_grants_workspace_insert_guard" after rebuilding table "tool_approval_grants"
-- +goose StatementBegin
CREATE TRIGGER tool_approval_grants_workspace_insert_guard
BEFORE INSERT ON tool_approval_grants
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- recreate trigger "tool_approval_grants_workspace_update_guard" after rebuilding table "tool_approval_grants"
-- +goose StatementBegin
CREATE TRIGGER tool_approval_grants_workspace_update_guard
BEFORE UPDATE OF workspace_id ON tool_approval_grants
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd

