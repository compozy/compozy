-- SPDX-License-Identifier: FSL-1.1-MIT
-- name: BeginCloudRestore :exec
INSERT INTO cloud_restores (restore_point, archive_through) VALUES ($1, $2)
ON CONFLICT (restore_point) DO NOTHING;

-- name: GetCloudRestore :one
SELECT * FROM cloud_restores WHERE restore_point = $1;

-- name: LockCloudRestore :one
SELECT * FROM cloud_restores WHERE restore_point = $1 FOR UPDATE;

-- name: HasPendingCloudRestore :one
SELECT EXISTS (SELECT 1 FROM cloud_restores WHERE completed_at IS NULL);

-- name: MarkRestoreApplied :exec
UPDATE cloud_restores SET applied_at = $2 WHERE restore_point = $1;

-- name: FinishCloudRestore :execrows
UPDATE cloud_restores SET completed_at = $2 WHERE restore_point = $1 AND applied_at IS NOT NULL
  AND completed_at IS NULL;

-- name: BumpRestoreEpochs :exec
UPDATE accounts SET security_epoch = security_epoch + 1000;

-- name: RevokeRestoreDeviceSessions :exec
UPDATE device_sessions SET revoked_at = $1 WHERE revoked_at IS NULL;

-- name: RevokeRestoreWebSessions :exec
UPDATE cloud_web_sessions SET revoked_at = $1 WHERE revoked_at IS NULL;

-- name: RetireRestoredGrantCredentials :exec
UPDATE device_grants SET state = 'cancelled', pkce_verifier_enc = NULL
WHERE state IN ('pending', 'authorized');

-- name: RetireRestoredEnrollmentCredentials :exec
UPDATE enrollment_tokens SET used_at = $1 WHERE used_at IS NULL;

-- name: RetireRestoredBrowserCredentials :exec
UPDATE browser_codes SET used_at = $1 WHERE used_at IS NULL;

-- name: ArchiveSecurityEvents :many
SELECT id, account_id, type, actor, at, details_json
FROM security_events WHERE id > sqlc.arg(after_id)::bigint
UNION ALL
SELECT security_archive_id, account_id, 'account_deleted', 'system', deleted_at, '{}'::jsonb
FROM deleted_accounts WHERE security_archive_id > sqlc.arg(after_id)::bigint
ORDER BY id;

-- name: HasArchivedSecurityEvent :one
SELECT EXISTS (
    SELECT 1 FROM security_events WHERE id = sqlc.arg(event_id) AND at = sqlc.arg(event_at) AND type = sqlc.arg(event_type)
    UNION ALL
    SELECT 1 FROM deleted_accounts WHERE security_archive_id = sqlc.arg(event_id)
      AND deleted_at = sqlc.arg(event_at) AND sqlc.arg(event_type)::text = 'account_deleted'
);

-- name: AdvanceSecurityArchiveSequence :exec
SELECT setval(pg_get_serial_sequence('security_events', 'id'),
              greatest(sqlc.arg(archive_id)::bigint, 1, (SELECT max(id) FROM security_events),
              (SELECT max(security_archive_id) FROM deleted_accounts)), true);

-- name: MarkRestoredAccountState :exec
UPDATE accounts SET state = sqlc.arg(state), suspended_at = sqlc.narg(suspended_at)
WHERE id = sqlc.arg(account_id) AND state <> 'deleting';

-- name: RevokeRestoredComputerAccess :exec
UPDATE computers SET remote_enabled = false, plan_limited = false WHERE id = $1 AND account_id = $2;

-- name: DisableRestoredAdmin :exec
UPDATE devices SET admin = false WHERE id = $1 AND account_id = $2;

-- name: RestrictRestoredScopes :exec
INSERT INTO device_scopes (device_id, computer_id, scopes)
SELECT d.id, c.id, '{}'::text[] FROM devices d JOIN computers c ON c.account_id = d.account_id
WHERE d.id = sqlc.arg(device_id) AND d.account_id = sqlc.arg(account_id)
ON CONFLICT (device_id, computer_id) DO UPDATE SET scopes = excluded.scopes;

-- name: RestoreKnownTunnels :many
SELECT hostname, generation, cf_tunnel_id FROM tunnels WHERE state <> 'deleted' ORDER BY hostname;

-- name: MarkRestoredTunnelDeleting :exec
UPDATE tunnels t SET state = 'deleting'
FROM computers c
WHERE t.computer_id = c.id AND c.account_id = sqlc.arg(account_id)
  AND t.computer_id = sqlc.arg(computer_id) AND t.generation = sqlc.arg(generation) AND t.state <> 'deleted';

-- name: RevokeRestoredBrowserKey :exec
UPDATE device_keys k SET state = 'revoked', revoked_at = sqlc.arg(revoked_at)
FROM devices d WHERE k.device_id = d.id AND d.account_id = sqlc.arg(account_id) AND k.jkt = sqlc.arg(jkt)
  AND k.state <> 'revoked';

-- name: ArchiveClock :one
SELECT clock_timestamp()::timestamptz;

-- name: DropRestoredTokenInstances :exec
SELECT cloud_drop_restored_token_instances(sqlc.arg(restore_point)::timestamptz);

-- name: RevokeRestoredAccountToken :exec
SELECT cloud_revoke_restored_account_tokens(sqlc.arg(account_id)::text,
    sqlc.arg(token_id)::text, sqlc.arg(revoked_at)::timestamptz);

-- name: RevokeRestoredAccountTokens :exec
SELECT cloud_revoke_restored_account_tokens(sqlc.arg(account_id)::text, NULL,
    sqlc.arg(revoked_at)::timestamptz);
