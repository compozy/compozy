-- name: ListAttentionWorkspaceMutes :many
SELECT workspace_id
FROM attention_workspace_mutes
WHERE profile_id = sqlc.arg(profile_id)
ORDER BY workspace_id ASC;

-- name: IsAttentionWorkspaceMuted :one
SELECT EXISTS(
  SELECT 1
  FROM attention_workspace_mutes
  WHERE profile_id = sqlc.arg(profile_id)
    AND workspace_id = sqlc.arg(workspace_id)
);

-- name: InsertAttentionWorkspaceMute :execrows
INSERT INTO attention_workspace_mutes (profile_id, workspace_id)
VALUES (sqlc.arg(profile_id), sqlc.arg(workspace_id))
ON CONFLICT(profile_id, workspace_id) DO NOTHING;

-- name: DeleteAttentionWorkspaceMutesForProfile :exec
DELETE FROM attention_workspace_mutes
WHERE profile_id = sqlc.arg(profile_id);
