-- name: CreateAppToken :exec
INSERT INTO gateway_device_sessions (id, name, token_hash, actor_kind, pairing_origin, created_at)
VALUES (sqlc.arg(id), sqlc.arg(name), sqlc.arg(token_hash), 'app_token', 'local', sqlc.arg(created_at));

-- name: InsertAppTokenOrigin :exec
INSERT INTO gateway_device_origins (device_id, origin, created_at)
VALUES (sqlc.arg(device_id), sqlc.arg(origin), sqlc.arg(created_at));

-- name: DeleteAppTokenOrigins :exec
DELETE FROM gateway_device_origins WHERE device_id = sqlc.arg(device_id);

-- name: GetAppToken :one
SELECT * FROM gateway_device_sessions WHERE id = sqlc.arg(id) AND actor_kind = 'app_token';

-- name: FindAppTokenByTokenHash :one
SELECT * FROM gateway_device_sessions WHERE token_hash = sqlc.arg(token_hash) AND actor_kind = 'app_token';

-- name: ListAppTokens :many
SELECT * FROM gateway_device_sessions
WHERE actor_kind = 'app_token'
  AND (sqlc.arg(state) = 'all'
    OR (sqlc.arg(state) = 'active' AND revoked_at IS NULL)
    OR (sqlc.arg(state) = 'revoked' AND revoked_at IS NOT NULL))
ORDER BY CASE WHEN revoked_at IS NULL THEN 0 ELSE 1 END, created_at DESC, id;

-- name: ListAppTokenOrigins :many
SELECT origin FROM gateway_device_origins WHERE device_id = sqlc.arg(device_id) ORDER BY origin;

-- name: ListActiveAppTokenOrigins :many
SELECT DISTINCT origins.origin FROM gateway_device_origins AS origins
JOIN gateway_device_sessions AS device ON device.id = origins.device_id
WHERE device.actor_kind = 'app_token' AND device.revoked_at IS NULL
ORDER BY origins.origin;

-- name: CountActiveAppTokens :one
SELECT count(*) FROM gateway_device_sessions WHERE actor_kind = 'app_token' AND revoked_at IS NULL;

-- name: RenameAppToken :exec
UPDATE gateway_device_sessions SET name = sqlc.arg(name)
WHERE id = sqlc.arg(id) AND actor_kind = 'app_token' AND revoked_at IS NULL;

-- name: RevokeAppToken :exec
UPDATE gateway_device_sessions SET revoked_at = sqlc.arg(revoked_at), revoke_epoch = revoke_epoch + 1
WHERE id = sqlc.arg(id) AND actor_kind = 'app_token' AND revoked_at IS NULL;

-- name: TouchAppTokenLastSeen :execrows
UPDATE gateway_device_sessions SET last_seen_at = sqlc.arg(last_seen_at)
WHERE id = sqlc.arg(id) AND actor_kind = 'app_token' AND revoked_at IS NULL
  AND revoke_epoch = sqlc.arg(revoke_epoch);
