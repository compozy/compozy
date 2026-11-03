-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_worktrees" table
CREATE TABLE `new_worktrees` (`id` text NULL, `profile_id` text NOT NULL, `workspace_id` text NOT NULL, `name` text NOT NULL, `branch` text NOT NULL DEFAULT '', `path` text NOT NULL, `git_dir` text NOT NULL DEFAULT '', `state` text NOT NULL, `pending_phase` text NOT NULL DEFAULT '', `origin` text NOT NULL, `setup_state` text NOT NULL DEFAULT 'none', `setup_error` text NOT NULL DEFAULT '', `create_error` text NOT NULL DEFAULT '', `create_rollback_error` text NOT NULL DEFAULT '', `base_ref` text NOT NULL DEFAULT '', `created_branch` integer NOT NULL DEFAULT 0, `run_namespace` text NOT NULL DEFAULT '', `created_head` text NOT NULL DEFAULT '', `run_id` text NOT NULL DEFAULT '', `created_at` text NOT NULL, `updated_at` text NOT NULL, `removal_mode` text NOT NULL DEFAULT '', PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (
			state IN ('pending', 'ready', 'failed', 'removing', 'missing', 'removed', 'dismissed')
		), CHECK (
			pending_phase IN ('', 'branch', 'checkout', 'copy', 'setup')
		), CHECK (origin IN ('manual', 'per_run', 'adopted')), CHECK (setup_state IN ('none', 'ok', 'failed')), CHECK (created_branch IN (0, 1)), CHECK (removal_mode IN ('', 'keep_branch')));
-- copy rows from old table "worktrees" to new temporary table "new_worktrees"
INSERT INTO `new_worktrees` (`id`, `profile_id`, `workspace_id`, `name`, `branch`, `path`, `git_dir`, `state`, `pending_phase`, `origin`, `setup_state`, `setup_error`, `create_error`, `create_rollback_error`, `base_ref`, `created_branch`, `run_namespace`, `created_head`, `run_id`, `created_at`, `updated_at`) SELECT `id`, `profile_id`, `workspace_id`, `name`, `branch`, `path`, `git_dir`, `state`, `pending_phase`, `origin`, `setup_state`, `setup_error`, `create_error`, `create_rollback_error`, `base_ref`, `created_branch`, `run_namespace`, `created_head`, `run_id`, `created_at`, `updated_at` FROM `worktrees`;
-- drop trigger "worktrees_profile_owner_active" before applying its declarative change
DROP TRIGGER IF EXISTS `worktrees_profile_owner_active`;
-- drop trigger "worktrees_profile_owner_immutable" before applying its declarative change
DROP TRIGGER IF EXISTS `worktrees_profile_owner_immutable`;
-- drop "worktrees" table after copying rows
DROP TABLE `worktrees`;
-- rename temporary table "new_worktrees" to "worktrees"
ALTER TABLE `new_worktrees` RENAME TO `worktrees`;
-- create index "worktrees_workspace_id_id" to table: "worktrees"
CREATE UNIQUE INDEX `worktrees_workspace_id_id` ON `worktrees` (`workspace_id`, `id`);
-- create index "idx_worktrees_workspace_state" to table: "worktrees"
CREATE INDEX `idx_worktrees_workspace_state` ON `worktrees` (`workspace_id`, `state`);
-- create index "idx_worktrees_reserved_name" to table: "worktrees"
CREATE UNIQUE INDEX `idx_worktrees_reserved_name` ON `worktrees` (`workspace_id`, `name`) WHERE state <> 'dismissed';
-- create index "idx_worktrees_live_path" to table: "worktrees"
CREATE UNIQUE INDEX `idx_worktrees_live_path` ON `worktrees` (`path`) WHERE state IN ('pending', 'ready', 'removing');
-- create "session_profile_moves" table
CREATE TABLE `session_profile_moves` (`session_id` text NOT NULL, `workspace_id` text NOT NULL, `profile_id` text NOT NULL, `target_profile_id` text NOT NULL, `copy_key` text NOT NULL, `source_worktree_id` text NOT NULL DEFAULT '', `branch` text NOT NULL DEFAULT '', `head_sha` text NOT NULL DEFAULT '', `target_worktree_id` text NOT NULL DEFAULT '', `restore_worktree_id` text NOT NULL DEFAULT '', `created_at` text NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`session_id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`, `session_id`) REFERENCES `sessions` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (length(trim(copy_key)) > 0));
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
-- apply declarative trigger "worktrees_profile_owner_active" on table "worktrees"
-- +goose StatementBegin
CREATE TRIGGER worktrees_profile_owner_active BEFORE INSERT ON worktrees BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd
-- apply declarative trigger "worktrees_profile_owner_immutable" on table "worktrees"
-- +goose StatementBegin
CREATE TRIGGER worktrees_profile_owner_immutable BEFORE UPDATE OF profile_id ON worktrees
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
