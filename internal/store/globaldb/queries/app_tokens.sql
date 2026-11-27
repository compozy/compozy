-- name: CreateAppToken :exec
INSERT INTO app_tokens (id, name, token_hash, created_at)
VALUES (sqlc.arg(id), sqlc.arg(name), sqlc.arg(token_hash), sqlc.arg(created_at));

-- name: InsertAppTokenOrigin :exec
INSERT INTO app_token_origins (app_token_id, origin, created_at)
VALUES (sqlc.arg(app_token_id), sqlc.arg(origin), sqlc.arg(created_at));

-- name: DeleteAppTokenOrigins :exec
DELETE FROM app_token_origins WHERE app_token_id = sqlc.arg(app_token_id);

-- name: GetAppToken :one
SELECT * FROM app_tokens WHERE id = sqlc.arg(id);

-- name: FindAppTokenByTokenHash :one
SELECT * FROM app_tokens WHERE token_hash = sqlc.arg(token_hash);

-- name: ListAppTokens :many
SELECT * FROM app_tokens
WHERE (sqlc.arg(state) = 'all'
    OR (sqlc.arg(state) = 'active' AND revoked_at IS NULL)
    OR (sqlc.arg(state) = 'revoked' AND revoked_at IS NOT NULL))
ORDER BY CASE WHEN revoked_at IS NULL THEN 0 ELSE 1 END, created_at DESC, id;

-- name: ListAppTokenOrigins :many
SELECT origin FROM app_token_origins WHERE app_token_id = sqlc.arg(app_token_id) ORDER BY origin;

-- name: ListActiveAppTokenOrigins :many
SELECT DISTINCT origins.origin FROM app_token_origins AS origins
JOIN app_tokens AS device ON device.id = origins.app_token_id
WHERE device.revoked_at IS NULL
ORDER BY origins.origin;

-- name: CountActiveAppTokens :one
SELECT count(*) FROM app_tokens WHERE revoked_at IS NULL;

-- name: RenameAppToken :exec
UPDATE app_tokens SET name = sqlc.arg(name)
WHERE id = sqlc.arg(id) AND revoked_at IS NULL;

-- name: RevokeAppToken :exec
UPDATE app_tokens SET revoked_at = sqlc.arg(revoked_at), revoke_epoch = revoke_epoch + 1
WHERE id = sqlc.arg(id) AND revoked_at IS NULL;

-- name: TouchAppTokenLastSeen :execrows
UPDATE app_tokens SET last_seen_at = sqlc.arg(last_seen_at)
WHERE id = sqlc.arg(id) AND revoked_at IS NULL
  AND revoke_epoch = sqlc.arg(revoke_epoch);

-- name: AppTokenEpochMatches :one
SELECT CAST(EXISTS (
	SELECT 1 FROM app_tokens
	WHERE id = sqlc.arg(id) AND revoke_epoch = sqlc.arg(revoke_epoch) AND revoked_at IS NULL
) AS BOOLEAN);
