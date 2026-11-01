-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_agent_heartbeat_wake_events" table
CREATE TABLE `new_agent_heartbeat_wake_events` (`id` text NULL, `workspace_id` text NOT NULL, `agent_name` text NOT NULL, `session_id` text NULL, `policy_snapshot_id` text NULL, `source` text NOT NULL, `result` text NOT NULL, `reason` text NOT NULL, `detail` text NOT NULL DEFAULT '', `denied_calls` integer NOT NULL DEFAULT 0, `occurrence_id` text NOT NULL DEFAULT '', `synthetic_prompt_id` text NULL, `created_at` text NOT NULL, `expires_at` text NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`policy_snapshot_id`) REFERENCES `agent_heartbeat_snapshots` (`id`) ON UPDATE NO ACTION ON DELETE SET NULL, CONSTRAINT `1` FOREIGN KEY (`session_id`) REFERENCES `sessions` (`id`) ON UPDATE NO ACTION ON DELETE SET NULL, CHECK (source IN ('scheduler', 'manual', 'harness_reentry', 'timer', 'instance_check')), CHECK (
				result IN ('sent', 'skipped', 'coalesced', 'rate_limited', 'failed', 'quiet', 'cancelled', 'reported')
			), CHECK (
				reason IN (
					'wake_sent',
					'heartbeat_disabled',
					'heartbeat_invalid',
					'heartbeat_no_policy',
					'heartbeat_rate_limited',
					'heartbeat_no_eligible_session',
					'cooldown_active',
					'quiet_window',
					'session_not_found',
					'session_unhealthy',
					'session_not_attachable',
					'session_prompt_active',
					'session_prompt_active_race',
					'synthetic_prompt_failed',
					'wake_coalesced',
					'quiet',
					'reported',
					'cancelled',
					'digest_empty',
					'digest_unchanged',
					'policy_disabled',
					'definition_not_stays_on',
					'definition_missing',
					'no_slot',
					'hook_cancelled',
					'duplicate_suppressed',
					'triage_unsupported',
					'triage_output_invalid',
					'triage_timeout',
					'triage_interrupted',
					'triage_failed'
				)
			), CHECK (denied_calls >= 0));
-- copy rows from old table "agent_heartbeat_wake_events" to new temporary table "new_agent_heartbeat_wake_events"
INSERT INTO `new_agent_heartbeat_wake_events` (`id`, `workspace_id`, `agent_name`, `session_id`, `policy_snapshot_id`, `source`, `result`, `reason`, `synthetic_prompt_id`, `created_at`, `expires_at`) SELECT `id`, `workspace_id`, `agent_name`, `session_id`, `policy_snapshot_id`, `source`, `result`, `reason`, `synthetic_prompt_id`, `created_at`, `expires_at` FROM `agent_heartbeat_wake_events`;
-- drop trigger "agent_heartbeat_wake_events_workspace_insert_guard" before applying its declarative change
DROP TRIGGER IF EXISTS `agent_heartbeat_wake_events_workspace_insert_guard`;
-- drop trigger "agent_heartbeat_wake_events_workspace_update_guard" before applying its declarative change
DROP TRIGGER IF EXISTS `agent_heartbeat_wake_events_workspace_update_guard`;
-- drop trigger "workspace_scope_cleanup_after_delete" before applying its declarative change
DROP TRIGGER IF EXISTS `workspace_scope_cleanup_after_delete`;
-- drop "agent_heartbeat_wake_events" table after copying rows
DROP TABLE `agent_heartbeat_wake_events`;
-- rename temporary table "new_agent_heartbeat_wake_events" to "agent_heartbeat_wake_events"
ALTER TABLE `new_agent_heartbeat_wake_events` RENAME TO `agent_heartbeat_wake_events`;
-- create index "idx_agent_heartbeat_wake_events_agent_created" to table: "agent_heartbeat_wake_events"
CREATE INDEX `idx_agent_heartbeat_wake_events_agent_created` ON `agent_heartbeat_wake_events` (`workspace_id`, `agent_name`, `created_at` DESC, `id` DESC);
-- create index "idx_agent_heartbeat_wake_events_expires" to table: "agent_heartbeat_wake_events"
CREATE INDEX `idx_agent_heartbeat_wake_events_expires` ON `agent_heartbeat_wake_events` (`expires_at`);
-- create index "idx_agent_heartbeat_wake_events_session" to table: "agent_heartbeat_wake_events"
CREATE INDEX `idx_agent_heartbeat_wake_events_session` ON `agent_heartbeat_wake_events` (`session_id`);
-- create "agent_instance_wake_state" table
CREATE TABLE `agent_instance_wake_state` (`instance_id` text NULL, `next_wake_at` text NOT NULL DEFAULT '', `last_wake_at` text NOT NULL DEFAULT '', `last_digest` text NOT NULL DEFAULT '', `last_digest_at` text NOT NULL DEFAULT '', `automation_cursor` text NOT NULL DEFAULT '', `quiet_streak` integer NOT NULL DEFAULT 0, `failure_streak` integer NOT NULL DEFAULT 0, `occurrence_id` text NOT NULL DEFAULT '', `triage_session_id` text NOT NULL DEFAULT '', `occurrence_started_at` text NOT NULL DEFAULT '', `occurrence_cancelled` integer NOT NULL DEFAULT 0, PRIMARY KEY (`instance_id`), CONSTRAINT `0` FOREIGN KEY (`instance_id`) REFERENCES `agent_instances` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (quiet_streak >= 0), CHECK (failure_streak >= 0), CHECK (occurrence_cancelled IN (0, 1)));
-- create index "idx_agent_instance_wake_state_due" to table: "agent_instance_wake_state"
CREATE INDEX `idx_agent_instance_wake_state_due` ON `agent_instance_wake_state` (`next_wake_at`, `instance_id`);
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
-- apply declarative trigger "agent_heartbeat_wake_events_workspace_insert_guard" on table "agent_heartbeat_wake_events"
-- +goose StatementBegin
CREATE TRIGGER agent_heartbeat_wake_events_workspace_insert_guard
BEFORE INSERT ON agent_heartbeat_wake_events
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
-- +goose StatementEnd
-- apply declarative trigger "agent_heartbeat_wake_events_workspace_update_guard" on table "agent_heartbeat_wake_events"
-- +goose StatementBegin
CREATE TRIGGER agent_heartbeat_wake_events_workspace_update_guard
BEFORE UPDATE OF workspace_id ON agent_heartbeat_wake_events
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;
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
-- Existing instances must explicitly opt in after upgrading.
UPDATE agent_instances SET checks_enabled = 0;
INSERT INTO agent_instance_wake_state (instance_id) SELECT id FROM agent_instances;
