CREATE TABLE dead_entities (
	workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	kind         TEXT NOT NULL CHECK (kind IN ('extension', 'bridge', 'mcp_sidecar')),
	entity_id    TEXT NOT NULL CHECK (trim(entity_id) <> ''),
	reason       TEXT NOT NULL CHECK (trim(reason) <> ''),
	marked_at    TEXT NOT NULL,
	PRIMARY KEY (workspace_id, kind, entity_id)
);
