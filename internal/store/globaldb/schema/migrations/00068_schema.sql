-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_loop_config" table
CREATE TABLE `new_loop_config` (`workspace_id` text NOT NULL, `loop_name` text NOT NULL, `human_gate_enabled` integer NOT NULL DEFAULT 0, `reattempt_strategy` text NULL, `enabled_checks_json` text NOT NULL DEFAULT '{}', `iteration_cap` integer NULL, `budget_tokens` integer NULL, `budget_wall_sec` integer NULL, `budget_on_exceeded` text NULL, `no_progress_window` integer NULL, `fan_out_width` integer NULL, `gate_max_revisions` integer NULL, `runtime_defaults_json` text NULL, `runtime_rules_json` text NULL, `environment_json` text NULL, PRIMARY KEY (`workspace_id`, `loop_name`), CHECK (runtime_defaults_json IS NULL OR json_valid(runtime_defaults_json)), CHECK (runtime_rules_json IS NULL OR json_valid(runtime_rules_json)), CHECK (environment_json IS NULL OR json_valid(environment_json)));
-- copy rows from old table "loop_config" to new temporary table "new_loop_config"
INSERT INTO `new_loop_config` (`workspace_id`, `loop_name`, `human_gate_enabled`, `reattempt_strategy`, `enabled_checks_json`, `iteration_cap`, `budget_tokens`, `budget_wall_sec`, `budget_on_exceeded`, `no_progress_window`, `fan_out_width`, `gate_max_revisions`, `runtime_defaults_json`, `runtime_rules_json`) SELECT `workspace_id`, `loop_name`, `human_gate_enabled`, `reattempt_strategy`, `enabled_checks_json`, `iteration_cap`, `budget_tokens`, `budget_wall_sec`, `budget_on_exceeded`, `no_progress_window`, `fan_out_width`, `gate_max_revisions`, `runtime_defaults_json`, `runtime_rules_json` FROM `loop_config`;
-- drop "loop_config" table after copying rows
DROP TABLE `loop_config`;
-- rename temporary table "new_loop_config" to "loop_config"
ALTER TABLE `new_loop_config` RENAME TO `loop_config`;
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
