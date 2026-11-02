-- +goose Up
-- add column "title" to table: "forge_pull_requests"
ALTER TABLE `forge_pull_requests` ADD COLUMN `title` text NOT NULL DEFAULT '';
-- create "session_work_links" table
CREATE TABLE `session_work_links` (`session_id` text NOT NULL, `workspace_id` text NOT NULL, `repo_key` text NOT NULL, `branch` text NOT NULL, `pr_number` integer NULL, `worktree_id` text NOT NULL DEFAULT '', `worktree_name` text NOT NULL DEFAULT '', `provenance` text NOT NULL, `unlinked` integer NOT NULL DEFAULT 0, PRIMARY KEY (`session_id`, `repo_key`, `branch`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`, `session_id`) REFERENCES `sessions` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (length(trim(repo_key)) > 0), CHECK (length(trim(branch)) > 0), CHECK (pr_number IS NULL OR pr_number > 0), CHECK (provenance IN ('created', 'user', 'agent')), CHECK (unlinked IN (0,1)));
-- create index "idx_session_work_links_workspace" to table: "session_work_links"
CREATE INDEX `idx_session_work_links_workspace` ON `session_work_links` (`workspace_id`, `repo_key`, `branch`);
