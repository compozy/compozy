-- SPDX-License-Identifier: FSL-1.1-MIT

-- name: InsertAccountToken :exec
INSERT INTO account_tokens (id, account_id, name, token_sha256, created_at)
VALUES ($1, $2, $3, $4, $5);

-- name: ListAccountTokens :many
SELECT id, name, last_used_at FROM account_tokens
WHERE account_id = $1 AND revoked_at IS NULL
ORDER BY created_at, id;

-- name: ReadAccountToken :one
SELECT * FROM account_tokens WHERE token_sha256 = $1;

-- name: TouchAccountToken :exec
UPDATE account_tokens SET last_used_at = GREATEST(last_used_at, sqlc.arg(as_of)) WHERE id = $1;

-- name: TouchAccountTokenInstance :exec
INSERT INTO account_token_instances (token_id, installation_id, last_seen_at)
VALUES ($1, $2, $3)
ON CONFLICT (token_id, installation_id) DO UPDATE
SET last_seen_at = GREATEST(account_token_instances.last_seen_at, excluded.last_seen_at);

-- name: RevokeAccountToken :execrows
UPDATE account_tokens SET revoked_at = $3 WHERE id = $1 AND account_id = $2 AND revoked_at IS NULL;
