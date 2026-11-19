-- A pin is organization only: it never touches updated_at or last_update_at,
-- so the session keeps its place in every other ordering.

-- name: PinSession :execrows
UPDATE sessions
SET pinned_at = COALESCE(pinned_at, sqlc.arg(pinned_at))
WHERE workspace_id = sqlc.arg(workspace_id) AND id = sqlc.arg(id);

-- name: UnpinSession :execrows
UPDATE sessions
SET pinned_at = NULL
WHERE workspace_id = sqlc.arg(workspace_id) AND id = sqlc.arg(id) AND pinned_at IS NOT NULL;

-- name: ListSessionPinnedAt :many
SELECT id, CAST(pinned_at AS TEXT) AS pinned_at
FROM sessions
WHERE pinned_at IS NOT NULL
  AND id IN (SELECT CAST(value AS TEXT) FROM json_each(sqlc.arg(session_ids)));
