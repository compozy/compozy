-- name: ListWorkspaceFolders :many
SELECT f.id, f.profile_id, f.name, f.position, f.created_at, f.updated_at,
	(SELECT COUNT(1) FROM workspace_placements AS p
		WHERE p.profile_id = f.profile_id AND p.folder_id = f.id) AS workspace_count
FROM workspace_folders AS f
WHERE f.profile_id = sqlc.arg(profile_id)
ORDER BY f.position ASC, f.created_at ASC, f.id ASC;

-- name: CountWorkspaceFolders :one
SELECT COUNT(1) FROM workspace_folders
WHERE profile_id = sqlc.arg(profile_id);

-- name: ShiftWorkspaceFolders :exec
UPDATE workspace_folders SET position = position + 1
WHERE profile_id = sqlc.arg(profile_id);

-- name: InsertWorkspaceFolder :exec
INSERT INTO workspace_folders (id, profile_id, name, position, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(profile_id), sqlc.arg(name), 0, sqlc.arg(now), sqlc.arg(now));

-- name: RenameWorkspaceFolder :execrows
UPDATE workspace_folders SET name = sqlc.arg(name), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND profile_id = sqlc.arg(profile_id);

-- name: DeleteWorkspaceFolder :execrows
DELETE FROM workspace_folders
WHERE id = sqlc.arg(id) AND profile_id = sqlc.arg(profile_id);

-- name: SetWorkspaceFolderPosition :exec
UPDATE workspace_folders SET position = sqlc.arg(position), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND profile_id = sqlc.arg(profile_id);

-- name: ListWorkspacePlacements :many
SELECT p.workspace_id, p.folder_id, p.position
FROM workspace_placements AS p
LEFT JOIN workspace_folders AS f ON f.profile_id = p.profile_id AND f.id = p.folder_id
WHERE p.profile_id = sqlc.arg(profile_id)
ORDER BY CASE WHEN p.folder_id IS NULL THEN 1 ELSE 0 END ASC,
	f.position ASC, p.position ASC, p.workspace_id ASC;

-- name: ListWorkspaceContainerMembers :many
SELECT workspace_id FROM workspace_placements
WHERE profile_id = sqlc.arg(profile_id) AND folder_id IS sqlc.narg(folder_id)
ORDER BY position ASC, workspace_id ASC;

-- name: UpsertWorkspacePlacement :exec
INSERT INTO workspace_placements (profile_id, workspace_id, folder_id, position, updated_at)
VALUES (sqlc.arg(profile_id), sqlc.arg(workspace_id), sqlc.narg(folder_id), sqlc.arg(position), sqlc.arg(now))
ON CONFLICT (profile_id, workspace_id) DO UPDATE SET
	folder_id = excluded.folder_id,
	position = excluded.position,
	updated_at = excluded.updated_at;

-- name: NextWorkspaceContainerPosition :one
SELECT CAST(COALESCE(MAX(position), -1) + 1 AS INTEGER) AS next_position FROM workspace_placements
WHERE profile_id = sqlc.arg(profile_id) AND folder_id IS sqlc.narg(folder_id);

-- name: WorkspaceLayoutTargetExists :one
SELECT EXISTS (SELECT 1 FROM workspaces WHERE id = sqlc.arg(id)) AS found;
