-- name: ReadSessionBranches :many
SELECT s.id, CAST(CASE WHEN COALESCE(s.worktree_id, '') <> '' THEN
 CASE WHEN w.state IN ('pending','ready') THEN COALESCE(w.branch,'') ELSE '' END
 ELSE s.branch END AS TEXT) AS branch
FROM sessions s JOIN json_each(sqlc.arg(ids)) i ON s.id = i.value
LEFT JOIN worktrees w ON w.workspace_id = s.workspace_id AND w.id = s.worktree_id;

-- name: SaveSessionBranch :execrows
UPDATE sessions SET branch = sqlc.arg(branch)
WHERE id = sqlc.arg(id) AND workspace_id = sqlc.arg(workspace_id)
 AND COALESCE(worktree_id,'') = '';

-- name: SessionCatalogProfiles :many
SELECT DISTINCT profile_id FROM sessions WHERE workspace_id = sqlc.arg(workspace_id);
