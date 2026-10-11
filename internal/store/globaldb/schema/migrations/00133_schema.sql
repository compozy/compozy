-- +goose Up
-- create index "idx_loop_runs_session_origin_recent" to table: "loop_runs"
CREATE INDEX `idx_loop_runs_session_origin_recent` ON `loop_runs` (`workspace_id`, `origin_session_id`, `created_at`) WHERE origin_kind = 'session' AND historical = 0;
