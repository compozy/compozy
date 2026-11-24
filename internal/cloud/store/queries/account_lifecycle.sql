-- name: UpdateLifecycleState :exec
UPDATE accounts
SET state = sqlc.arg(state), suspended_at = sqlc.narg(suspended_at)
WHERE id = sqlc.arg(account_id);

-- name: RevokeLifecycleDeviceSessions :exec
UPDATE device_sessions SET revoked_at = sqlc.arg(as_of)
WHERE account_id = sqlc.arg(account_id) AND revoked_at IS NULL;

-- name: RevokeLifecycleWebSessions :exec
UPDATE cloud_web_sessions SET revoked_at = sqlc.arg(as_of)
WHERE account_id = sqlc.arg(account_id) AND revoked_at IS NULL;

-- name: RevokeLifecycleAccountTokens :exec
SELECT revoke_lifecycle_tokens(sqlc.arg(account_id)::text, sqlc.arg(as_of)::timestamptz);

-- name: ExpireLifecycleCheckouts :exec
UPDATE billing_checkouts SET expired_at = sqlc.arg(as_of)
WHERE account_id = sqlc.arg(account_id) AND expired_at IS NULL;

-- name: StartAccountDeletion :exec
INSERT INTO account_deletion_jobs (account_id, requested_at, retry_at)
VALUES (sqlc.arg(account_id), sqlc.arg(as_of), sqlc.arg(as_of));

-- name: DueAccountDeletions :many
SELECT account_id FROM account_deletion_jobs
WHERE retry_at <= sqlc.arg(as_of) ORDER BY retry_at, account_id LIMIT 32;

-- name: ClaimAccountDeletion :one
UPDATE account_deletion_jobs
SET retry_at = sqlc.arg(lease_until)
WHERE account_id = sqlc.arg(account_id) AND retry_at <= sqlc.arg(as_of)
RETURNING step;

-- name: LockClaimedAccountDeletion :one
SELECT step FROM account_deletion_jobs
WHERE account_id = sqlc.arg(account_id) AND retry_at = sqlc.arg(lease_until) FOR UPDATE;

-- name: AdvanceAccountDeletion :exec
UPDATE account_deletion_jobs SET step = step + 1, retry_at = sqlc.arg(as_of)
WHERE account_id = sqlc.arg(account_id);

-- name: RetryAccountDeletion :exec
UPDATE account_deletion_jobs SET retry_at = sqlc.arg(retry_at)
WHERE account_id = sqlc.arg(account_id) AND retry_at = sqlc.arg(lease_until);

-- name: AccountDeletionRequestedAt :one
SELECT requested_at FROM account_deletion_jobs WHERE account_id = $1;

-- name: RemoveDeletingAccountIdentities :exec
WITH removed_identities AS (
    DELETE FROM identities i WHERE i.account_id = $1 RETURNING i.account_id, i.provider, i.subject
), held_identities AS (
    INSERT INTO account_deletion_identity_holds (subject_sha256, account_id)
    SELECT sha256(convert_to(r.provider || ':' || r.subject, 'UTF8')), r.account_id
    FROM removed_identities r
)
DELETE FROM cloud_web_sessions ws WHERE ws.account_id = $1;

-- name: DeletingAccountTunnelRefs :many
SELECT t.computer_id, t.generation
FROM tunnels t JOIN computers c ON c.id = t.computer_id
WHERE c.account_id = $1 AND t.state = 'deleting'
ORDER BY t.computer_id;

-- name: InsertDeletedAccount :exec
INSERT INTO deleted_accounts (account_id, deleted_at) VALUES ($1, $2);

-- name: CompactDeletedAccountEvents :exec
INSERT INTO security_event_retention (deleted_account_id) VALUES (sqlc.arg(account_id)::text);

-- name: RemoveCloudAccount :exec
DELETE FROM accounts WHERE id = $1 AND state = 'deleting';

-- name: SweepDeletedAccounts :exec
DELETE FROM deleted_accounts WHERE deleted_at <= $1;

