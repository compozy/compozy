-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- Greenfield repair for the v37 loop-lineage cut: v37 cleared ownership before deleting
-- loop_runs. Remove the now-identifiable loop execution rows so no coordinator or worker can
-- escape into an ordinary claim queue and no delegated automation run lacks its target.
DELETE FROM task_runs
WHERE loop_run_id IS NULL
  AND (
    run_kind = 'coordinator'
    OR (
      run_kind = 'worker'
      AND json_valid(metadata_json)
      AND json_type(metadata_json, '$.generation') = 'integer'
      AND json_type(metadata_json, '$.node_id') = 'text'
      AND json_type(metadata_json, '$.item_index') = 'integer'
    )
  );
DELETE FROM automation_runs
WHERE status = 'delegated'
  AND task_run_id IS NULL
  AND loop_run_id IS NULL;
-- create "new_loop_gate_verdicts" table
CREATE TABLE `new_loop_gate_verdicts` (`loop_run_id` text NOT NULL, `generation` integer NOT NULL, `gate_id` text NOT NULL, `item_index` integer NOT NULL DEFAULT 0, `outcome` text NOT NULL, `score` real NULL, `route_cause_rank` integer NULL, `blocking_issues_json` text NOT NULL DEFAULT '[]', `criteria_json` text NOT NULL DEFAULT '[]', `decided_at` timestamp NOT NULL, PRIMARY KEY (`loop_run_id`, `generation`, `gate_id`, `item_index`), CONSTRAINT `0` FOREIGN KEY (`loop_run_id`) REFERENCES `loop_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (generation >= 1), CHECK (item_index >= 0), CHECK (outcome IN (
				'approved','rejected','awaiting_approval','blocked','error','timeout','invalid_output'
			)), CHECK (json_valid(blocking_issues_json)), CHECK (json_valid(criteria_json)));
-- copy rows from old table "loop_gate_verdicts" to new temporary table "new_loop_gate_verdicts"
INSERT INTO `new_loop_gate_verdicts` (`loop_run_id`, `generation`, `gate_id`, `outcome`, `score`, `route_cause_rank`, `blocking_issues_json`, `criteria_json`, `decided_at`) SELECT `loop_run_id`, `generation`, `gate_id`, `outcome`, `score`, `route_cause_rank`, `blocking_issues_json`, `criteria_json`, `decided_at` FROM `loop_gate_verdicts`;
-- drop "loop_gate_verdicts" table after copying rows
DROP TABLE `loop_gate_verdicts`;
-- rename temporary table "new_loop_gate_verdicts" to "loop_gate_verdicts"
ALTER TABLE `new_loop_gate_verdicts` RENAME TO `loop_gate_verdicts`;
-- create index "idx_loop_gate_verdicts_route_cause" to table: "loop_gate_verdicts"
CREATE INDEX `idx_loop_gate_verdicts_route_cause` ON `loop_gate_verdicts` (`loop_run_id`, `generation`, `route_cause_rank`) WHERE route_cause_rank IS NOT NULL;
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
