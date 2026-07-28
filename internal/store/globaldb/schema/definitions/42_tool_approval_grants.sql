CREATE TABLE tool_approval_grants (
	id           TEXT NOT NULL PRIMARY KEY CHECK (trim(id) <> ''),
	workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	agent_name   TEXT NOT NULL DEFAULT '',
	tool_id      TEXT NOT NULL CHECK (trim(tool_id) <> ''),
	input_digest TEXT NOT NULL DEFAULT '' CHECK (input_digest = '' OR input_digest LIKE 'sha256:%'),
	decision     TEXT NOT NULL CHECK (decision IN ('allow', 'reject')),
	created_at   TEXT NOT NULL,
	last_used_at TEXT NOT NULL,
	UNIQUE (workspace_id, agent_name, tool_id, input_digest)
);

CREATE INDEX idx_tool_approval_grants_lookup
	ON tool_approval_grants (workspace_id, tool_id, agent_name, input_digest);

CREATE INDEX idx_tool_approval_grants_list
	ON tool_approval_grants (workspace_id, created_at DESC, id);
