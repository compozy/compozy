-- +goose Up
-- create index "idx_task_runs_workspace_active" to table: "task_runs"
CREATE INDEX `idx_task_runs_workspace_active` ON `task_runs` (`workspace_id`, `status`, `lease_until`) WHERE workspace_id IS NOT NULL AND run_kind IN ('worker', 'coordinator');
