-- +goose Up
-- add column "create_error" to table: "worktrees"
ALTER TABLE `worktrees` ADD COLUMN `create_error` text NOT NULL DEFAULT '';
-- add column "create_rollback_error" to table: "worktrees"
ALTER TABLE `worktrees` ADD COLUMN `create_rollback_error` text NOT NULL DEFAULT '';
