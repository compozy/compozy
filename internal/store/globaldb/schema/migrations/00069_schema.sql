-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_loop_node_controls" table
CREATE TABLE `new_loop_node_controls` (`loop_run_id` text NOT NULL, `node_id` text NOT NULL, `paused` integer NOT NULL DEFAULT 0, `pause_actor_kind` text NULL, `pause_actor_id` text NULL, `pause_reason` text NULL, `pause_rule_id` text NULL, `pause_requested_at` timestamp NULL, `quarantined` integer NOT NULL DEFAULT 0, `quarantine_entry_json` text NULL, `quarantined_at` timestamp NULL, `attention_flag` text NOT NULL DEFAULT '', `attention_reason` text NOT NULL DEFAULT '', `attention_producer_node_id` text NOT NULL DEFAULT '', `cancel_state` text NOT NULL DEFAULT '', `cancel_actor_kind` text NULL, `cancel_actor_id` text NULL, `cancel_reason` text NULL, `cancel_requested_at` timestamp NULL, `last_evidence_at` timestamp NULL, `death_resume_streak` integer NOT NULL DEFAULT 0, `gate_revisions_json` text NOT NULL DEFAULT '{}', `revision` integer NOT NULL DEFAULT 0, `updated_at` timestamp NOT NULL, PRIMARY KEY (`loop_run_id`, `node_id`), CONSTRAINT `0` FOREIGN KEY (`loop_run_id`) REFERENCES `loop_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (quarantine_entry_json IS NULL OR json_valid(quarantine_entry_json)), CHECK (attention_flag IN (
		'', 'silence', 'resume_exhausted', 'dependency_quarantined', 'wait_intervention', 'expired_wait'
	)), CHECK (cancel_state IN (
		'', 'requested', 'delivering', 'draining', 'canceled'
	)), CHECK (death_resume_streak >= 0), CHECK (
		json_valid(gate_revisions_json) AND json_type(gate_revisions_json) = 'object'
	));
-- copy rows from old table "loop_node_controls" to new temporary table "new_loop_node_controls"
INSERT INTO `new_loop_node_controls` (`loop_run_id`, `node_id`, `paused`, `pause_actor_kind`, `pause_actor_id`, `pause_reason`, `pause_rule_id`, `pause_requested_at`, `quarantined`, `quarantine_entry_json`, `quarantined_at`, `attention_flag`, `attention_reason`, `attention_producer_node_id`, `cancel_state`, `cancel_actor_kind`, `cancel_actor_id`, `cancel_reason`, `cancel_requested_at`, `last_evidence_at`, `death_resume_streak`, `revision`, `updated_at`) SELECT `loop_run_id`, `node_id`, `paused`, `pause_actor_kind`, `pause_actor_id`, `pause_reason`, `pause_rule_id`, `pause_requested_at`, `quarantined`, `quarantine_entry_json`, `quarantined_at`, `attention_flag`, `attention_reason`, `attention_producer_node_id`, `cancel_state`, `cancel_actor_kind`, `cancel_actor_id`, `cancel_reason`, `cancel_requested_at`, `last_evidence_at`, `death_resume_streak`, `revision`, `updated_at` FROM `loop_node_controls`;
-- drop "loop_node_controls" table after copying rows
DROP TABLE `loop_node_controls`;
-- rename temporary table "new_loop_node_controls" to "loop_node_controls"
ALTER TABLE `new_loop_node_controls` RENAME TO `loop_node_controls`;
-- create index "idx_loop_node_controls_quarantined" to table: "loop_node_controls"
CREATE INDEX `idx_loop_node_controls_quarantined` ON `loop_node_controls` (`quarantined`) WHERE quarantined = 1;
-- create index "idx_loop_node_controls_attention" to table: "loop_node_controls"
CREATE INDEX `idx_loop_node_controls_attention` ON `loop_node_controls` (`attention_flag`) WHERE attention_flag != '';
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
