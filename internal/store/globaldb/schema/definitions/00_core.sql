CREATE TABLE app_metadata (
			key        TEXT PRIMARY KEY,
			value      TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);

CREATE TABLE workspaces (
		id            TEXT PRIMARY KEY,
		root_dir      TEXT NOT NULL UNIQUE,
		add_dirs      TEXT NOT NULL DEFAULT '[]',
		name          TEXT NOT NULL UNIQUE,
		default_agent TEXT DEFAULT '',
		sandbox_ref TEXT NOT NULL DEFAULT '',
		created_at    TEXT NOT NULL,
		updated_at    TEXT NOT NULL
	);

CREATE INDEX idx_workspaces_name ON workspaces(name);
