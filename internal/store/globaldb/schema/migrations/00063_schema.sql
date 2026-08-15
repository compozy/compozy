-- +goose Up
-- drop index "worktrees_workspace_id_name" from table: "worktrees"
DROP INDEX `worktrees_workspace_id_name`;
-- create index "idx_worktrees_reserved_name" to table: "worktrees"
CREATE UNIQUE INDEX `idx_worktrees_reserved_name` ON `worktrees` (`workspace_id`, `name`) WHERE state <> 'dismissed';
