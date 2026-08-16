-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_loop_generations" table
CREATE TABLE `new_loop_generations` (`loop_run_id` text NOT NULL, `generation` integer NOT NULL, `parent_generation` integer NOT NULL DEFAULT 0, `origin` text NOT NULL, `created_at` timestamp NOT NULL, PRIMARY KEY (`loop_run_id`, `generation`), CONSTRAINT `0` FOREIGN KEY (`loop_run_id`) REFERENCES `loop_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (generation >= 1), CHECK (parent_generation >= 0 AND parent_generation < generation), CHECK (origin IN (
				'initial','stop_when','reattempt','gate_revise','gate_next_generation',
				'dod_retry','ratchet_restore','requeue','operator_rerun'
			)));
-- copy rows from old table "loop_generations" to new temporary table "new_loop_generations"
INSERT INTO `new_loop_generations` (`loop_run_id`, `generation`, `parent_generation`, `origin`, `created_at`) SELECT `loop_run_id`, `generation`, `parent_generation`, `origin`, `created_at` FROM `loop_generations`;
-- drop "loop_generations" table after copying rows
DROP TABLE `loop_generations`;
-- rename temporary table "new_loop_generations" to "loop_generations"
ALTER TABLE `new_loop_generations` RENAME TO `loop_generations`;
-- create "loop_timetravel_ops" table
CREATE TABLE `loop_timetravel_ops` (`workspace_id` text NOT NULL, `op_id` text NOT NULL, `kind` text NOT NULL, `idempotency_key` text NOT NULL DEFAULT '', `request_digest` text NOT NULL, `source_run_id` text NOT NULL, `source_generation` integer NULL, `from_node` text NULL, `item_index` integer NULL, `actor_kind` text NOT NULL, `actor_id` text NOT NULL, `reason` text NULL, `result_run_id` text NOT NULL, `result_generation` integer NULL, `created_at` timestamp NOT NULL, PRIMARY KEY (`workspace_id`, `op_id`), CONSTRAINT `0` FOREIGN KEY (`source_run_id`) REFERENCES `loop_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (kind IN ('rerun','fork')), CHECK (length(request_digest) = 64), CHECK (source_generation IS NULL OR source_generation >= 1), CHECK (item_index IS NULL OR item_index >= 0), CHECK (result_generation IS NULL OR result_generation >= 1), CHECK (
		(kind = 'rerun' AND source_generation IS NOT NULL AND from_node IS NOT NULL
		 AND result_generation IS NOT NULL AND result_run_id = source_run_id)
		OR
		(kind = 'fork' AND source_generation IS NOT NULL AND from_node IS NULL
		 AND item_index IS NULL AND result_generation IS NULL)
	));
-- create index "uq_loop_timetravel_ops_idempotency" to table: "loop_timetravel_ops"
CREATE UNIQUE INDEX `uq_loop_timetravel_ops_idempotency` ON `loop_timetravel_ops` (`workspace_id`, `idempotency_key`) WHERE idempotency_key != '';
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
