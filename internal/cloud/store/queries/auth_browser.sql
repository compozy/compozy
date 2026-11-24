-- name: InsertCloudWebSession :exec
INSERT INTO cloud_web_sessions (id_sha256, account_id, created_at, last_used_at, expires_at)
VALUES ($1, $2, $3, $3, $4);

-- name: TouchCloudWebSession :one
UPDATE cloud_web_sessions w SET last_used_at = greatest(w.last_used_at, sqlc.arg(as_of))
FROM accounts a
WHERE w.id_sha256 = $1 AND w.account_id = a.id AND a.state = 'active' AND w.revoked_at IS NULL
  AND w.expires_at > sqlc.arg(as_of) AND w.last_used_at > sqlc.arg(idle_limit)::timestamptz
  AND (w.browser_device_id IS NULL OR EXISTS (
      SELECT 1 FROM devices d WHERE d.id = w.browser_device_id AND d.revoked_at IS NULL
  ))
RETURNING w.account_id, a.email, w.browser_device_id;

-- name: BindCloudWebSession :execrows
UPDATE cloud_web_sessions SET browser_device_id = $2
WHERE id_sha256 = $1 AND account_id = $3 AND revoked_at IS NULL
  AND (browser_device_id IS NULL OR browser_device_id = $2);

-- name: FindBrowserComputer :one
SELECT c.id, c.account_id FROM computers c JOIN tunnels t ON t.computer_id = c.id
WHERE t.hostname = $1 AND t.state IN ('active', 'deleting')
  AND c.unlinked_at IS NULL AND c.remote_enabled;

-- name: FindBoundBrowserDevice :one
SELECT * FROM devices WHERE browser_binding_sha256 = $1 AND account_id = $2 AND kind = 'browser';

-- name: ExtendPendingBrowserKey :execrows
UPDATE device_keys SET pending_expires_at = greatest(pending_expires_at, $2)
WHERE jkt = $1 AND state = 'pending';

-- name: ExpirePendingBrowserKey :exec
DELETE FROM device_keys
WHERE device_id = $1 AND origin = $2 AND state = 'pending' AND pending_expires_at <= sqlc.arg(as_of);

-- name: InsertCloudBrowserCode :exec
INSERT INTO browser_codes (code_sha256, account_id, device_id, computer_id, jkt, code_challenge, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: CloudBrowserCode :one
SELECT * FROM browser_codes WHERE code_sha256 = $1;

-- name: UseCloudBrowserCode :execrows
UPDATE browser_codes SET used_at = sqlc.arg(as_of)
WHERE code_sha256 = $1 AND used_at IS NULL AND expires_at > sqlc.arg(as_of);

-- name: RetireBrowserOriginKey :exec
UPDATE device_keys SET state = 'revoked', revoked_at = sqlc.arg(as_of)
WHERE device_id = $1 AND origin = $2 AND state = 'active';

-- name: ActivateBrowserKey :execrows
UPDATE device_keys SET state = 'active', activated_at = sqlc.arg(as_of), pending_expires_at = NULL
WHERE jkt = $1 AND device_id = $2 AND state = 'pending' AND pending_expires_at > sqlc.arg(as_of);

-- name: SweepBrowserCodes :exec
DELETE FROM browser_codes WHERE expires_at <= $1;

-- name: SweepPendingBrowserKeys :exec
DELETE FROM device_keys WHERE state = 'pending' AND pending_expires_at <= $1;

-- name: SweepUnactivatedBrowserDevices :exec
DELETE FROM devices d
WHERE kind = 'browser' AND last_seen_at <= $1 AND revoked_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM device_keys k WHERE k.device_id = d.id);
