-- name: PutApprovalGrant :one
INSERT INTO tool_approval_grants (
  id, workspace_id, agent_name, tool_id, input_digest, decision, created_at, last_used_at
) VALUES (
  sqlc.arg(id), sqlc.arg(workspace_id), sqlc.arg(agent_name), sqlc.arg(tool_id),
  sqlc.arg(input_digest), sqlc.arg(decision), sqlc.arg(created_at), sqlc.arg(last_used_at)
)
ON CONFLICT(workspace_id, agent_name, tool_id, input_digest) DO UPDATE SET
  decision = excluded.decision,
  created_at = excluded.created_at,
  last_used_at = excluded.last_used_at
RETURNING id, workspace_id, agent_name, tool_id, input_digest, decision, created_at, last_used_at;

-- name: LookupApprovalGrant :one
UPDATE tool_approval_grants
SET last_used_at = sqlc.arg(last_used_at)
WHERE id = (
  SELECT candidate.id
  FROM tool_approval_grants AS candidate
  WHERE candidate.workspace_id = sqlc.arg(workspace_id)
    AND candidate.tool_id = sqlc.arg(tool_id)
    AND (candidate.agent_name = '' OR candidate.agent_name = sqlc.arg(agent_name))
    AND (candidate.input_digest = '' OR candidate.input_digest = sqlc.arg(input_digest))
  ORDER BY CASE
    WHEN candidate.agent_name <> '' AND candidate.input_digest <> '' THEN 4
    WHEN candidate.agent_name <> '' THEN 3
    WHEN candidate.input_digest <> '' THEN 2
    ELSE 1
  END DESC
  LIMIT 1
)
RETURNING id, workspace_id, agent_name, tool_id, input_digest, decision, created_at, last_used_at;

-- name: ListApprovalGrants :many
SELECT id, workspace_id, agent_name, tool_id, input_digest, decision, created_at, last_used_at
FROM tool_approval_grants
WHERE workspace_id = sqlc.arg(workspace_id)
ORDER BY created_at DESC, id;

-- name: RevokeApprovalGrant :execrows
DELETE FROM tool_approval_grants
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id);
