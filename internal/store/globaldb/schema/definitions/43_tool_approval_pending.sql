CREATE TABLE tool_approval_pending (
	approval_id TEXT NOT NULL PRIMARY KEY CHECK (approval_id LIKE 'apr_%'),
	workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	invocation_id TEXT NOT NULL UNIQUE CHECK (trim(invocation_id) <> ''),
	target_kind TEXT NOT NULL CHECK (target_kind IN ('tool', 'client_op', 'navigate', 'view')),
	tool_id TEXT,
	target_json TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(target_json)),
	command_id TEXT,
	args_json TEXT NOT NULL CHECK (json_valid(args_json)),
	approval_status TEXT NOT NULL CHECK (
		approval_status IN ('pending', 'approved', 'denied', 'timeout', 'canceled')
	),
	execution_status TEXT CHECK (
		execution_status IS NULL OR execution_status IN ('dispatching', 'completed', 'failed', 'uncertain')
	),
	result_json TEXT CHECK (result_json IS NULL OR json_valid(result_json)),
	error_json TEXT CHECK (error_json IS NULL OR json_valid(error_json)),
	requested_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL CHECK (expires_at > requested_at),
	resolved_at INTEGER,
	executed_at INTEGER,
	resume_fence INTEGER NOT NULL DEFAULT 0 CHECK (resume_fence IN (0, 1)),
	CHECK ((target_kind = 'tool' AND trim(coalesce(tool_id, '')) <> '') OR target_kind <> 'tool'),
	CHECK ((approval_status = 'pending' AND resolved_at IS NULL) OR (approval_status <> 'pending' AND resolved_at IS NOT NULL)),
	CHECK ((execution_status IS NULL AND executed_at IS NULL) OR execution_status IS NOT NULL)
);

CREATE INDEX idx_tool_approval_pending_workspace_status
	ON tool_approval_pending (workspace_id, approval_status, expires_at, approval_id);

CREATE INDEX idx_tool_approval_pending_recovery
	ON tool_approval_pending (approval_status, execution_status, resume_fence, expires_at);
