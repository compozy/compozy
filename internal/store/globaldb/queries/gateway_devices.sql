-- name: CreateGatewayDevice :exec
INSERT INTO gateway_device_sessions (
  id, name, token_hash, actor_kind, pairing_origin, revoke_epoch, created_at, last_seen_at, revoked_at
)
VALUES (
  sqlc.arg(id), sqlc.arg(name), sqlc.arg(token_hash), sqlc.arg(actor_kind),
  sqlc.arg(pairing_origin), sqlc.arg(revoke_epoch), sqlc.arg(created_at),
  sqlc.narg(last_seen_at), sqlc.narg(revoked_at)
);

-- name: FindActiveGatewayDeviceByTokenHash :one
UPDATE gateway_device_sessions
SET last_seen_at = sqlc.arg(last_seen_at)
WHERE token_hash = sqlc.arg(token_hash) AND revoked_at IS NULL
RETURNING *;

-- name: ListGatewayDevices :many
SELECT *
FROM gateway_device_sessions
ORDER BY
  CASE WHEN revoked_at IS NULL THEN 0 ELSE 1 END,
  CASE WHEN last_seen_at IS NULL THEN 1 ELSE 0 END,
  last_seen_at DESC,
  created_at DESC,
  id;

-- name: GetGatewayDevice :one
SELECT *
FROM gateway_device_sessions
WHERE id = sqlc.arg(id);

-- name: RenameGatewayDevice :one
UPDATE gateway_device_sessions
SET name = sqlc.arg(name)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: RenameGatewayDeviceForActor :one
UPDATE gateway_device_sessions
SET name = sqlc.arg(name)
WHERE gateway_device_sessions.id = sqlc.arg(target_id)
  AND EXISTS (
    SELECT 1
    FROM gateway_device_sessions AS actor
    WHERE actor.id = sqlc.arg(actor_id)
      AND actor.revoke_epoch = sqlc.arg(actor_epoch)
      AND actor.revoked_at IS NULL
  )
RETURNING *;

-- name: RevokeGatewayDevice :one
UPDATE gateway_device_sessions
SET revoked_at = sqlc.arg(revoked_at), revoke_epoch = revoke_epoch + 1
WHERE id = sqlc.arg(id) AND revoked_at IS NULL
RETURNING *;

-- name: RevokeGatewayDeviceForActor :one
UPDATE gateway_device_sessions
SET revoked_at = sqlc.arg(revoked_at), revoke_epoch = revoke_epoch + 1
WHERE gateway_device_sessions.id = sqlc.arg(target_id)
  AND gateway_device_sessions.revoked_at IS NULL
  AND EXISTS (
    SELECT 1
    FROM gateway_device_sessions AS actor
    WHERE actor.id = sqlc.arg(actor_id)
      AND actor.revoke_epoch = sqlc.arg(actor_epoch)
      AND actor.revoked_at IS NULL
  )
RETURNING *;

-- name: GatewayDeviceEpochMatches :one
SELECT EXISTS (
  SELECT 1
  FROM gateway_device_sessions
  WHERE id = sqlc.arg(id)
    AND revoke_epoch = sqlc.arg(revoke_epoch)
    AND revoked_at IS NULL
);

-- name: CountActiveGatewayDeviceSessions :one
SELECT count(*)
FROM gateway_device_sessions
WHERE revoked_at IS NULL;

-- name: ListActiveGatewayDeviceSessions :many
SELECT *
FROM gateway_device_sessions
WHERE revoked_at IS NULL
ORDER BY CASE WHEN last_seen_at IS NULL THEN 1 ELSE 0 END, last_seen_at DESC, created_at DESC, id;
