-- +goose Up
-- add column "removal_mode" to table: "worktrees"
ALTER TABLE `worktrees` ADD COLUMN `removal_mode` text NOT NULL DEFAULT '';
-- create "session_profile_moves" table
CREATE TABLE `session_profile_moves` (`session_id` text NOT NULL, `workspace_id` text NOT NULL, `profile_id` text NOT NULL, `target_profile_id` text NOT NULL, `copy_key` text NOT NULL, `source_worktree_id` text NOT NULL DEFAULT '', `branch` text NOT NULL DEFAULT '', `head_sha` text NOT NULL DEFAULT '', `target_worktree_id` text NOT NULL DEFAULT '', `restore_worktree_id` text NOT NULL DEFAULT '', `created_at` text NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`session_id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`, `session_id`) REFERENCES `sessions` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (length(trim(copy_key)) > 0));
