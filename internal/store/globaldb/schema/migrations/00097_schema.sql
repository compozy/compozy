-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_task_execution_profiles" table
CREATE TABLE `new_task_execution_profiles` (`task_id` text NULL, `coordinator_mode` text NOT NULL DEFAULT 'inherit', `coordinator_agent_name` text NOT NULL DEFAULT '', `coordinator_provider` text NOT NULL DEFAULT '', `coordinator_model` text NOT NULL DEFAULT '', `coordinator_guidance` text NOT NULL DEFAULT '', `worker_mode` text NOT NULL DEFAULT 'inherit', `worker_agent_name` text NOT NULL DEFAULT '', `worker_provider` text NOT NULL DEFAULT '', `worker_model` text NOT NULL DEFAULT '', `worker_reasoning_effort` text NOT NULL DEFAULT '', `worker_speed` text NOT NULL DEFAULT '', `worker_acp_options_json` text NOT NULL DEFAULT '[]', `review_agent_name` text NOT NULL DEFAULT '', `review_provider` text NOT NULL DEFAULT '', `review_model` text NOT NULL DEFAULT '', `review_reasoning_effort` text NOT NULL DEFAULT '', `review_speed` text NOT NULL DEFAULT '', `review_acp_options_json` text NOT NULL DEFAULT '[]', `sandbox_mode` text NOT NULL DEFAULT 'inherit', `sandbox_ref` text NOT NULL DEFAULT '', `worktree_mode` text NOT NULL DEFAULT 'inherit', `worktree_ref` text NOT NULL DEFAULT '', `created_at` text NOT NULL, `updated_at` text NOT NULL, `runtime_mode` text NOT NULL DEFAULT 'default', `network_mode` text NOT NULL DEFAULT '', `network_channel_strategy` text NULL, `network_channel` text NULL, `network_bounds_json` text NULL, PRIMARY KEY (`task_id`), CONSTRAINT `0` FOREIGN KEY (`task_id`) REFERENCES `tasks` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (
				coordinator_mode IN ('inherit', 'guided')
			), CHECK (
				worker_mode IN ('inherit', 'select')
			), CHECK (worker_speed IN ('', 'normal', 'fast')), CHECK (json_valid(worker_acp_options_json)), CHECK (review_speed IN ('', 'normal', 'fast')), CHECK (json_valid(review_acp_options_json)), CHECK (
				sandbox_mode IN ('inherit', 'none', 'ref')
			), CHECK (
				worktree_mode IN ('inherit', 'none', 'ref', 'per_run')
			), CHECK (
				runtime_mode IN ('default', 'evidence')
			), CHECK (network_mode IN ('', 'local', 'live')), CHECK (
				network_channel_strategy IS NULL OR network_channel_strategy IN ('named', 'run', 'loop_run')
			), CHECK (
				network_bounds_json IS NULL OR json_valid(network_bounds_json)
			), CHECK (
				(sandbox_mode = 'ref' AND sandbox_ref <> '') OR
				(sandbox_mode <> 'ref' AND sandbox_ref = '')
			), CHECK (
				(worktree_mode = 'ref') = (worktree_ref <> '')
			));
-- copy rows from old table "task_execution_profiles" to new temporary table "new_task_execution_profiles"
INSERT INTO `new_task_execution_profiles` (`task_id`, `coordinator_mode`, `coordinator_agent_name`, `coordinator_provider`, `coordinator_model`, `coordinator_guidance`, `worker_mode`, `worker_agent_name`, `worker_provider`, `worker_model`, `review_agent_name`, `review_provider`, `review_model`, `sandbox_mode`, `sandbox_ref`, `worktree_mode`, `worktree_ref`, `created_at`, `updated_at`, `runtime_mode`, `network_mode`, `network_channel_strategy`, `network_channel`, `network_bounds_json`) SELECT `task_id`, `coordinator_mode`, `coordinator_agent_name`, `coordinator_provider`, `coordinator_model`, `coordinator_guidance`, `worker_mode`, `worker_agent_name`, `worker_provider`, `worker_model`, `review_agent_name`, `review_provider`, `review_model`, `sandbox_mode`, `sandbox_ref`, `worktree_mode`, `worktree_ref`, `created_at`, `updated_at`, `runtime_mode`, `network_mode`, `network_channel_strategy`, `network_channel`, `network_bounds_json` FROM `task_execution_profiles`;
-- drop "task_execution_profiles" table after copying rows
DROP TABLE `task_execution_profiles`;
-- rename temporary table "new_task_execution_profiles" to "task_execution_profiles"
ALTER TABLE `new_task_execution_profiles` RENAME TO `task_execution_profiles`;
-- create index "task_execution_profiles_task_id_idx" to table: "task_execution_profiles"
CREATE INDEX `task_execution_profiles_task_id_idx` ON `task_execution_profiles` (`task_id`);
-- drop index "idx_model_catalog_binding_selections_binding" from table: "model_catalog_transport_binding_selections"
DROP INDEX `idx_model_catalog_binding_selections_binding`;
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
