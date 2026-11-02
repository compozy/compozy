-- +goose Up
-- create index "idx_automation_runs_session" to table: "automation_runs"
CREATE INDEX `idx_automation_runs_session` ON `automation_runs` (`session_id`) WHERE session_id IS NOT NULL;
