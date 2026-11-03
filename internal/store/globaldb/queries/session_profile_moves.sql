-- name: InsertSessionProfileMove :exec
INSERT INTO session_profile_moves (
  session_id, workspace_id, profile_id, target_profile_id, copy_key,
  source_worktree_id, branch, head_sha, target_worktree_id, restore_worktree_id,
  created_at, updated_at
) VALUES (
  sqlc.arg(session_id), sqlc.arg(workspace_id), sqlc.arg(profile_id), sqlc.arg(target_profile_id),
  sqlc.arg(copy_key), sqlc.arg(source_worktree_id), sqlc.arg(branch), sqlc.arg(head_sha),
  sqlc.arg(target_worktree_id), sqlc.arg(restore_worktree_id), sqlc.arg(created_at), sqlc.arg(updated_at)
);

-- name: GetSessionProfileMove :one
SELECT * FROM session_profile_moves WHERE session_id = sqlc.arg(session_id);

-- name: ListSessionProfileMoves :many
SELECT * FROM session_profile_moves ORDER BY created_at, session_id;

-- name: SetSessionProfileMoveRestoreWorktree :execrows
UPDATE session_profile_moves SET
  restore_worktree_id = sqlc.arg(restore_worktree_id),
  updated_at = sqlc.arg(updated_at)
WHERE session_id = sqlc.arg(session_id);

-- name: DeleteSessionProfileMove :exec
DELETE FROM session_profile_moves WHERE session_id = sqlc.arg(session_id);
