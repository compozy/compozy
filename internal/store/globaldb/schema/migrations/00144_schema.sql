-- +goose Up
-- create index "idx_tasks_profile" to table: "tasks"
CREATE INDEX `idx_tasks_profile` ON `tasks` (`profile_id`);
