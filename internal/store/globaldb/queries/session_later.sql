-- name: ParkSession :execrows
UPDATE sessions
SET parked_at = sqlc.arg(parked_at),
  park_reason = sqlc.arg(park_reason),
  park_until = sqlc.narg(park_until),
  later_returned_at = NULL,
  updated_at = sqlc.arg(parked_at)
WHERE sessions.workspace_id = sqlc.arg(workspace_id) AND sessions.id = sqlc.arg(id);

-- name: UnparkSession :execrows
UPDATE sessions
SET parked_at = NULL, park_reason = '', park_until = NULL, updated_at = sqlc.arg(updated_at)
WHERE workspace_id = sqlc.arg(workspace_id) AND id = sqlc.arg(id) AND parked_at IS NOT NULL;

-- name: ListDueLaterSessions :many
SELECT workspace_id, id, profile_id, COALESCE(name, '') AS name, park_reason
FROM sessions
WHERE parked_at IS NOT NULL AND park_until IS NOT NULL AND park_until <= sqlc.arg(now)
ORDER BY park_until, id
LIMIT sqlc.arg(row_limit);

-- name: ReturnLaterSession :execrows
UPDATE sessions
SET parked_at = NULL,
  park_reason = '',
  park_until = NULL,
  later_returned_at = sqlc.arg(now),
  attention_revision = attention_revision + 1,
  attention_changed_at = sqlc.arg(now),
  updated_at = sqlc.arg(now)
WHERE workspace_id = sqlc.arg(workspace_id) AND id = sqlc.arg(id)
  AND parked_at IS NOT NULL AND park_until IS NOT NULL AND park_until <= sqlc.arg(now);

-- name: ClearLaterReturn :execrows
UPDATE sessions
SET later_returned_at = NULL,
  attention_revision = attention_revision + 1,
  attention_changed_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND later_returned_at IS NOT NULL;

-- name: GetSessionLaterState :one
SELECT parked_at, park_reason, park_until, later_returned_at,
  CAST((CASE WHEN parked_at IS NULL THEN 0 ELSE (
    SELECT COUNT(1) FROM session_input_queue AS later_input
    WHERE later_input.session_id = sessions.id AND later_input.status = 'queued'
      AND later_input.origin_class = 'agent') END) AS INTEGER) AS pending_messages
FROM sessions
WHERE sessions.workspace_id = sqlc.arg(workspace_id) AND sessions.id = sqlc.arg(id);

-- name: UnparkSessionParkedAt :execrows
UPDATE sessions
SET parked_at = NULL, park_reason = '', park_until = NULL, updated_at = sqlc.arg(updated_at)
WHERE workspace_id = sqlc.arg(workspace_id) AND id = sqlc.arg(id) AND parked_at = sqlc.arg(parked_at);