-- name: UpdateMarketingConsent :exec
UPDATE accounts
SET marketing_opt_in = sqlc.arg(opt_in), marketing_consent_at = sqlc.arg(as_of),
    marketing_consent_source = sqlc.arg(source), contact_synced_at = NULL, contact_retry_at = NULL
WHERE id = sqlc.arg(account_id);

-- name: TrackMarketingAddress :exec
INSERT INTO account_contact_addresses (account_id, email) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: MarketingAddresses :many
SELECT email FROM account_contact_addresses WHERE account_id = $1 ORDER BY email;

-- name: ForgetMarketingAddress :exec
DELETE FROM account_contact_addresses WHERE account_id = $1 AND email = $2;

-- name: DirtyMarketingAccounts :many
SELECT id FROM accounts
WHERE state <> 'deleting' AND marketing_consent_at IS NOT NULL AND contact_synced_at IS NULL
  AND (contact_retry_at IS NULL OR contact_retry_at <= sqlc.arg(as_of)::timestamptz)
ORDER BY marketing_consent_at, id LIMIT 32;

-- name: ClaimMarketingAccount :one
SELECT * FROM accounts
WHERE id = sqlc.arg(account_id) AND state <> 'deleting' AND marketing_consent_at IS NOT NULL AND contact_synced_at IS NULL
  AND (contact_retry_at IS NULL OR contact_retry_at <= sqlc.arg(as_of)::timestamptz)
FOR UPDATE SKIP LOCKED;

-- name: RecordMarketingSync :exec
UPDATE accounts SET contact_synced_at = sqlc.arg(as_of), contact_retry_at = NULL WHERE id = sqlc.arg(account_id);

-- name: RetryMarketingAccount :exec
UPDATE accounts SET contact_retry_at = sqlc.arg(retry_at)
WHERE id = sqlc.arg(account_id) AND contact_synced_at IS NULL;

-- name: QueueAccountEmail :exec
INSERT INTO account_email_outbox (event_id, retry_at) VALUES ($1, $2);

-- name: DueAccountEmails :many
SELECT o.event_id, e.account_id FROM account_email_outbox o
JOIN security_events e ON e.id = o.event_id
WHERE o.retry_at <= sqlc.arg(as_of) ORDER BY o.retry_at, o.event_id LIMIT 32;

-- name: LockAccountEmail :one
SELECT o.event_id, e.account_id, e.type, e.details_json, e.at
FROM account_email_outbox o JOIN security_events e ON e.id = o.event_id
WHERE o.event_id = sqlc.arg(event_id) AND o.retry_at <= sqlc.arg(as_of)
FOR UPDATE OF o SKIP LOCKED;

-- name: RetryAccountEmail :exec
UPDATE account_email_outbox SET retry_at = sqlc.arg(retry_at)
WHERE event_id = sqlc.arg(event_id) AND retry_at <= sqlc.arg(as_of);

-- name: RemoveAccountEmail :exec
DELETE FROM account_email_outbox WHERE event_id = $1;

-- name: RecentSignInEvents :many
SELECT DISTINCT ON (details_json->>'device_id') id, details_json, at
FROM security_events
WHERE account_id = $1 AND type = 'device_enrolled' AND at > sqlc.arg(since)::timestamptz
ORDER BY details_json->>'device_id', at DESC, id DESC;

-- name: AccountSignInEvent :one
SELECT details_json, at FROM security_events
WHERE id = $1 AND account_id = $2 AND type = 'device_enrolled';

-- name: HasActivatedBrowserKey :one
SELECT EXISTS (SELECT 1 FROM device_keys WHERE device_id = $1 AND activated_at IS NOT NULL);

-- name: MarkLifecycleTunnelsDeleting :exec
UPDATE tunnels t SET state = 'deleting'
FROM computers c
WHERE t.computer_id = c.id AND c.account_id = $1 AND t.state NOT IN ('deleted', 'deleting');
