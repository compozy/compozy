-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_loop_goal_judge_attempts" table
CREATE TABLE `new_loop_goal_judge_attempts` (`attempt_id` text NULL, `loop_run_id` text NOT NULL, `generation` integer NOT NULL, `node_id` text NOT NULL, `item_index` integer NOT NULL DEFAULT 0, `turn` integer NOT NULL, `judge_digest` text NOT NULL, `status` text NOT NULL, `outcome` text NULL, `blocking_json` text NOT NULL DEFAULT '[]', `criteria_json` text NOT NULL DEFAULT '[]', `warnings_json` text NOT NULL DEFAULT '[]', `evidence_ref` text NULL, `tokens_used` integer NULL, `usage_base_tokens` integer NOT NULL DEFAULT 0, `started_at` timestamp NOT NULL, `completed_at` timestamp NULL, PRIMARY KEY (`attempt_id`), CONSTRAINT `0` FOREIGN KEY (`loop_run_id`) REFERENCES `loop_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (length(trim(attempt_id)) > 0), CHECK (generation >= 1), CHECK (length(trim(node_id)) > 0), CHECK (item_index >= 0), CHECK (turn >= 1), CHECK (length(trim(judge_digest)) > 0), CHECK (status IN ('running','completed','ambiguous')), CHECK (
		outcome IS NULL OR outcome IN ('approved','rejected','blocked','error','timeout','invalid_output')
	), CHECK (json_valid(blocking_json)), CHECK (json_valid(criteria_json)), CHECK (json_valid(warnings_json)), CHECK (tokens_used IS NULL OR tokens_used >= 0), CHECK (usage_base_tokens >= 0), CHECK (completed_at IS NULL OR completed_at >= started_at), CHECK (
		(status = 'running' AND outcome IS NULL AND completed_at IS NULL)
		OR
		(status = 'completed' AND outcome IS NOT NULL AND completed_at IS NOT NULL)
		OR
		(status = 'ambiguous' AND outcome IS NULL AND completed_at IS NOT NULL)
	));
-- copy rows from old table "loop_goal_judge_attempts" to new temporary table "new_loop_goal_judge_attempts"
INSERT INTO `new_loop_goal_judge_attempts` (`attempt_id`, `loop_run_id`, `generation`, `node_id`, `item_index`, `turn`, `judge_digest`, `status`, `outcome`, `blocking_json`, `evidence_ref`, `tokens_used`, `usage_base_tokens`, `started_at`, `completed_at`) SELECT `attempt_id`, `loop_run_id`, `generation`, `node_id`, `item_index`, `turn`, `judge_digest`, `status`, `outcome`, `blocking_json`, `evidence_ref`, `tokens_used`, `usage_base_tokens`, `started_at`, `completed_at` FROM `loop_goal_judge_attempts`;
-- drop "loop_goal_judge_attempts" table after copying rows
DROP TABLE `loop_goal_judge_attempts`;
-- rename temporary table "new_loop_goal_judge_attempts" to "loop_goal_judge_attempts"
ALTER TABLE `new_loop_goal_judge_attempts` RENAME TO `loop_goal_judge_attempts`;
-- create index "loop_goal_judge_attempts_loop_run_id_generation_node_id_item_index_turn" to table: "loop_goal_judge_attempts"
CREATE UNIQUE INDEX `loop_goal_judge_attempts_loop_run_id_generation_node_id_item_index_turn` ON `loop_goal_judge_attempts` (`loop_run_id`, `generation`, `node_id`, `item_index`, `turn`);
-- create "new_loop_goal_turns" table
CREATE TABLE `new_loop_goal_turns` (`loop_run_id` text NOT NULL, `seq` integer NOT NULL, `generation` integer NOT NULL, `node_id` text NOT NULL, `item_index` integer NOT NULL DEFAULT 0, `turn` integer NOT NULL, `session_id` text NOT NULL, `binding_handle` text NOT NULL, `binding_epoch` integer NOT NULL, `prompt_id` text NOT NULL, `prompt_attempt` integer NOT NULL DEFAULT 0, `usage_base_tokens` integer NOT NULL DEFAULT 0, `result_status` text NULL, `stop_reason` text NULL, `reason_code` text NULL, `verdict_outcome` text NULL, `blocking_json` text NOT NULL DEFAULT '[]', `criteria_json` text NOT NULL DEFAULT '[]', `warnings_json` text NOT NULL DEFAULT '[]', `evidence_ref` text NULL, `prompt_ref` text NULL, `tokens_used` integer NULL, `actor_kind` text NOT NULL, `actor_id` text NOT NULL, `started_at` timestamp NOT NULL, `ended_at` timestamp NULL, PRIMARY KEY (`loop_run_id`, `generation`, `node_id`, `item_index`, `turn`), CONSTRAINT `0` FOREIGN KEY (`loop_run_id`) REFERENCES `loop_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (seq >= 1), CHECK (generation >= 1), CHECK (length(trim(node_id)) > 0), CHECK (item_index >= 0), CHECK (turn >= 1), CHECK (length(trim(session_id)) > 0), CHECK (length(trim(binding_handle)) > 0), CHECK (binding_epoch >= 1), CHECK (length(trim(prompt_id)) > 0), CHECK (prompt_attempt >= 0), CHECK (usage_base_tokens >= 0), CHECK (
		result_status IS NULL OR result_status IN ('completed','invalid-result','failed','ambiguous')
	), CHECK (
		stop_reason IS NULL OR stop_reason IN ('end_turn','max_tokens','max_turn_requests','refusal','cancelled')
	), CHECK (
		verdict_outcome IS NULL OR verdict_outcome IN ('approved','rejected','blocked','error','timeout','invalid_output')
	), CHECK (json_valid(blocking_json)), CHECK (json_valid(criteria_json)), CHECK (json_valid(warnings_json)), CHECK (tokens_used IS NULL OR tokens_used >= 0), CHECK (length(trim(actor_kind)) > 0), CHECK (length(trim(actor_id)) > 0), CHECK (ended_at IS NULL OR ended_at >= started_at), CHECK (
		(result_status IS NULL AND ended_at IS NULL AND stop_reason IS NULL
		 AND reason_code IS NULL AND verdict_outcome IS NULL)
		OR
		(result_status = 'completed' AND ended_at IS NOT NULL AND stop_reason IS NOT NULL AND reason_code IS NULL)
		OR
		(result_status = 'invalid-result' AND ended_at IS NOT NULL AND stop_reason IS NULL
		 AND reason_code = 'goal_stop_reason_invalid' AND verdict_outcome IS NULL)
		OR
		(result_status = 'failed' AND ended_at IS NOT NULL AND stop_reason IS NULL
		 AND reason_code = 'goal_prompt_request_failed' AND verdict_outcome IS NULL)
		OR
		(result_status = 'ambiguous' AND ended_at IS NOT NULL AND stop_reason IS NULL
		 AND reason_code IN ('goal_recovery_ambiguous','goal_control_revoked_in_flight')
		 AND verdict_outcome IS NULL)
	));
