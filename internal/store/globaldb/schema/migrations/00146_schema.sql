-- +goose Up
-- create index "idx_loop_requests_state_expiry" to table: "loop_requests"
CREATE INDEX `idx_loop_requests_state_expiry` ON `loop_requests` (`state`, `expires_at`, `opened_at`);
-- create index "idx_loop_runs_profile_created" to table: "loop_runs"
CREATE INDEX `idx_loop_runs_profile_created` ON `loop_runs` (`profile_id`, `created_at` DESC, `id` DESC);
-- create index "idx_loop_runs_created" to table: "loop_runs"
CREATE INDEX `idx_loop_runs_created` ON `loop_runs` (`created_at` DESC, `id` DESC);
