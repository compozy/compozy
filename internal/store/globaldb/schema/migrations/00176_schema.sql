-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_loop_goal_checkpoints" table
CREATE TABLE `new_loop_goal_checkpoints` (`wait_started_at` timestamp NULL, `wait_deferrals` integer NOT NULL DEFAULT 0, `pending_compaction` integer NOT NULL DEFAULT 0, `loop_run_id` text NOT NULL, `generation` integer NOT NULL, `node_id` text NOT NULL, `item_index` integer NOT NULL DEFAULT 0, `control_epoch` integer NOT NULL DEFAULT 1, `control_actor_kind` text NULL, `control_actor_id` text NULL, `control_requested_at` timestamp NULL, `phase` text NOT NULL, `goal_status` text NOT NULL, `turns_used` integer NOT NULL DEFAULT 0, `turn_limit` integer NOT NULL, `broken_streak` integer NOT NULL DEFAULT 0, `recovery_streak` integer NOT NULL DEFAULT 0, `task_run_id` text NULL, `queue_entry_id` text NULL, `prompt_id` text NULL, `prompt_kind` text NULL, `prompt_attempt` integer NOT NULL DEFAULT 0, `context_state` text NOT NULL DEFAULT 'unknown', `usage_sequence` integer NULL, `usage_pending_after_sequence` integer NULL, `session_id` text NULL, `binding_handle` text NULL, `binding_epoch` integer NULL, `context_nudge_ratio` real NOT NULL, `control_grant_id` integer NOT NULL DEFAULT 0, `control_grant_kind` text NULL, `control_grant_cause` text NULL, `control_grant_turn` integer NULL, `control_grant_scope` text NULL, `control_grant_consumed` integer NOT NULL DEFAULT 1, `judge_attempt_id` text NULL, `compaction_cancel_prompt_id` text NULL, `compaction_cancel_cause` text NULL, `compaction_cancel_requested_at` timestamp NULL, `report_prompt_id` text NULL, `report_status` text NULL, `report_evidence_ref` text NULL, `report_binding_epoch` integer NULL, `report_actor_kind` text NULL, `report_actor_id` text NULL, `report_recorded_at` timestamp NULL, `updated_at` timestamp NOT NULL, `control_cause` text NULL, `compaction_baseline_used` integer NULL, `compaction_recovery_required` integer NOT NULL DEFAULT 0, PRIMARY KEY (`loop_run_id`, `generation`, `node_id`, `item_index`), CONSTRAINT `0` FOREIGN KEY (`loop_run_id`) REFERENCES `loop_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (wait_deferrals >= 0), CHECK (pending_compaction IN (0,1)), CHECK (generation >= 1), CHECK (length(trim(node_id)) > 0), CHECK (item_index >= 0), CHECK (control_epoch >= 1), CHECK (phase IN (
		'idle','waiting','preparing','queued','prompting','compacting','judging','persisting','awaiting_control','terminal'
	)), CHECK (goal_status IN (
		'active','waiting','paused','blocked','usage-limited','budget-limited','complete'
	)), CHECK (turns_used >= 0), CHECK (turn_limit >= 1), CHECK (broken_streak >= 0), CHECK (recovery_streak >= 0), CHECK (prompt_kind IS NULL OR prompt_kind IN ('work','continuation','compact')), CHECK (prompt_attempt >= 0), CHECK (context_state IN ('known','unknown','pending')), CHECK (usage_sequence IS NULL OR usage_sequence >= 0), CHECK (usage_pending_after_sequence IS NULL OR usage_pending_after_sequence >= 0), CHECK (binding_epoch IS NULL OR binding_epoch >= 1), CHECK (context_nudge_ratio >= 0.0 AND context_nudge_ratio <= 1.0), CHECK (control_grant_id >= 0), CHECK (
		control_grant_kind IS NULL OR control_grant_kind IN ('turn-extension','budget','reseed','plain-resume')
	), CHECK (control_grant_turn IS NULL OR control_grant_turn >= 0), CHECK (
		control_grant_scope IS NULL OR control_grant_scope IN (
			'turn-limit','settle-current','work-and-settle','rotate-binding','reactivate'
		)
	), CHECK (control_grant_consumed IN (0,1)), CHECK (compaction_cancel_cause IS NULL OR compaction_cancel_cause = 'timeout'), CHECK (report_status IS NULL OR report_status IN ('complete','blocked')), CHECK (report_binding_epoch IS NULL OR report_binding_epoch >= 1), CHECK (compaction_baseline_used IS NULL OR compaction_baseline_used >= 0), CHECK (compaction_recovery_required IN (0,1)), CHECK (
		(context_state = 'known' AND usage_sequence IS NOT NULL AND usage_pending_after_sequence IS NULL)
		OR
		(context_state = 'unknown' AND usage_sequence IS NULL AND usage_pending_after_sequence IS NULL)
		OR
		(context_state = 'pending' AND (
			usage_pending_after_sequence IS NULL OR usage_pending_after_sequence = usage_sequence
		))
	), CHECK (
		(control_actor_kind IS NULL AND control_actor_id IS NULL AND control_requested_at IS NULL)
		OR
		(control_actor_kind IS NOT NULL AND control_actor_id IS NOT NULL AND control_requested_at IS NOT NULL)
	), CHECK (
		(control_grant_id = 0 AND control_grant_kind IS NULL AND control_grant_cause IS NULL
		 AND control_grant_turn IS NULL AND control_grant_scope IS NULL AND control_grant_consumed = 1)
		OR
		(control_grant_id >= 1 AND control_grant_kind IS NOT NULL AND control_grant_cause IS NOT NULL
		 AND control_grant_scope IS NOT NULL AND control_grant_turn IS NOT NULL)
	), CHECK (
		(compaction_cancel_cause IS NULL AND compaction_cancel_prompt_id IS NULL AND compaction_cancel_requested_at IS NULL)
		OR
		(compaction_cancel_cause = 'timeout' AND compaction_cancel_prompt_id IS NOT NULL
		 AND compaction_cancel_requested_at IS NOT NULL)
	), CHECK (
		(report_status IS NULL AND report_prompt_id IS NULL AND report_evidence_ref IS NULL
		 AND report_binding_epoch IS NULL AND report_actor_kind IS NULL AND report_actor_id IS NULL
		 AND report_recorded_at IS NULL)
		OR
		(report_status IS NOT NULL AND report_prompt_id IS NOT NULL AND report_binding_epoch IS NOT NULL
		 AND report_actor_kind IS NOT NULL AND report_actor_id IS NOT NULL AND report_recorded_at IS NOT NULL)
	));
-- copy rows from old table "loop_goal_checkpoints" to new temporary table "new_loop_goal_checkpoints"
INSERT INTO `new_loop_goal_checkpoints` (`loop_run_id`, `generation`, `node_id`, `item_index`, `control_epoch`, `control_actor_kind`, `control_actor_id`, `control_requested_at`, `phase`, `goal_status`, `turns_used`, `turn_limit`, `broken_streak`, `recovery_streak`, `task_run_id`, `queue_entry_id`, `prompt_id`, `prompt_kind`, `prompt_attempt`, `context_state`, `usage_sequence`, `usage_pending_after_sequence`, `session_id`, `binding_handle`, `binding_epoch`, `context_nudge_ratio`, `control_grant_id`, `control_grant_kind`, `control_grant_cause`, `control_grant_turn`, `control_grant_scope`, `control_grant_consumed`, `judge_attempt_id`, `compaction_cancel_prompt_id`, `compaction_cancel_cause`, `compaction_cancel_requested_at`, `report_prompt_id`, `report_status`, `report_evidence_ref`, `report_binding_epoch`, `report_actor_kind`, `report_actor_id`, `report_recorded_at`, `updated_at`, `control_cause`, `compaction_baseline_used`, `compaction_recovery_required`) SELECT `loop_run_id`, `generation`, `node_id`, `item_index`, `control_epoch`, `control_actor_kind`, `control_actor_id`, `control_requested_at`, `phase`, `goal_status`, `turns_used`, `turn_limit`, `broken_streak`, `recovery_streak`, `task_run_id`, `queue_entry_id`, `prompt_id`, `prompt_kind`, `prompt_attempt`, `context_state`, `usage_sequence`, `usage_pending_after_sequence`, `session_id`, `binding_handle`, `binding_epoch`, `context_nudge_ratio`, `control_grant_id`, `control_grant_kind`, `control_grant_cause`, `control_grant_turn`, `control_grant_scope`, `control_grant_consumed`, `judge_attempt_id`, `compaction_cancel_prompt_id`, `compaction_cancel_cause`, `compaction_cancel_requested_at`, `report_prompt_id`, `report_status`, `report_evidence_ref`, `report_binding_epoch`, `report_actor_kind`, `report_actor_id`, `report_recorded_at`, `updated_at`, `control_cause`, `compaction_baseline_used`, `compaction_recovery_required` FROM `loop_goal_checkpoints`;
-- drop "loop_goal_checkpoints" table after copying rows
DROP TABLE `loop_goal_checkpoints`;
-- rename temporary table "new_loop_goal_checkpoints" to "loop_goal_checkpoints"
ALTER TABLE `new_loop_goal_checkpoints` RENAME TO `loop_goal_checkpoints`;
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
