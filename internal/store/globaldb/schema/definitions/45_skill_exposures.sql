CREATE TABLE skill_exposures (
	id            INTEGER PRIMARY KEY,
	skill_name    TEXT NOT NULL CHECK (trim(skill_name) <> ''),
	canonical_dir TEXT NOT NULL CHECK (trim(canonical_dir) <> ''),
	target_slug   TEXT NOT NULL CHECK (trim(target_slug) <> ''),
	link_path     TEXT NOT NULL UNIQUE CHECK (trim(link_path) <> ''),
	link_target   TEXT NOT NULL CHECK (trim(link_target) <> ''),
	owner_scope   TEXT NOT NULL CHECK (owner_scope IN ('user', 'workspace')),
	workspace_id  TEXT,
	created_at    TIMESTAMP NOT NULL,
	updated_at    TIMESTAMP NOT NULL,
	CHECK (
		(owner_scope = 'user' AND workspace_id IS NULL)
		OR
		(owner_scope = 'workspace' AND trim(COALESCE(workspace_id, '')) <> '')
	)
);

CREATE UNIQUE INDEX idx_skill_exposures_owner_target
	ON skill_exposures(skill_name, owner_scope, COALESCE(workspace_id, ''), target_slug);

CREATE INDEX idx_skill_exposures_skill_name
	ON skill_exposures(skill_name);

CREATE INDEX idx_skill_exposures_workspace_id
	ON skill_exposures(workspace_id);
