-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- add column "delivery_key" to table: "loop_run_events"
ALTER TABLE `loop_run_events` ADD COLUMN `delivery_key` text NULL;
-- create index "uq_loop_run_events_delivery" to table: "loop_run_events"
CREATE UNIQUE INDEX `uq_loop_run_events_delivery` ON `loop_run_events` (`loop_run_id`, `delivery_key`) WHERE delivery_key IS NOT NULL;
-- create "new_dead_entities" table
CREATE TABLE `new_dead_entities` (`workspace_id` text NOT NULL, `kind` text NOT NULL, `entity_id` text NOT NULL, `reason` text NOT NULL, `marked_at` text NOT NULL, PRIMARY KEY (`workspace_id`, `kind`, `entity_id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (kind IN ('extension', 'bridge', 'mcp_sidecar', 'loop_target')), CHECK (trim(entity_id) <> ''), CHECK (trim(reason) <> ''));
-- copy rows from old table "dead_entities" to new temporary table "new_dead_entities"
INSERT INTO `new_dead_entities` (`workspace_id`, `kind`, `entity_id`, `reason`, `marked_at`) SELECT `workspace_id`, `kind`, `entity_id`, `reason`, `marked_at` FROM `dead_entities`;
-- drop "dead_entities" table after copying rows
DROP TABLE `dead_entities`;
-- rename temporary table "new_dead_entities" to "dead_entities"
ALTER TABLE `new_dead_entities` RENAME TO `dead_entities`;
-- create "new_loop_generation_outputs" table
CREATE TABLE `new_loop_generation_outputs` (`loop_run_id` text NOT NULL, `generation` integer NOT NULL, `node_id` text NOT NULL, `item_index` integer NOT NULL DEFAULT 0, `status` text NOT NULL, `output_ref` text NULL, `task_run_id` text NULL, `child_loop_run_id` text NULL, `resolved_runtime_json` text NULL, `attempt` integer NOT NULL DEFAULT 1, `next_attempt_at` timestamp NULL, `first_scheduled_at` timestamp NULL, `epoch` integer NOT NULL DEFAULT 0, `goal_status` text NULL, `goal_turns_used` integer NULL, `goal_turn_limit` integer NULL, PRIMARY KEY (`loop_run_id`, `generation`, `node_id`, `item_index`), CONSTRAINT `0` FOREIGN KEY (`loop_run_id`) REFERENCES `loop_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (status IN (
				'pending','enqueued','running','retrying','waiting','paused','awaiting_child',
				'control_pending','awaiting_goal','succeeded','failed','canceled','quarantined'
			)), CHECK (resolved_runtime_json IS NULL OR json_valid(resolved_runtime_json)), CHECK (attempt >= 1));
-- copy rows from old table "loop_generation_outputs" to new temporary table "new_loop_generation_outputs"
INSERT INTO `new_loop_generation_outputs` (`loop_run_id`, `generation`, `node_id`, `item_index`, `status`, `output_ref`, `task_run_id`, `child_loop_run_id`, `resolved_runtime_json`, `goal_status`, `goal_turns_used`, `goal_turn_limit`) SELECT `loop_run_id`, `generation`, `node_id`, `item_index`, `status`, `output_ref`, `task_run_id`, `child_loop_run_id`, `resolved_runtime_json`, `goal_status`, `goal_turns_used`, `goal_turn_limit` FROM `loop_generation_outputs`;
-- drop "loop_generation_outputs" table after copying rows
DROP TABLE `loop_generation_outputs`;
-- rename temporary table "new_loop_generation_outputs" to "loop_generation_outputs"
ALTER TABLE `new_loop_generation_outputs` RENAME TO `loop_generation_outputs`;
-- create index "idx_loop_generation_outputs_output_ref" to table: "loop_generation_outputs"
CREATE INDEX `idx_loop_generation_outputs_output_ref` ON `loop_generation_outputs` (`output_ref`);
-- create "new_loop_runs" table
CREATE TABLE `new_loop_runs` (`id` text NULL, `workspace_id` text NOT NULL, `loop_name` text NOT NULL, `status` text NOT NULL, `generation` integer NOT NULL DEFAULT 0, `reattempt_strategy` text NOT NULL DEFAULT 'failed_only', `last_progress_at` timestamp NOT NULL, `budget_tokens` integer NOT NULL DEFAULT 0, `budget_wall_sec` integer NOT NULL DEFAULT 0, `budget_on_exceeded` text NOT NULL DEFAULT 'halt', `tokens_used` integer NOT NULL DEFAULT 0, `parent_loop_run_id` text NULL, `pause_requested` integer NOT NULL DEFAULT 0, `inputs_json` text NOT NULL, `created_at` text NOT NULL DEFAULT '1970-01-01T00:00:00.000000000Z', `iteration_cap` integer NOT NULL DEFAULT 0, `started_by_kind` text NOT NULL DEFAULT '', `started_by_ref` text NOT NULL DEFAULT '', `started_origin_kind` text NOT NULL DEFAULT '', `started_origin_ref` text NOT NULL DEFAULT '', `started_at` text NOT NULL DEFAULT '1970-01-01T00:00:00.000000000Z', `definition_version` integer NOT NULL DEFAULT 0, `definition_digest` text NOT NULL DEFAULT '', `active_gate_id` text NOT NULL DEFAULT '', `active_human_criteria_json` text NOT NULL DEFAULT '[]', `budget_approval_seq` integer NOT NULL DEFAULT 0, `start_metadata_json` text NOT NULL DEFAULT '{}', `origin_kind` text NOT NULL DEFAULT 'catalog', `origin_session_id` text NULL, `goal_cleared_at` timestamp NULL, `budget_version` integer NOT NULL DEFAULT 0, `goal_context_nudge_ratio` real NOT NULL DEFAULT 0.8, `control_actor_kind` text NULL, `control_actor_id` text NULL, `control_requested_at` timestamp NULL, `origin_creation_profile_ref` text NULL, `origin_policy_spec_digest` text NULL, `origin_creation_digest` text NULL, `network_spec_json` text NOT NULL DEFAULT '{"version":"network-participation/v1","mode":"local","source":"built_in_local"}', `network_mode` text NOT NULL DEFAULT 'local', `network_channel` text NULL, `network_source` text NOT NULL DEFAULT 'built_in_local', `best_generation` integer NULL, `best_score` real NULL, `cancel_requested` integer NOT NULL DEFAULT 0, `cancel_kind` text NOT NULL DEFAULT '', PRIMARY KEY (`id`), CHECK (budget_version >= 0), CHECK (goal_context_nudge_ratio >= 0.0 AND goal_context_nudge_ratio <= 1.0), CHECK (origin_creation_profile_ref IS NULL OR length(trim(origin_creation_profile_ref)) > 0), CHECK (origin_policy_spec_digest IS NULL OR length(trim(origin_policy_spec_digest)) > 0), CHECK (origin_creation_digest IS NULL OR length(trim(origin_creation_digest)) > 0), CHECK (json_valid(network_spec_json)), CHECK (network_mode IN ('local', 'live')), CHECK (network_source IN (
						'explicit_request', 'task_profile', 'workspace_coordination',
						'loop_definition', 'automation_job', 'built_in_local'
					)), CHECK (cancel_kind IN ('', 'cancel', 'kill')), CHECK (
						(best_generation IS NULL AND best_score IS NULL)
						OR (best_generation IS NOT NULL AND best_score IS NOT NULL
							AND best_generation >= 1 AND best_generation <= generation)
					));
-- copy rows from old table "loop_runs" to new temporary table "new_loop_runs"
INSERT INTO `new_loop_runs` (`id`, `workspace_id`, `loop_name`, `status`, `generation`, `reattempt_strategy`, `last_progress_at`, `budget_tokens`, `budget_wall_sec`, `budget_on_exceeded`, `tokens_used`, `parent_loop_run_id`, `pause_requested`, `inputs_json`, `created_at`, `iteration_cap`, `started_by_kind`, `started_by_ref`, `started_origin_kind`, `started_origin_ref`, `started_at`, `definition_version`, `definition_digest`, `active_gate_id`, `active_human_criteria_json`, `budget_approval_seq`, `start_metadata_json`, `origin_kind`, `origin_session_id`, `goal_cleared_at`, `budget_version`, `goal_context_nudge_ratio`, `control_actor_kind`, `control_actor_id`, `control_requested_at`, `origin_creation_profile_ref`, `origin_policy_spec_digest`, `origin_creation_digest`, `network_spec_json`, `network_mode`, `network_channel`, `network_source`, `best_generation`, `best_score`) SELECT `id`, `workspace_id`, `loop_name`, `status`, `generation`, `reattempt_strategy`, `last_progress_at`, `budget_tokens`, `budget_wall_sec`, `budget_on_exceeded`, `tokens_used`, `parent_loop_run_id`, `pause_requested`, `inputs_json`, `created_at`, `iteration_cap`, `started_by_kind`, `started_by_ref`, `started_origin_kind`, `started_origin_ref`, `started_at`, `definition_version`, `definition_digest`, `active_gate_id`, `active_human_criteria_json`, `budget_approval_seq`, `start_metadata_json`, `origin_kind`, `origin_session_id`, `goal_cleared_at`, `budget_version`, `goal_context_nudge_ratio`, `control_actor_kind`, `control_actor_id`, `control_requested_at`, `origin_creation_profile_ref`, `origin_policy_spec_digest`, `origin_creation_digest`, `network_spec_json`, `network_mode`, `network_channel`, `network_source`, `best_generation`, `best_score` FROM `loop_runs`;
-- drop "loop_runs" table after copying rows
DROP TABLE `loop_runs`;
-- rename temporary table "new_loop_runs" to "loop_runs"
ALTER TABLE `new_loop_runs` RENAME TO `loop_runs`;
-- create index "idx_loop_runs_catalog" to table: "loop_runs"
CREATE INDEX `idx_loop_runs_catalog` ON `loop_runs` (`workspace_id`, `loop_name`, `created_at` DESC, `id` DESC, `status`);
-- create index "idx_loop_runs_queue_order" to table: "loop_runs"
CREATE INDEX `idx_loop_runs_queue_order` ON `loop_runs` (`workspace_id`, `loop_name`, `status`, `created_at`, `id`);
-- create index "uq_loop_runs_active_session_goal" to table: "loop_runs"
CREATE UNIQUE INDEX `uq_loop_runs_active_session_goal` ON `loop_runs` (`origin_session_id`) WHERE origin_kind='session'
			  AND status IN ('queued','running','watching','needs-approval','paused');
-- create "new_loop_generations" table
CREATE TABLE `new_loop_generations` (`loop_run_id` text NOT NULL, `generation` integer NOT NULL, `parent_generation` integer NOT NULL DEFAULT 0, `origin` text NOT NULL, `created_at` timestamp NOT NULL, PRIMARY KEY (`loop_run_id`, `generation`), CONSTRAINT `0` FOREIGN KEY (`loop_run_id`) REFERENCES `loop_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (generation >= 1), CHECK (parent_generation >= 0 AND parent_generation < generation), CHECK (origin IN (
				'initial','stop_when','reattempt','gate_revise','gate_next_generation',
				'dod_retry','ratchet_restore','requeue'
			)));
-- copy rows from old table "loop_generations" to new temporary table "new_loop_generations"
INSERT INTO `new_loop_generations` (`loop_run_id`, `generation`, `parent_generation`, `origin`, `created_at`) SELECT `loop_run_id`, `generation`, `parent_generation`, `origin`, `created_at` FROM `loop_generations`;
-- drop "loop_generations" table after copying rows
DROP TABLE `loop_generations`;
-- rename temporary table "new_loop_generations" to "loop_generations"
ALTER TABLE `new_loop_generations` RENAME TO `loop_generations`;
-- create "loop_node_controls" table
CREATE TABLE `loop_node_controls` (`loop_run_id` text NOT NULL, `node_id` text NOT NULL, `paused` integer NOT NULL DEFAULT 0, `pause_actor_kind` text NULL, `pause_actor_id` text NULL, `pause_reason` text NULL, `pause_rule_id` text NULL, `pause_requested_at` timestamp NULL, `quarantined` integer NOT NULL DEFAULT 0, `quarantine_entry_json` text NULL, `quarantined_at` timestamp NULL, `attention_flag` text NOT NULL DEFAULT '', `attention_reason` text NOT NULL DEFAULT '', `cancel_state` text NOT NULL DEFAULT '', `cancel_actor_kind` text NULL, `cancel_actor_id` text NULL, `cancel_reason` text NULL, `cancel_requested_at` timestamp NULL, `last_evidence_at` timestamp NULL, `death_resume_streak` integer NOT NULL DEFAULT 0, `revision` integer NOT NULL DEFAULT 0, `updated_at` timestamp NOT NULL, PRIMARY KEY (`loop_run_id`, `node_id`), CONSTRAINT `0` FOREIGN KEY (`loop_run_id`) REFERENCES `loop_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (quarantine_entry_json IS NULL OR json_valid(quarantine_entry_json)), CHECK (attention_flag IN (
		'', 'silence', 'resume_exhausted', 'dependency_quarantined', 'wait_intervention', 'expired_wait'
	)), CHECK (cancel_state IN (
		'', 'requested', 'delivering', 'draining', 'canceled'
	)), CHECK (death_resume_streak >= 0));
-- create index "idx_loop_node_controls_quarantined" to table: "loop_node_controls"
CREATE INDEX `idx_loop_node_controls_quarantined` ON `loop_node_controls` (`quarantined`) WHERE quarantined = 1;
-- create index "idx_loop_node_controls_attention" to table: "loop_node_controls"
CREATE INDEX `idx_loop_node_controls_attention` ON `loop_node_controls` (`attention_flag`) WHERE attention_flag != '';
-- create "loop_node_attempts" table
CREATE TABLE `loop_node_attempts` (`loop_run_id` text NOT NULL, `generation` integer NOT NULL, `node_id` text NOT NULL, `item_index` integer NOT NULL DEFAULT 0, `attempt` integer NOT NULL, `failure_class` text NULL, `failure_code` text NOT NULL DEFAULT '', `cause` text NOT NULL DEFAULT '', `hint` text NOT NULL DEFAULT '', `target` text NOT NULL DEFAULT '', `disposition` text NOT NULL, `started_at` timestamp NOT NULL, `ended_at` timestamp NULL, `next_attempt_at` timestamp NULL, PRIMARY KEY (`loop_run_id`, `generation`, `node_id`, `item_index`, `attempt`), CONSTRAINT `0` FOREIGN KEY (`loop_run_id`) REFERENCES `loop_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (generation >= 1), CHECK (attempt >= 1), CHECK (failure_class IS NULL OR failure_class IN (
		'transport','payload_declared','quality_rejection','authoring','cancellation',
		'attempt_timeout','budget_exhausted','target_unavailable'
	)), CHECK (disposition IN (
		'succeeded','retried','routed','absorbed','escalated','quarantined','canceled','resumed'
	)));
-- create "loop_node_waits" table
CREATE TABLE `loop_node_waits` (`loop_run_id` text NOT NULL, `generation` integer NOT NULL, `node_id` text NOT NULL, `item_index` integer NOT NULL DEFAULT 0, `kind` text NOT NULL, `resume_at` timestamp NULL, `next_escalation_at` timestamp NULL, `escalation_cursor` integer NOT NULL DEFAULT 0, `claim_state` text NOT NULL DEFAULT 'waiting', `claimed_by_kind` text NULL, `claimed_by_id` text NULL, `claimed_at` timestamp NULL, `admission_failures` integer NOT NULL DEFAULT 0, `expect_json` text NULL, `ahead_payload_json` text NULL, `issued_epoch` integer NOT NULL DEFAULT 0, `created_at` timestamp NOT NULL, PRIMARY KEY (`loop_run_id`, `generation`, `node_id`, `item_index`), CONSTRAINT `0` FOREIGN KEY (`loop_run_id`) REFERENCES `loop_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (generation >= 1), CHECK (kind IN ('timer','event','approval_escalation')), CHECK (claim_state IN (
		'waiting','claimed','resumed','intervention_required'
	)), CHECK (expect_json IS NULL OR json_valid(expect_json)), CHECK (ahead_payload_json IS NULL OR json_valid(ahead_payload_json)));
-- create index "idx_loop_node_waits_due" to table: "loop_node_waits"
CREATE INDEX `idx_loop_node_waits_due` ON `loop_node_waits` (`resume_at`) WHERE claim_state = 'waiting' AND resume_at IS NOT NULL;
-- create index "idx_loop_node_waits_ladder" to table: "loop_node_waits"
CREATE INDEX `idx_loop_node_waits_ladder` ON `loop_node_waits` (`next_escalation_at`) WHERE claim_state = 'waiting' AND next_escalation_at IS NOT NULL;
-- create index "idx_loop_node_waits_state" to table: "loop_node_waits"
CREATE INDEX `idx_loop_node_waits_state` ON `loop_node_waits` (`claim_state`);
-- create "loop_admission_claims" table
CREATE TABLE `loop_admission_claims` (`workspace_id` text NOT NULL, `loop_name` text NOT NULL, `source_key` text NOT NULL, `event_key` text NOT NULL, `loop_run_id` text NOT NULL, `claimed_at` timestamp NOT NULL, `suppressed_count` integer NOT NULL DEFAULT 0, `last_suppressed_at` timestamp NULL, PRIMARY KEY (`workspace_id`, `loop_name`, `source_key`, `event_key`));
-- create "loop_effect_outbox" table
CREATE TABLE `loop_effect_outbox` (`loop_run_id` text NOT NULL, `delivery_id` text NOT NULL, `source_event_id` text NOT NULL, `trigger` text NOT NULL, `generation` integer NOT NULL, `node_id` text NOT NULL DEFAULT '', `item_index` integer NOT NULL DEFAULT 0, `entry_index` integer NOT NULL, `entry_json` text NOT NULL, `state` text NOT NULL DEFAULT 'pending', `attempts` integer NOT NULL DEFAULT 0, `created_at` timestamp NOT NULL, `delivered_at` timestamp NULL, PRIMARY KEY (`loop_run_id`, `delivery_id`), CONSTRAINT `0` FOREIGN KEY (`loop_run_id`) REFERENCES `loop_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (entry_index >= 0), CHECK (json_valid(entry_json)), CHECK (state IN ('pending','delivered','failed')));
-- create index "idx_loop_effect_outbox_pending" to table: "loop_effect_outbox"
CREATE INDEX `idx_loop_effect_outbox_pending` ON `loop_effect_outbox` (`state`) WHERE state = 'pending';
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
