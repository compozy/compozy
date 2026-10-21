-- Session layout is the operator-owned sidebar organization of one profile's
-- sessions inside one workspace: named folders plus manual session order.
-- Deleting a folder returns its sessions to the workspace root; it never
-- deletes or archives sessions.
CREATE TABLE session_folders (
	id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
	profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
	workspace_id TEXT NOT NULL,
	name TEXT NOT NULL CHECK (length(trim(name)) > 0 AND length(name) <= 120),
	position INTEGER NOT NULL CHECK (position >= 0),
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE (profile_id, workspace_id, id)
);

CREATE INDEX idx_session_folders_scope
	ON session_folders(profile_id, workspace_id, position);

-- A placement pins one root session into a folder (folder_id set) or into
-- the workspace root (folder_id NULL) at a dense manual position. Sessions
-- without a placement are unplaced and sort first, newest first.
CREATE TABLE session_placements (
	session_id TEXT PRIMARY KEY,
	profile_id TEXT NOT NULL,
	workspace_id TEXT NOT NULL,
	folder_id TEXT,
	position INTEGER NOT NULL CHECK (position >= 0),
	updated_at TEXT NOT NULL,
	FOREIGN KEY (workspace_id, session_id)
		REFERENCES sessions(workspace_id, id) ON DELETE CASCADE,
	FOREIGN KEY (profile_id, workspace_id, folder_id)
		REFERENCES session_folders(profile_id, workspace_id, id) ON DELETE CASCADE
);

CREATE INDEX idx_session_placements_container
	ON session_placements(profile_id, workspace_id, folder_id, position);

CREATE INDEX idx_session_placements_folder
	ON session_placements(folder_id);
