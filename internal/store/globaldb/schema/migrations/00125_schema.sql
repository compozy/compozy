-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_worktree_exit_ops" table
CREATE TABLE `new_worktree_exit_ops` (`op_id` text NULL, `workspace_id` text NOT NULL, `worktree_id` text NOT NULL, `action` text NOT NULL, `state` text NOT NULL, `started_at` text NOT NULL, `finished_at` text NULL, PRIMARY KEY (`op_id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`, `worktree_id`) REFERENCES `worktrees` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (action IN ('commit', 'commit_push', 'push', 'open_pr', 'deliver')), CHECK (state IN ('running', 'completed', 'failed', 'canceled')));
-- copy rows from old table "worktree_exit_ops" to new temporary table "new_worktree_exit_ops"
INSERT INTO `new_worktree_exit_ops` (`op_id`, `workspace_id`, `worktree_id`, `action`, `state`, `started_at`, `finished_at`) SELECT `op_id`, `workspace_id`, `worktree_id`, `action`, `state`, `started_at`, `finished_at` FROM `worktree_exit_ops`;
-- drop "worktree_exit_ops" table after copying rows
DROP TABLE `worktree_exit_ops`;
-- rename temporary table "new_worktree_exit_ops" to "worktree_exit_ops"
ALTER TABLE `new_worktree_exit_ops` RENAME TO `worktree_exit_ops`;
-- create index "idx_worktree_exit_ops_active" to table: "worktree_exit_ops"
CREATE UNIQUE INDEX `idx_worktree_exit_ops_active` ON `worktree_exit_ops` (`worktree_id`) WHERE state = 'running';
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
