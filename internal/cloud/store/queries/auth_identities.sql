-- name: FindIdentityAccount :one
SELECT account_id FROM identities WHERE provider = $1 AND subject = $2;

-- name: LockAuthAccount :one
SELECT * FROM accounts WHERE id = $1 FOR UPDATE;

-- name: LockAuthAccountByEmail :one
SELECT * FROM accounts WHERE email = $1 FOR UPDATE;

-- name: InsertAuthAccount :one
INSERT INTO accounts (id, email, name, first_sign_in_provider, created_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (email) DO NOTHING RETURNING *;

-- name: UpdateAuthAccountEmail :exec
UPDATE accounts SET email = $2 WHERE id = $1;

-- name: RecordSignInIdentity :one
INSERT INTO identities AS i (provider, subject, account_id, email, created_at, last_sign_in_at)
VALUES ($1, $2, $3, $4, $5, $5)
ON CONFLICT (provider, subject) DO UPDATE
SET email = excluded.email, last_sign_in_at = greatest(i.last_sign_in_at, excluded.last_sign_in_at)
WHERE i.account_id = excluded.account_id
RETURNING account_id;

-- name: AdvanceAuthAccountActivity :exec
UPDATE accounts SET last_seen_at = greatest(last_seen_at, sqlc.arg(as_of)) WHERE id = $1;

-- name: AdvanceAuthSecurityEpoch :one
UPDATE accounts SET security_epoch = security_epoch + 1 WHERE id = $1 RETURNING security_epoch;

-- name: HasDeletedAuthAccount :one
SELECT EXISTS (SELECT 1 FROM deleted_accounts WHERE account_id = $1);
