-- +goose Up
-- create "task_run_terminal_commands" table
CREATE TABLE `task_run_terminal_commands` (`command_id` text NOT NULL, `run_id` text NOT NULL, `task_id` text NOT NULL, `workspace_id` text NOT NULL, `kind` text NOT NULL, `phase` text NOT NULL, `source_status` text NOT NULL, `source_session_id` text NOT NULL, `source_claim_token_hash` text NOT NULL, `source_lease_until` text NULL, `intent_json` text NOT NULL, `actor_json` text NOT NULL, `command_at` text NOT NULL, `admitted_at` text NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`run_id`), CONSTRAINT `0` FOREIGN KEY (`task_id`) REFERENCES `tasks` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`run_id`) REFERENCES `task_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (
			kind IN ('completed', 'failed', 'canceled', 'needs_attention')
		), CHECK (
			phase IN ('admitted', 'stop_requested', 'stop_confirmed')
		), CHECK (json_valid(intent_json)), CHECK (json_valid(actor_json)));
-- create index "task_run_terminal_commands_command_id" to table: "task_run_terminal_commands"
CREATE UNIQUE INDEX `task_run_terminal_commands_command_id` ON `task_run_terminal_commands` (`command_id`);
-- create index "idx_task_run_terminal_commands_phase" to table: "task_run_terminal_commands"
CREATE INDEX `idx_task_run_terminal_commands_phase` ON `task_run_terminal_commands` (`phase`, `admitted_at`, `run_id`);
-- reject lifecycle writes until the matching durable command is consumed
-- +goose StatementBegin
CREATE TRIGGER trg_task_runs_terminal_command_guard
	BEFORE UPDATE ON task_runs
	WHEN EXISTS (
		SELECT 1
		FROM task_run_terminal_commands
		WHERE run_id = OLD.id
	)
	BEGIN
		SELECT RAISE(ABORT, 'task run terminal command in progress');
	END;
-- +goose StatementEnd
-- reject deletion until the matching durable command is consumed
-- +goose StatementBegin
CREATE TRIGGER trg_task_runs_terminal_command_delete_guard
	BEFORE DELETE ON task_runs
	WHEN EXISTS (
		SELECT 1
		FROM task_run_terminal_commands
		WHERE run_id = OLD.id
	)
	BEGIN
		SELECT RAISE(ABORT, 'task run terminal command in progress');
	END;
-- +goose StatementEnd
-- reject parent-task deletion before cascades can erase the durable command
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
