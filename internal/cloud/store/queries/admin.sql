-- SPDX-License-Identifier: FSL-1.1-MIT

-- name: SearchAdminAccounts :many
SELECT a.id, a.email, a.name, a.state
FROM accounts a
WHERE a.id = sqlc.arg(term)::text
   OR strpos(a.email, lower(sqlc.arg(term)::text)) > 0
   OR EXISTS (SELECT 1 FROM subscriptions s WHERE s.account_id = a.id AND s.stripe_customer_id = sqlc.arg(term)::text)
ORDER BY a.email, a.id
LIMIT 50;

-- name: AdminTunnel :one
SELECT a.id AS account_id, a.email, t.computer_id, t.hostname, t.generation, t.state
FROM tunnels t JOIN computers c ON c.id = t.computer_id JOIN accounts a ON a.id = c.account_id
WHERE t.hostname = $1 AND t.state <> 'deleted';

-- name: TakeDownAdminTunnel :exec
UPDATE tunnels SET admin_takedown_at = sqlc.arg(at)
WHERE computer_id = sqlc.arg(computer_id) AND generation = sqlc.arg(generation);

-- name: SetAdminHostedRouteCap :execrows
UPDATE accounts SET cloud_routes_max = sqlc.arg(cap)
WHERE id = sqlc.arg(account_id) AND cloud_routes_max IS DISTINCT FROM sqlc.arg(cap);

-- name: RevokeAdminDeviceSession :execrows
UPDATE device_sessions SET revoked_at = sqlc.arg(at)
WHERE account_id = sqlc.arg(account_id) AND id = sqlc.arg(session_id) AND revoked_at IS NULL;

-- name: DetachAdminIdentity :execrows
DELETE FROM identities
WHERE account_id = sqlc.arg(account_id) AND provider = sqlc.arg(provider) AND subject = sqlc.arg(subject);

-- name: InsertComplimentaryGrant :exec
INSERT INTO subscriptions (id, account_id, source, status, ends_at, granted_by, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(account_id), 'complimentary', 'active',
        sqlc.arg(ends_at), sqlc.arg(actor), sqlc.arg(as_of));

-- name: AdminOptedInContacts :many
SELECT id, email, name, marketing_consent_at, marketing_consent_source
FROM accounts
WHERE marketing_opt_in AND marketing_consent_at IS NOT NULL AND state <> 'deleting'
ORDER BY email, id;

-- name: AdminSecurityEvent :one
SELECT id, account_id, type, actor, at, details_json FROM security_events WHERE id = $1;

-- name: GetAdminAccount :one
SELECT id, email, name, state, cloud_routes_max FROM accounts WHERE id = $1;
