-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- Existing contracted tasks predate task-level budget persistence. The task
-- runner used the built-in 256 KiB/store default for those rows.
PRAGMA ignore_check_constraints = on;
-- create "new_tasks" table
CREATE TABLE `new_tasks` (`id` text NULL, `profile_id` text NOT NULL, `identifier` text NULL, `scope` text NOT NULL, `workspace_id` text NULL, `parent_task_id` text NULL, `title` text NOT NULL, `description` text NULL, `priority` text NOT NULL DEFAULT 'medium', `max_attempts` integer NOT NULL DEFAULT 3, `status` text NOT NULL, `approval_policy` text NOT NULL DEFAULT 'none', `approval_state` text NOT NULL DEFAULT 'not_required', `owner_kind` text NULL, `owner_ref` text NULL, `created_by_kind` text NOT NULL, `created_by_ref` text NOT NULL, `origin_kind` text NOT NULL, `origin_ref` text NOT NULL, `created_at` text NOT NULL, `updated_at` text NOT NULL, `closed_at` text NULL, `metadata_json` text NULL, `current_run_id` text NULL, `paused` integer NOT NULL DEFAULT 0, `paused_by` text NOT NULL DEFAULT '', `paused_at` text NULL, `paused_reason` text NOT NULL DEFAULT '', `max_runtime_seconds` integer NOT NULL DEFAULT 0, `spawn_failure_count` integer NOT NULL DEFAULT 0, `last_spawn_error` text NOT NULL DEFAULT '', `review_policy` text NOT NULL DEFAULT 'none', `review_max_rounds` integer NOT NULL DEFAULT 3, `review_round` integer NOT NULL DEFAULT 0, `last_review_id` text NULL, `last_review_outcome` text NULL, `review_circuit_opened_at` text NULL, `review_circuit_reason` text NULL, `auto_enqueue_on_ready` integer NOT NULL DEFAULT 0, `needs_attention_reason` text NULL, `needs_attention_at` text NULL, `needs_attention_by_kind` text NULL, `needs_attention_by_ref` text NULL, `wake_creator` integer NOT NULL DEFAULT 1, `expect_digest` text NULL, `result_budget_bytes` integer NULL, `result_overflow` text NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`expect_digest`) REFERENCES `contract_schemas` (`digest`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `1` FOREIGN KEY (`current_run_id`) REFERENCES `task_runs` (`id`) ON UPDATE NO ACTION ON DELETE SET NULL, CONSTRAINT `2` FOREIGN KEY (`parent_task_id`) REFERENCES `tasks` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `3` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `4` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (scope IN ('global', 'workspace')), CHECK (
			priority IN ('low', 'medium', 'high', 'urgent')
		), CHECK (max_attempts > 0 AND max_attempts <= 10), CHECK (
			approval_policy IN ('none', 'manual')
		), CHECK (
			approval_state IN ('not_required', 'pending', 'approved', 'rejected')
		), CHECK (
			owner_kind IS NULL OR owner_kind IN (
				'human', 'agent_session', 'automation', 'extension', 'network_peer', 'pool'
			)
		), CHECK (
			created_by_kind IN (
				'human', 'agent_session', 'automation', 'extension', 'network_peer', 'daemon'
			)
		), CHECK (
			origin_kind IN (
				'cli', 'web', 'uds', 'http', 'automation', 'extension', 'network', 'agent_session', 'daemon'
			)
		), CHECK (paused IN (0, 1)), CHECK (max_runtime_seconds >= 0), CHECK (spawn_failure_count >= 0), CHECK (
			review_policy IN ('none', 'on_success', 'on_failure', 'always')
		), CHECK (review_max_rounds >= 0), CHECK (review_round >= 0), CHECK (
			last_review_outcome IS NULL OR last_review_outcome IN (
				'approved', 'rejected', 'blocked', 'error', 'timeout', 'invalid_output'
			)
		), CHECK (auto_enqueue_on_ready IN (0, 1)), CHECK (result_budget_bytes IS NULL OR result_budget_bytes > 0), CHECK (result_overflow IS NULL OR result_overflow IN ('store', 'reject')), CHECK (
			(scope = 'global' AND workspace_id IS NULL) OR
			(scope = 'workspace' AND workspace_id IS NOT NULL)
		), CHECK (
			(owner_kind IS NULL AND owner_ref IS NULL) OR
			(owner_kind IS NOT NULL AND owner_ref IS NOT NULL)
		), CHECK (parent_task_id IS NULL OR parent_task_id <> id), CHECK (
			(approval_policy = 'none' AND approval_state = 'not_required') OR
			(approval_policy = 'manual' AND approval_state IN ('pending', 'approved', 'rejected'))
		), CHECK (
			(expect_digest IS NULL AND result_budget_bytes IS NULL AND result_overflow IS NULL) OR
			(expect_digest IS NOT NULL AND result_budget_bytes IS NOT NULL AND result_overflow IS NOT NULL)
		));
