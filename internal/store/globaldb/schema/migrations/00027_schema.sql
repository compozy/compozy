-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_loop_config" table
CREATE TABLE `new_loop_config` (`workspace_id` text NOT NULL, `loop_name` text NOT NULL, `human_gate_enabled` integer NOT NULL DEFAULT 0, `reattempt_strategy` text NULL, `enabled_checks_json` text NOT NULL DEFAULT '{}', `iteration_cap` integer NULL, `budget_tokens` integer NULL, `budget_wall_sec` integer NULL, `budget_on_exceeded` text NULL, `no_progress_window` integer NULL, `fan_out_width` integer NULL, `gate_max_revisions` integer NULL, `runtime_defaults_json` text NULL, `runtime_rules_json` text NULL, PRIMARY KEY (`workspace_id`, `loop_name`), CHECK (runtime_defaults_json IS NULL OR json_valid(runtime_defaults_json)), CHECK (runtime_rules_json IS NULL OR json_valid(runtime_rules_json)));
-- copy rows from old table "loop_config" to new temporary table "new_loop_config"
INSERT INTO `new_loop_config` (`workspace_id`, `loop_name`, `human_gate_enabled`, `reattempt_strategy`, `enabled_checks_json`, `iteration_cap`, `budget_tokens`, `budget_wall_sec`, `budget_on_exceeded`, `no_progress_window`, `fan_out_width`, `gate_max_revisions`) SELECT `workspace_id`, `loop_name`, `human_gate_enabled`, `reattempt_strategy`, `enabled_checks_json`, `iteration_cap`, `budget_tokens`, `budget_wall_sec`, `budget_on_exceeded`, `no_progress_window`, `fan_out_width`, `gate_max_revisions` FROM `loop_config`;
-- drop "loop_config" table after copying rows
DROP TABLE `loop_config`;
-- rename temporary table "new_loop_config" to "loop_config"
ALTER TABLE `new_loop_config` RENAME TO `loop_config`;
-- create "new_loop_generation_outputs" table
CREATE TABLE `new_loop_generation_outputs` (`loop_run_id` text NOT NULL, `generation` integer NOT NULL, `node_id` text NOT NULL, `item_index` integer NOT NULL DEFAULT 0, `status` text NOT NULL, `output_ref` text NULL, `task_run_id` text NULL, `child_loop_run_id` text NULL, `resolved_runtime_json` text NULL, `goal_status` text NULL, `goal_turns_used` integer NULL, `goal_turn_limit` integer NULL, PRIMARY KEY (`loop_run_id`, `generation`, `node_id`, `item_index`), CONSTRAINT `0` FOREIGN KEY (`loop_run_id`) REFERENCES `loop_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (resolved_runtime_json IS NULL OR json_valid(resolved_runtime_json)));
-- copy rows from old table "loop_generation_outputs" to new temporary table "new_loop_generation_outputs"
INSERT INTO `new_loop_generation_outputs` (`loop_run_id`, `generation`, `node_id`, `item_index`, `status`, `output_ref`, `task_run_id`, `child_loop_run_id`, `goal_status`, `goal_turns_used`, `goal_turn_limit`) SELECT `loop_run_id`, `generation`, `node_id`, `item_index`, `status`, `output_ref`, `task_run_id`, `child_loop_run_id`, `goal_status`, `goal_turns_used`, `goal_turn_limit` FROM `loop_generation_outputs`;
-- drop "loop_generation_outputs" table after copying rows
DROP TABLE `loop_generation_outputs`;
-- rename temporary table "new_loop_generation_outputs" to "loop_generation_outputs"
ALTER TABLE `new_loop_generation_outputs` RENAME TO `loop_generation_outputs`;
-- create index "idx_loop_generation_outputs_output_ref" to table: "loop_generation_outputs"
CREATE INDEX `idx_loop_generation_outputs_output_ref` ON `loop_generation_outputs` (`output_ref`);
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
