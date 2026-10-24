-- +goose Up
-- add column "branch" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `branch` text NOT NULL DEFAULT '';
-- create "forge_pull_requests" table
CREATE TABLE `forge_pull_requests` (`workspace_id` text NOT NULL, `repo_key` text NOT NULL, `branch` text NOT NULL, `provider` text NOT NULL DEFAULT '', `pr_number` integer NULL, `pr_url` text NOT NULL DEFAULT '', `pr_state` text NULL, `merged` integer NULL, `draft` integer NULL, `head_sha` text NOT NULL DEFAULT '', `base_ref` text NOT NULL DEFAULT '', `checks_state` text NOT NULL DEFAULT 'unknown', `checks_passed` integer NOT NULL DEFAULT 0, `checks_failed` integer NOT NULL DEFAULT 0, `checks_pending` integer NOT NULL DEFAULT 0, `checks_failed_names` text NOT NULL DEFAULT '[]', `review_decision` text NOT NULL DEFAULT 'unknown', `mergeable` text NOT NULL DEFAULT 'unknown', `pr_updated_at` text NULL, `observed_at` text NULL, `lookup_state` text NOT NULL DEFAULT 'pending', `lookup_cause` text NOT NULL DEFAULT '', `attempt_at` text NULL, `failure_count` integer NOT NULL DEFAULT 0, `retry_after` text NULL, PRIMARY KEY (`workspace_id`, `repo_key`, `branch`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (length(repo_key) > 0), CHECK (length(branch) > 0), CHECK (pr_number IS NULL OR pr_number > 0), CHECK (pr_state IS NULL OR pr_state IN ('open', 'closed', 'merged')), CHECK (merged IS NULL OR merged IN (0, 1)), CHECK (draft IS NULL OR draft IN (0, 1)), CHECK (checks_state IN ('passing', 'failing', 'pending', 'none', 'unknown')), CHECK (checks_passed >= 0), CHECK (checks_failed >= 0), CHECK (checks_pending >= 0), CHECK (json_valid(checks_failed_names)), CHECK (lookup_state IN ('ok', 'pending', 'error', 'incomplete')), CHECK (failure_count >= 0));

-- Preserve identity from each historical PR URL, never from the current origin.
WITH urls AS (
 SELECT f.*, w.workspace_id, w.branch, substr(f.pr_url, 9) AS remainder
 FROM worktree_forge_status f JOIN worktrees w ON w.id = f.worktree_id
 WHERE w.branch <> '' AND f.pr_url LIKE 'https://%/pull/%'
   AND f.pr_url NOT GLOB '*[?#@%]*'
), paths AS (
 SELECT *, substr(remainder, 1, instr(remainder, '/') - 1) AS host,
   substr(remainder, instr(remainder, '/') + 1) AS path
 FROM urls WHERE instr(remainder, '/') > 1
), owners AS (
 SELECT *, substr(path, 1, instr(path, '/') - 1) AS owner,
   substr(path, instr(path, '/') + 1) AS repo_path
 FROM paths WHERE instr(path, '/') > 1
), repos AS (
 SELECT *, substr(repo_path, 1, instr(repo_path, '/') - 1) AS repo,
   substr(repo_path, instr(repo_path, '/') + 1) AS pull_path
 FROM owners WHERE instr(repo_path, '/') > 1
), ranked AS (
 SELECT *, lower(host || '/' || owner || '/' || repo) AS repo_key,
   row_number() OVER (
     PARTITION BY workspace_id, lower(host || '/' || owner || '/' || repo), branch
     ORDER BY fetched_at DESC, worktree_id ASC
   ) AS position
 FROM repos
 WHERE pull_path GLOB 'pull/[0-9]*'
   AND substr(pull_path, 6) NOT GLOB '*[^0-9]*'
   AND CAST(substr(pull_path, 6) AS INTEGER) > 0
   AND host NOT LIKE '%:%' AND owner NOT IN ('.', '..') AND repo NOT IN ('.', '..')
)
INSERT INTO forge_pull_requests (
 workspace_id, repo_key, branch, provider, pr_number, pr_url, pr_state, merged,
 observed_at, lookup_state
)
SELECT workspace_id, repo_key, branch, provider, pr_number, pr_url, pr_state, merged,
 fetched_at, 'ok'
FROM ranked WHERE position = 1;
ALTER TABLE worktree_forge_status RENAME TO worktree_forge_status_legacy;
