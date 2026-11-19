-- A pin is organization only: it never touches updated_at or last_update_at,
-- so the session keeps its place in every other ordering.

-- A new pin takes the top of the Pinned order; pinning a pinned session keeps
-- its pin time and position.
-- name: PinSession :execrows
UPDATE sessions
SET pin_position = CASE
        WHEN pinned_at IS NULL OR pin_position IS NULL THEN (
            SELECT COALESCE(MIN(pinned.pin_position), 0) - 1
            FROM sessions AS pinned
            WHERE pinned.pinned_at IS NOT NULL AND pinned.pin_position IS NOT NULL
        )
        ELSE pin_position
    END,
    pinned_at = COALESCE(pinned_at, sqlc.arg(pinned_at))
WHERE sessions.workspace_id = sqlc.arg(workspace_id) AND sessions.id = sqlc.arg(id);

-- name: UnpinSession :execrows
UPDATE sessions
SET pinned_at = NULL, pin_position = NULL
WHERE workspace_id = sqlc.arg(workspace_id) AND id = sqlc.arg(id) AND pinned_at IS NOT NULL;

-- name: ListSessionPinnedAt :many
SELECT id, CAST(pinned_at AS TEXT) AS pinned_at
FROM sessions
WHERE pinned_at IS NOT NULL
  AND id IN (SELECT CAST(value AS TEXT) FROM json_each(sqlc.arg(session_ids)));

-- name: ListLayoutPinPositions :many
SELECT id, CAST(COALESCE(pin_position, 0) AS INTEGER) AS pin_position
FROM sessions
WHERE pinned_at IS NOT NULL
  AND workspace_id = sqlc.arg(workspace_id)
  AND profile_id = sqlc.arg(profile_id)
  AND id IN (SELECT CAST(value AS TEXT) FROM json_each(sqlc.arg(session_ids)));

-- name: SetSessionPinPosition :execrows
UPDATE sessions
SET pin_position = sqlc.arg(pin_position)
WHERE workspace_id = sqlc.arg(workspace_id) AND id = sqlc.arg(id) AND pinned_at IS NOT NULL;
