CREATE TABLE dead_entities (
	profile_id   TEXT NOT NULL REFERENCES profiles(id),
	workspace_id TEXT NOT NULL,
	kind         TEXT NOT NULL CHECK (kind IN ('extension', 'bridge', 'mcp_sidecar', 'loop_target')),
	entity_id    TEXT NOT NULL CHECK (trim(entity_id) <> ''),
	reason       TEXT NOT NULL CHECK (trim(reason) <> ''),
	marked_at    TEXT NOT NULL,
	PRIMARY KEY (profile_id, workspace_id, kind, entity_id)
);
