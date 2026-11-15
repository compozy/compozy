-- name: ListMCPServerApprovals :many
SELECT * FROM mcp_server_approvals WHERE profile_id = ? AND workspace_id = ? ORDER BY server_name, source_id;

-- name: PutMCPServerApproval :exec
INSERT INTO mcp_server_approvals (id, profile_id, workspace_id, server_name, source_id, definition_digest, approved_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (profile_id, workspace_id, server_name, source_id) DO UPDATE SET
 definition_digest = excluded.definition_digest, approved_at = excluded.approved_at;

-- name: DeleteMCPServerApproval :exec
DELETE FROM mcp_server_approvals WHERE profile_id = ? AND workspace_id = ? AND server_name = ? AND source_id = ?;
