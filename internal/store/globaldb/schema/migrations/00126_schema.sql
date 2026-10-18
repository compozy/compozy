-- +goose Up
-- create index "idx_task_runs_loop_status" to table: "task_runs"
CREATE INDEX `idx_task_runs_loop_status` ON `task_runs` (`loop_run_id`, `status`, `task_id`) WHERE loop_run_id IS NOT NULL;
-- create index "idx_task_runs_coordinator_provenance" to table: "task_runs"
CREATE INDEX `idx_task_runs_coordinator_provenance` ON `task_runs` (`task_id`, `workspace_id`, `loop_run_id`) WHERE run_kind = 'coordinator';
-- create index "idx_tasks_unsettled" to table: "tasks"
CREATE INDEX `idx_tasks_unsettled` ON `tasks` (`id`) WHERE status NOT IN ('completed','failed','canceled');
