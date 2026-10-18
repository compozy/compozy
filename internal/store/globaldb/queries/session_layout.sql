-- name: ListSessionFolders :many
SELECT f.id, f.profile_id, f.workspace_id, f.name, f.position, f.created_at, f.updated_at,
	(SELECT COUNT(1) FROM session_placements AS p
		JOIN sessions AS s ON s.id = p.session_id
		WHERE p.folder_id = f.id AND s.archived_at IS NULL) AS session_count
FROM session_folders AS f
WHERE f.profile_id = sqlc.arg(profile_id) AND f.workspace_id = sqlc.arg(workspace_id)
ORDER BY f.position ASC, f.created_at ASC, f.id ASC;

-- name: CountSessionFolders :one
SELECT COUNT(1) FROM session_folders
WHERE profile_id = sqlc.arg(profile_id) AND workspace_id = sqlc.arg(workspace_id);

-- name: ShiftSessionFolders :exec
UPDATE session_folders SET position = position + 1
WHERE profile_id = sqlc.arg(profile_id) AND workspace_id = sqlc.arg(workspace_id);

-- name: InsertSessionFolder :exec
INSERT INTO session_folders (id, profile_id, workspace_id, name, position, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(profile_id), sqlc.arg(workspace_id), sqlc.arg(name), 0, sqlc.arg(now), sqlc.arg(now));

-- name: RenameSessionFolder :execrows
UPDATE session_folders SET name = sqlc.arg(name), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND profile_id = sqlc.arg(profile_id) AND workspace_id = sqlc.arg(workspace_id);

-- name: DeleteSessionFolder :execrows
DELETE FROM session_folders
WHERE id = sqlc.arg(id) AND profile_id = sqlc.arg(profile_id) AND workspace_id = sqlc.arg(workspace_id);

-- name: SetSessionFolderPosition :exec
UPDATE session_folders SET position = sqlc.arg(position), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND profile_id = sqlc.arg(profile_id) AND workspace_id = sqlc.arg(workspace_id);

-- name: GetSessionLayoutOwner :one
SELECT profile_id, spawn_role FROM sessions
WHERE workspace_id = sqlc.arg(workspace_id) AND id = sqlc.arg(id);

-- name: ListSessionContainerMembers :many
SELECT session_id FROM session_placements
WHERE profile_id = sqlc.arg(profile_id) AND workspace_id = sqlc.arg(workspace_id)
	AND folder_id IS sqlc.narg(folder_id)
ORDER BY position ASC, session_id DESC;

-- name: UpsertSessionPlacement :exec
INSERT INTO session_placements (session_id, profile_id, workspace_id, folder_id, position, updated_at)
VALUES (sqlc.arg(session_id), sqlc.arg(profile_id), sqlc.arg(workspace_id), sqlc.narg(folder_id), sqlc.arg(position), sqlc.arg(now))
ON CONFLICT (session_id) DO UPDATE SET
	folder_id = excluded.folder_id,
	position = excluded.position,
	updated_at = excluded.updated_at;

-- name: DeleteSessionPlacement :exec
DELETE FROM session_placements
WHERE session_id = sqlc.arg(session_id) AND workspace_id = sqlc.arg(workspace_id);

-- name: ListSessionPlacementsByIDs :many
SELECT p.session_id, p.folder_id, p.position, f.position AS folder_position
FROM session_placements AS p
LEFT JOIN session_folders AS f ON f.id = p.folder_id
WHERE p.session_id IN (SELECT CAST(value AS TEXT) FROM json_each(sqlc.arg(ids)));

-- name: NextSessionContainerPosition :one
SELECT CAST(COALESCE(MAX(position), -1) + 1 AS INTEGER) AS next_position FROM session_placements
WHERE profile_id = sqlc.arg(profile_id) AND workspace_id = sqlc.arg(workspace_id)
	AND folder_id IS sqlc.narg(folder_id);
