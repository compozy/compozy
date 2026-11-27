-- SPDX-License-Identifier: FSL-1.1-MIT

-- name: ControlAccount :one
SELECT id, state, security_epoch FROM accounts WHERE id = $1;

-- name: ControlDevice :one
SELECT id, name, kind, admin, last_seen_at
FROM devices
WHERE account_id = $1 AND id = $2 AND revoked_at IS NULL;

-- name: ControlPrincipalActive :one
SELECT EXISTS (
    SELECT 1 FROM device_sessions AS session
    JOIN device_keys AS key ON key.device_id = session.device_id AND key.jkt = session.jkt
    WHERE session.account_id = $1 AND session.device_id = $2 AND session.jkt = $3
      AND session.id = $4 AND session.revoked_at IS NULL AND key.state = 'active'
) AS active;

-- name: ListControlDevices :many
SELECT id, name, kind, admin, last_seen_at
FROM devices AS device
WHERE account_id = $1 AND revoked_at IS NULL AND EXISTS (
    SELECT 1 FROM device_keys WHERE device_id = device.id AND state = 'active'
)
ORDER BY lower(name), id;

-- name: UpdateControlDevice :exec
UPDATE devices SET name = $3, admin = $4 WHERE account_id = $1 AND id = $2;

-- name: ControlDeviceScopes :many
SELECT computer_id, scopes FROM device_scopes
WHERE device_id = $1 AND (computer_id IS NULL OR computer_id = sqlc.arg(target_id)::text);

-- name: SetControlDeviceScopes :execrows
INSERT INTO device_scopes (device_id, computer_id, scopes)
VALUES ($1, $2, $3)
ON CONFLICT (device_id, computer_id) DO UPDATE SET scopes = excluded.scopes
WHERE device_scopes.scopes IS DISTINCT FROM excluded.scopes;

-- name: RevokeControlDevice :exec
UPDATE devices SET revoked_at = $3 WHERE account_id = $1 AND id = $2 AND revoked_at IS NULL;

-- name: RevokeControlDeviceKeys :exec
UPDATE device_keys SET state = 'revoked', revoked_at = $2
WHERE device_id = $1 AND state <> 'revoked';

-- name: RevokeControlDeviceSessions :exec
UPDATE device_sessions SET revoked_at = $3 WHERE account_id = $1 AND device_id = $2 AND revoked_at IS NULL;

-- name: RevokeControlWebSessions :exec
UPDATE cloud_web_sessions SET revoked_at = $3
WHERE account_id = $1 AND browser_device_id = $2 AND revoked_at IS NULL;

-- name: ReleaseControlDeviceReservations :exec
UPDATE slot_reservations SET released_at = $3
WHERE account_id = $1 AND device_id = $2 AND consumed_at IS NULL AND released_at IS NULL;

-- name: ControlSnapshotDevices :many
SELECT device.id, device.admin, array_agg(key.jkt ORDER BY key.jkt)::text[] AS jkts,
    (global_scope.device_id IS NOT NULL)::boolean AS has_global_scopes, global_scope.scopes AS global_scopes,
    (local_scope.device_id IS NOT NULL)::boolean AS has_local_scopes, local_scope.scopes AS local_scopes
FROM devices AS device
JOIN device_keys AS key ON key.device_id = device.id AND key.state = 'active'
LEFT JOIN device_scopes AS global_scope ON global_scope.device_id = device.id AND global_scope.computer_id IS NULL
LEFT JOIN device_scopes AS local_scope
    ON local_scope.device_id = device.id AND local_scope.computer_id = sqlc.arg(target_id)::text
WHERE device.account_id = sqlc.arg(account_id) AND device.revoked_at IS NULL
GROUP BY device.id, global_scope.device_id, global_scope.scopes, local_scope.device_id, local_scope.scopes
ORDER BY device.id;
