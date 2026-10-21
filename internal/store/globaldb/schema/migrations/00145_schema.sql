-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_loop_definition_snapshots" table
CREATE TABLE `new_loop_definition_snapshots` (`profile_id` text NOT NULL DEFAULT '', `workspace_id` text NOT NULL, `definition_digest` text NOT NULL, `definition_version` integer NOT NULL DEFAULT 0, `definition_json` text NOT NULL, `byte_size` integer NOT NULL, `created_at` text NOT NULL, `last_used_at` text NOT NULL, PRIMARY KEY (`profile_id`, `workspace_id`, `definition_digest`), CHECK (byte_size >= 0));
-- copy rows from old table "loop_definition_snapshots" to new temporary table "new_loop_definition_snapshots"
INSERT INTO `new_loop_definition_snapshots` (`workspace_id`, `definition_digest`, `definition_version`, `definition_json`, `byte_size`, `created_at`, `last_used_at`) SELECT `workspace_id`, `definition_digest`, `definition_version`, `definition_json`, `byte_size`, `created_at`, `last_used_at` FROM `loop_definition_snapshots`;
-- drop trigger "workspace_scope_cleanup_after_delete" before applying its declarative change
DROP TRIGGER IF EXISTS `workspace_scope_cleanup_after_delete`;
-- drop "loop_definition_snapshots" table after copying rows
DROP TABLE `loop_definition_snapshots`;
-- rename temporary table "new_loop_definition_snapshots" to "loop_definition_snapshots"
ALTER TABLE `new_loop_definition_snapshots` RENAME TO `loop_definition_snapshots`;
-- create "new_loop_ui_annotations" table
CREATE TABLE `new_loop_ui_annotations` (`profile_id` text NOT NULL DEFAULT '', `workspace_id` text NOT NULL, `loop_name` text NOT NULL, `node_id` text NOT NULL, `x` real NOT NULL, `y` real NOT NULL, PRIMARY KEY (`profile_id`, `workspace_id`, `loop_name`, `node_id`));
-- copy rows from old table "loop_ui_annotations" to new temporary table "new_loop_ui_annotations"
INSERT INTO `new_loop_ui_annotations` (`workspace_id`, `loop_name`, `node_id`, `x`, `y`) SELECT `workspace_id`, `loop_name`, `node_id`, `x`, `y` FROM `loop_ui_annotations`;
-- drop "loop_ui_annotations" table after copying rows
DROP TABLE `loop_ui_annotations`;
-- rename temporary table "new_loop_ui_annotations" to "loop_ui_annotations"
ALTER TABLE `new_loop_ui_annotations` RENAME TO `loop_ui_annotations`;
-- create "new_loop_admission_claims" table
CREATE TABLE `new_loop_admission_claims` (`profile_id` text NOT NULL DEFAULT '', `workspace_id` text NOT NULL, `loop_name` text NOT NULL, `source_key` text NOT NULL, `event_key` text NOT NULL, `loop_run_id` text NOT NULL, `claimed_at` timestamp NOT NULL, `expires_at` timestamp NOT NULL, `suppressed_count` integer NOT NULL DEFAULT 0, `last_suppressed_at` timestamp NULL, PRIMARY KEY (`profile_id`, `workspace_id`, `loop_name`, `source_key`, `event_key`));
-- copy rows from old table "loop_admission_claims" to new temporary table "new_loop_admission_claims"
INSERT INTO `new_loop_admission_claims` (`workspace_id`, `loop_name`, `source_key`, `event_key`, `loop_run_id`, `claimed_at`, `expires_at`, `suppressed_count`, `last_suppressed_at`) SELECT `workspace_id`, `loop_name`, `source_key`, `event_key`, `loop_run_id`, `claimed_at`, `expires_at`, `suppressed_count`, `last_suppressed_at` FROM `loop_admission_claims`;
-- drop "loop_admission_claims" table after copying rows
DROP TABLE `loop_admission_claims`;
-- rename temporary table "new_loop_admission_claims" to "loop_admission_claims"
ALTER TABLE `new_loop_admission_claims` RENAME TO `loop_admission_claims`;
-- create index "idx_loop_admission_claims_expiry" to table: "loop_admission_claims"
CREATE INDEX `idx_loop_admission_claims_expiry` ON `loop_admission_claims` (`expires_at`);
-- create "new_loop_config" table
CREATE TABLE `new_loop_config` (`profile_id` text NOT NULL DEFAULT '', `workspace_id` text NOT NULL, `loop_name` text NOT NULL, `human_gate_enabled` integer NOT NULL DEFAULT 0, `reattempt_strategy` text NULL, `enabled_checks_json` text NOT NULL DEFAULT '{}', `iteration_cap` integer NULL, `budget_tokens` integer NULL, `budget_wall_sec` integer NULL, `budget_on_exceeded` text NULL, `no_progress_window` integer NULL, `fan_out_width` integer NULL, `gate_max_revisions` integer NULL, `runtime_defaults_json` text NULL, `runtime_rules_json` text NULL, `environment_json` text NULL, PRIMARY KEY (`profile_id`, `workspace_id`, `loop_name`), CHECK (runtime_defaults_json IS NULL OR json_valid(runtime_defaults_json)), CHECK (runtime_rules_json IS NULL OR json_valid(runtime_rules_json)), CHECK (environment_json IS NULL OR json_valid(environment_json)));
-- copy rows from old table "loop_config" to new temporary table "new_loop_config"
INSERT INTO `new_loop_config` (`workspace_id`, `loop_name`, `human_gate_enabled`, `reattempt_strategy`, `enabled_checks_json`, `iteration_cap`, `budget_tokens`, `budget_wall_sec`, `budget_on_exceeded`, `no_progress_window`, `fan_out_width`, `gate_max_revisions`, `runtime_defaults_json`, `runtime_rules_json`, `environment_json`) SELECT `workspace_id`, `loop_name`, `human_gate_enabled`, `reattempt_strategy`, `enabled_checks_json`, `iteration_cap`, `budget_tokens`, `budget_wall_sec`, `budget_on_exceeded`, `no_progress_window`, `fan_out_width`, `gate_max_revisions`, `runtime_defaults_json`, `runtime_rules_json`, `environment_json` FROM `loop_config`;
-- drop "loop_config" table after copying rows
DROP TABLE `loop_config`;
-- rename temporary table "new_loop_config" to "loop_config"
ALTER TABLE `new_loop_config` RENAME TO `loop_config`;
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
