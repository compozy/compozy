-- +goose Up
-- add column "create_error" to table: "worktrees"
ALTER TABLE `worktrees` ADD COLUMN `create_error` text NOT NULL DEFAULT '';
