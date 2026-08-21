CREATE TABLE cmd_palette_usage (
	workspace_id TEXT NOT NULL,
	command_id TEXT NOT NULL CHECK (trim(command_id) <> ''),
	use_count INTEGER NOT NULL DEFAULT 0 CHECK (use_count >= 0),
	frecency_weight REAL NOT NULL DEFAULT 0 CHECK (frecency_weight >= 0),
	last_used_at INTEGER NOT NULL CHECK (last_used_at >= 0),
	updated_at INTEGER NOT NULL CHECK (updated_at >= 0),
	PRIMARY KEY (workspace_id, command_id)
);

CREATE INDEX idx_cmd_palette_usage_recents
	ON cmd_palette_usage (workspace_id, last_used_at DESC, command_id);

CREATE TABLE cmd_palette_query_hits (
	workspace_id TEXT NOT NULL,
	query TEXT NOT NULL CHECK (trim(query) <> ''),
	command_id TEXT NOT NULL CHECK (trim(command_id) <> ''),
	weight REAL NOT NULL DEFAULT 0 CHECK (weight >= 0),
	last_used_at INTEGER NOT NULL CHECK (last_used_at >= 0),
	PRIMARY KEY (workspace_id, query, command_id)
);

CREATE INDEX idx_cmd_palette_query_hits_lookup
	ON cmd_palette_query_hits (workspace_id, query, last_used_at DESC, command_id);

CREATE TABLE cmd_palette_pins (
	workspace_id TEXT NOT NULL,
	command_id TEXT NOT NULL CHECK (trim(command_id) <> ''),
	pinned_at INTEGER NOT NULL CHECK (pinned_at >= 0),
	PRIMARY KEY (workspace_id, command_id)
);

CREATE INDEX idx_cmd_palette_pins_order
	ON cmd_palette_pins (workspace_id, pinned_at ASC, command_id);

CREATE TRIGGER cmd_palette_workspace_delete
AFTER DELETE ON workspaces
BEGIN
	DELETE FROM cmd_palette_usage WHERE workspace_id = OLD.id;
	DELETE FROM cmd_palette_query_hits WHERE workspace_id = OLD.id;
	DELETE FROM cmd_palette_pins WHERE workspace_id = OLD.id;
END;
