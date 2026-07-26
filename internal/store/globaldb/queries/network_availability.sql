-- name: GetNetworkAvailability :one
SELECT enabled, epoch, updated_at, updated_by
FROM network_availability
WHERE id = 1;

-- name: UpdateNetworkAvailability :one
UPDATE network_availability
SET enabled = sqlc.arg(enabled),
    epoch = epoch + CASE WHEN enabled <> sqlc.arg(enabled) THEN 1 ELSE 0 END,
    updated_at = sqlc.arg(updated_at),
    updated_by = sqlc.arg(updated_by)
WHERE id = 1
RETURNING enabled, epoch, updated_at, updated_by;
