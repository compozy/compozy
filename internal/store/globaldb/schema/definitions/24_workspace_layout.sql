-- Workspace layout is the operator-owned All-projects organization of one
-- profile's sidebar: named root folders that group workspaces, plus a manual
-- workspace order. Folders are one level deep and hold workspaces only.
-- Deleting a folder returns its workspaces to the root; it never touches a
-- workspace or its sessions.
CREATE TABLE workspace_folders (
	id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
	profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
	name TEXT NOT NULL CHECK (length(trim(name)) > 0 AND length(name) <= 120),
	position INTEGER NOT NULL CHECK (position >= 0),
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE (profile_id, id)
);

CREATE INDEX idx_workspace_folders_profile
	ON workspace_folders(profile_id, position);

-- A placement pins one workspace (or the User scope, workspace_id '') into a
-- folder (folder_id set) or the root (folder_id NULL) at a manual position.
-- Workspaces without a placement are unplaced and follow the placed root
-- workspaces in name order.
CREATE TABLE workspace_placements (
	profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
	workspace_id TEXT NOT NULL,
	folder_id TEXT,
	position INTEGER NOT NULL CHECK (position >= 0),
	updated_at TEXT NOT NULL,
	PRIMARY KEY (profile_id, workspace_id),
	FOREIGN KEY (profile_id, folder_id)
		REFERENCES workspace_folders(profile_id, id) ON DELETE CASCADE
);

CREATE INDEX idx_workspace_placements_container
	ON workspace_placements(profile_id, folder_id, position);