-- copy rows from old table "loop_goal_turns" to new temporary table "new_loop_goal_turns"
INSERT INTO `new_loop_goal_turns` (`loop_run_id`, `seq`, `generation`, `node_id`, `item_index`, `turn`, `session_id`, `binding_handle`, `binding_epoch`, `prompt_id`, `prompt_attempt`, `usage_base_tokens`, `result_status`, `stop_reason`, `reason_code`, `verdict_outcome`, `blocking_json`, `evidence_ref`, `prompt_ref`, `tokens_used`, `actor_kind`, `actor_id`, `started_at`, `ended_at`) SELECT `loop_run_id`, `seq`, `generation`, `node_id`, `item_index`, `turn`, `session_id`, `binding_handle`, `binding_epoch`, `prompt_id`, `prompt_attempt`, `usage_base_tokens`, `result_status`, `stop_reason`, `reason_code`, `verdict_outcome`, `blocking_json`, `evidence_ref`, `prompt_ref`, `tokens_used`, `actor_kind`, `actor_id`, `started_at`, `ended_at` FROM `loop_goal_turns`;
-- drop "loop_goal_turns" table after copying rows
DROP TABLE `loop_goal_turns`;
-- rename temporary table "new_loop_goal_turns" to "loop_goal_turns"
ALTER TABLE `new_loop_goal_turns` RENAME TO `loop_goal_turns`;
-- create index "loop_goal_turns_loop_run_id_seq" to table: "loop_goal_turns"
CREATE UNIQUE INDEX `loop_goal_turns_loop_run_id_seq` ON `loop_goal_turns` (`loop_run_id`, `seq`);
-- create index "loop_goal_turns_loop_run_id_prompt_id" to table: "loop_goal_turns"
CREATE UNIQUE INDEX `loop_goal_turns_loop_run_id_prompt_id` ON `loop_goal_turns` (`loop_run_id`, `prompt_id`);
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
