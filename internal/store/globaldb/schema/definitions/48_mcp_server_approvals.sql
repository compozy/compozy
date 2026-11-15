CREATE TABLE mcp_server_approvals (
 id TEXT PRIMARY KEY,
 profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
 workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
 server_name TEXT NOT NULL,
 source_id TEXT NOT NULL,
 definition_digest TEXT NOT NULL,
 approved_at TEXT NOT NULL,
 UNIQUE (profile_id, workspace_id, server_name, source_id)
);
