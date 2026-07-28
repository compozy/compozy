CREATE TABLE tool_processes (
			id               TEXT PRIMARY KEY,
			source           TEXT NOT NULL,
			session_id       TEXT NOT NULL DEFAULT '',
			turn_id          TEXT NOT NULL DEFAULT '',
			tool_call_id     TEXT NOT NULL DEFAULT '',
			terminal_id      TEXT NOT NULL DEFAULT '',
			extension_name   TEXT NOT NULL DEFAULT '',
			hook_name        TEXT NOT NULL DEFAULT '',
			sandbox_id   TEXT NOT NULL DEFAULT '',
			pid              INTEGER NOT NULL DEFAULT 0,
			process_group_id INTEGER NOT NULL DEFAULT 0,
			command          TEXT NOT NULL DEFAULT '',
			args_json        TEXT NOT NULL DEFAULT '[]',
			cwd              TEXT NOT NULL DEFAULT '',
			started_at       TEXT,
			started_by_pid   INTEGER NOT NULL DEFAULT 0,
			state            TEXT NOT NULL,
			exit_code        INTEGER,
			error            TEXT NOT NULL DEFAULT '',
			created_at       TEXT NOT NULL,
			updated_at       TEXT NOT NULL,
			completed_at     TEXT
		);

CREATE INDEX idx_tool_processes_extension
			ON tool_processes(extension_name);

CREATE INDEX idx_tool_processes_hook
			ON tool_processes(hook_name);

CREATE INDEX idx_tool_processes_session_turn
			ON tool_processes(session_id, turn_id);

CREATE INDEX idx_tool_processes_state_updated
			ON tool_processes(state, updated_at);

CREATE INDEX idx_tool_processes_terminal
			ON tool_processes(terminal_id);

CREATE INDEX idx_tool_processes_tool_call
			ON tool_processes(tool_call_id);
