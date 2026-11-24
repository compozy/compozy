-- name: FindIdentityAccount :one
SELECT i.account_id FROM identities i WHERE i.provider = sqlc.arg(provider) AND i.subject = sqlc.arg(subject)
UNION ALL
SELECT h.account_id FROM account_deletion_identity_holds h
WHERE h.subject_sha256 = sha256(convert_to(sqlc.arg(provider)::text || ':' || sqlc.arg(subject)::text, 'UTF8'));

-- name: LockAuthAccount :one
SELECT * FROM accounts WHERE id = $1 FOR UPDATE;

-- name: LockAuthAccountByEmail :one
SELECT * FROM accounts WHERE email = $1 FOR UPDATE;

-- name: InsertAuthAccount :one
INSERT INTO accounts (id, email, name, first_sign_in_provider, created_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (email) DO NOTHING RETURNING *;

-- name: UpdateAuthAccountEmail :exec
UPDATE accounts
SET email = $2, contact_synced_at = CASE WHEN marketing_opt_in THEN NULL ELSE contact_synced_at END
WHERE id = $1;

-- name: RecordSignInIdentity :one
INSERT INTO identities AS i (provider, subject, account_id, email, created_at, last_sign_in_at)
VALUES ($1, $2, $3, $4, $5, $5)
ON CONFLICT (provider, subject) DO UPDATE
SET email = excluded.email, last_sign_in_at = greatest(i.last_sign_in_at, excluded.last_sign_in_at)
WHERE i.account_id = excluded.account_id
RETURNING account_id;

-- name: AdvanceAuthAccountActivity :exec
UPDATE accounts SET last_seen_at = greatest(last_seen_at, sqlc.arg(as_of)) WHERE id = $1;

-- name: HasDeletedAuthAccount :one
SELECT EXISTS (SELECT 1 FROM deleted_accounts WHERE account_id = $1);