-- copy rows from old table "tasks" to new temporary table "new_tasks"
INSERT INTO `new_tasks` (`id`, `profile_id`, `identifier`, `scope`, `workspace_id`, `parent_task_id`, `title`, `description`, `priority`, `max_attempts`, `status`, `approval_policy`, `approval_state`, `owner_kind`, `owner_ref`, `created_by_kind`, `created_by_ref`, `origin_kind`, `origin_ref`, `created_at`, `updated_at`, `closed_at`, `metadata_json`, `current_run_id`, `paused`, `paused_by`, `paused_at`, `paused_reason`, `max_runtime_seconds`, `spawn_failure_count`, `last_spawn_error`, `review_policy`, `review_max_rounds`, `review_round`, `last_review_id`, `last_review_outcome`, `review_circuit_opened_at`, `review_circuit_reason`, `auto_enqueue_on_ready`, `needs_attention_reason`, `needs_attention_at`, `needs_attention_by_kind`, `needs_attention_by_ref`, `wake_creator`, `expect_digest`) SELECT `id`, `profile_id`, `identifier`, `scope`, `workspace_id`, `parent_task_id`, `title`, `description`, `priority`, `max_attempts`, `status`, `approval_policy`, `approval_state`, `owner_kind`, `owner_ref`, `created_by_kind`, `created_by_ref`, `origin_kind`, `origin_ref`, `created_at`, `updated_at`, `closed_at`, `metadata_json`, `current_run_id`, `paused`, `paused_by`, `paused_at`, `paused_reason`, `max_runtime_seconds`, `spawn_failure_count`, `last_spawn_error`, `review_policy`, `review_max_rounds`, `review_round`, `last_review_id`, `last_review_outcome`, `review_circuit_opened_at`, `review_circuit_reason`, `auto_enqueue_on_ready`, `needs_attention_reason`, `needs_attention_at`, `needs_attention_by_kind`, `needs_attention_by_ref`, `wake_creator`, `expect_digest` FROM `tasks`;
UPDATE `new_tasks`
SET `result_budget_bytes` = 262144,
    `result_overflow` = 'store'
WHERE `expect_digest` IS NOT NULL;
PRAGMA ignore_check_constraints = off;
-- drop trigger "tasks_profile_owner_active" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `tasks_profile_owner_active`;
-- drop trigger "tasks_profile_owner_immutable" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `tasks_profile_owner_immutable`;
-- drop trigger "trg_tasks_terminal_command_delete_guard" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `trg_tasks_terminal_command_delete_guard`;
-- drop "tasks" table after copying rows
DROP TABLE `tasks`;
-- rename temporary table "new_tasks" to "tasks"
ALTER TABLE `new_tasks` RENAME TO `tasks`;
-- create index "idx_tasks_approval_state" to table: "tasks"
CREATE INDEX `idx_tasks_approval_state` ON `tasks` (`approval_state`);
-- create index "idx_tasks_created_by" to table: "tasks"
CREATE INDEX `idx_tasks_created_by` ON `tasks` (`created_by_kind`, `created_by_ref`);
-- create index "idx_tasks_current_run" to table: "tasks"
CREATE INDEX `idx_tasks_current_run` ON `tasks` (`current_run_id`);
-- create index "idx_tasks_owner" to table: "tasks"
CREATE INDEX `idx_tasks_owner` ON `tasks` (`owner_kind`, `owner_ref`);
-- create index "idx_tasks_parent" to table: "tasks"
CREATE INDEX `idx_tasks_parent` ON `tasks` (`parent_task_id`);
-- create index "idx_tasks_paused" to table: "tasks"
CREATE INDEX `idx_tasks_paused` ON `tasks` (`paused`, `updated_at` DESC);
-- create index "idx_tasks_priority" to table: "tasks"
CREATE INDEX `idx_tasks_priority` ON `tasks` (`priority`);
-- create index "idx_tasks_review_policy" to table: "tasks"
CREATE INDEX `idx_tasks_review_policy` ON `tasks` (`review_policy`);
-- create index "idx_tasks_review_round" to table: "tasks"
CREATE INDEX `idx_tasks_review_round` ON `tasks` (`review_round`);
-- create index "idx_tasks_scope" to table: "tasks"
CREATE INDEX `idx_tasks_scope` ON `tasks` (`scope`);
-- create index "idx_tasks_status" to table: "tasks"
CREATE INDEX `idx_tasks_status` ON `tasks` (`status`);
-- create index "idx_tasks_workspace" to table: "tasks"
CREATE INDEX `idx_tasks_workspace` ON `tasks` (`workspace_id`);
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
-- recreate trigger "tasks_profile_owner_active" after rebuilding table "tasks"
-- +goose StatementBegin
CREATE TRIGGER tasks_profile_owner_active BEFORE INSERT ON tasks BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd
-- recreate trigger "tasks_profile_owner_immutable" after rebuilding table "tasks"
-- +goose StatementBegin
CREATE TRIGGER tasks_profile_owner_immutable BEFORE UPDATE OF profile_id ON tasks
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
-- recreate trigger "trg_tasks_terminal_command_delete_guard" after rebuilding table "tasks"
-- +goose StatementBegin
CREATE TRIGGER trg_tasks_terminal_command_delete_guard
	BEFORE DELETE ON tasks
	WHEN EXISTS (
		SELECT 1
		FROM task_run_terminal_commands
		WHERE task_id = OLD.id
	)
	BEGIN
		SELECT RAISE(ABORT, 'task run terminal command in progress');
	END;
-- +goose StatementEnd
