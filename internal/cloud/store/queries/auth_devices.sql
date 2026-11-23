-- name: FindAuthDeviceKey :one
SELECT k.*, d.account_id, d.admin, d.kind, d.revoked_at AS device_revoked_at
FROM device_keys k JOIN devices d ON d.id = k.device_id WHERE k.jkt = $1;

-- name: AuthDevicePopulation :one
SELECT
    count(*) FILTER (WHERE kind <> 'browser' OR EXISTS (
        SELECT 1 FROM device_keys k WHERE k.device_id = d.id AND k.activated_at IS NOT NULL
    )) AS enrolled,
    count(*) FILTER (WHERE admin AND revoked_at IS NULL AND EXISTS (
        SELECT 1 FROM device_keys k WHERE k.device_id = d.id AND k.state = 'active'
    )) AS admins
FROM devices d WHERE account_id = $1;

-- name: InsertAuthDevice :exec
INSERT INTO devices (id, account_id, name, kind, installation_id, browser_binding_sha256, admin, last_seen_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: InsertAuthDeviceKey :execrows
INSERT INTO device_keys (jkt, device_id, public_jwk, origin, state, pending_expires_at, created_at, activated_at)
VALUES ($1, $2, sqlc.arg(public_jwk)::text::jsonb, $3, $4, $5, $6, $7)
ON CONFLICT DO NOTHING;

-- name: PromoteRecoveredAdmin :exec
UPDATE devices SET admin = true WHERE id = $1 AND revoked_at IS NULL;

-- name: AdvanceAuthDeviceActivity :exec
UPDATE devices SET last_seen_at = greatest(last_seen_at, sqlc.arg(as_of)) WHERE id = $1;

-- name: RevokeAuthDeviceSession :exec
UPDATE device_sessions SET revoked_at = sqlc.arg(as_of) WHERE device_id = $1 AND revoked_at IS NULL;

-- name: InsertAuthDeviceSession :exec
INSERT INTO device_sessions (id, account_id, device_id, jkt, refresh_token_sha256, provider, created_at, last_used_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $7);

-- name: ReadAuthDeviceSession :one
SELECT s.*, a.state AS account_state, d.admin, d.revoked_at AS device_revoked_at, k.state AS key_state
FROM device_sessions s
JOIN accounts a ON a.id = s.account_id
JOIN devices d ON d.id = s.device_id AND d.account_id = s.account_id
JOIN device_keys k ON k.device_id = s.device_id AND k.jkt = s.jkt
WHERE s.id = $1 AND s.account_id = $2;

-- name: RotateAuthDeviceSession :execrows
UPDATE device_sessions SET refresh_token_sha256 = sqlc.arg(next_sha256),
    last_used_at = greatest(last_used_at, sqlc.arg(as_of)), rotated_at = sqlc.arg(as_of)
WHERE id = $1 AND refresh_token_sha256 = sqlc.arg(previous_sha256) AND revoked_at IS NULL;

-- name: RememberCloudProof :execrows
INSERT INTO cloud_dpop_replays AS p (jkt, jti, expires_at) VALUES ($1, $2, $3)
ON CONFLICT (jkt, jti) DO UPDATE SET expires_at = excluded.expires_at
WHERE p.expires_at <= sqlc.arg(as_of);

-- name: SweepCloudProofs :exec
DELETE FROM cloud_dpop_replays WHERE expires_at <= $1;
