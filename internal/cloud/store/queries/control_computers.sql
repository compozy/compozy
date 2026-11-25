-- SPDX-License-Identifier: FSL-1.1-MIT

-- name: ControlComputer :one
SELECT * FROM computers WHERE id = $1;

-- name: ControlComputers :many
SELECT * FROM computers WHERE account_id = $1 AND unlinked_at IS NULL ORDER BY lower(name), id;

-- name: ControlEndpoints :many
SELECT endpoint.computer_id, endpoint.address
FROM computer_endpoints AS endpoint
JOIN computers AS computer ON computer.id = endpoint.computer_id
WHERE computer.account_id = $1 AND computer.unlinked_at IS NULL
ORDER BY endpoint.computer_id, endpoint.address;

-- name: ClearControlEndpoints :exec
DELETE FROM computer_endpoints WHERE computer_id = $1;

-- name: SetControlEndpoints :exec
INSERT INTO computer_endpoints (computer_id, kind, address, reported_at)
SELECT sqlc.arg(computer_id)::text, 'direct', address, sqlc.arg(reported_at)::timestamptz
FROM unnest(sqlc.arg(addresses)::text[]) AS endpoint(address);

-- name: CreateControlComputer :one
INSERT INTO computers (id, account_id, name, platform, version, computer_jwk, tls_root_sha256, remote_enabled, linked_at, installation_id)
VALUES (sqlc.arg(id), sqlc.arg(account_id), sqlc.arg(name), sqlc.arg(platform), sqlc.arg(version),
        sqlc.arg(computer_jwk)::text::jsonb, sqlc.arg(tls_root_sha256), true, sqlc.arg(linked_at), sqlc.narg(installation_id))
ON CONFLICT (id) DO NOTHING
RETURNING *;

-- name: UpdateControlComputer :one
UPDATE computers SET name = $3, remote_enabled = $4,
    plan_limited = CASE WHEN $4 THEN plan_limited ELSE false END
WHERE id = $1 AND account_id = $2 AND unlinked_at IS NULL
RETURNING *;

-- name: UnlinkControlComputer :exec
UPDATE computers SET unlinked_at = $3, remote_enabled = false, plan_limited = false
WHERE id = $1 AND account_id = $2 AND unlinked_at IS NULL;

-- name: OpenComputerControlLink :exec
UPDATE computers SET control_link_id = sqlc.arg(control_link_id),
    last_control_at = greatest(last_control_at, sqlc.arg(last_control_at)::timestamptz)
WHERE id = sqlc.arg(id) AND account_id = sqlc.arg(account_id) AND unlinked_at IS NULL;

-- name: CloseComputerControlLink :execrows
UPDATE computers SET control_link_id = NULL
WHERE id = $1 AND control_link_id = $2;

-- name: ApplyControlProPlan :execrows
UPDATE computers SET plan_limited = false
WHERE account_id = $1 AND unlinked_at IS NULL AND plan_limited;

-- name: ApplyControlFreePlan :execrows
WITH ranked AS (
    SELECT id, row_number() OVER (
        ORDER BY plan_limited, last_session_at DESC NULLS LAST, linked_at, id
    ) AS slot
    FROM computers AS candidate
    WHERE candidate.account_id = $1 AND candidate.unlinked_at IS NULL AND candidate.remote_enabled
)
UPDATE computers SET plan_limited = ranked.slot > 1
FROM ranked
WHERE computers.id = ranked.id AND computers.plan_limited IS DISTINCT FROM (ranked.slot > 1);

-- name: AdvanceControlLease :one
UPDATE computers
SET lease_sequence = greatest(lease_sequence + 1, sqlc.arg(sequence_floor)::bigint),
    last_control_at = greatest(last_control_at, sqlc.arg(control_at)::timestamptz)
WHERE id = sqlc.arg(computer_id) AND account_id = sqlc.arg(account_id)
  AND control_link_id = sqlc.arg(link_id)::text AND unlinked_at IS NULL
RETURNING lease_sequence;

-- name: RecordControlMint :exec
WITH minted AS (
UPDATE computers AS computer SET last_session_at = greatest(last_session_at, sqlc.arg(minted_at)::timestamptz)
WHERE computer.id = sqlc.arg(computer_id) AND computer.account_id = sqlc.arg(account_id)
  AND computer.remote_enabled AND computer.unlinked_at IS NULL
  AND EXISTS (
      SELECT 1 FROM devices AS device
      JOIN device_keys AS key ON key.device_id = device.id
      WHERE device.id = sqlc.arg(device_id)::text AND device.account_id = computer.account_id
        AND (device.revoked_at IS NULL OR device.revoked_at >= sqlc.arg(minted_at)::timestamptz)
        AND key.jkt = sqlc.arg(jkt)::text AND (
            key.state = 'active'
            OR (key.activated_at IS NOT NULL AND key.revoked_at >= sqlc.arg(minted_at)::timestamptz)
        )
  )
RETURNING computer.account_id
)
UPDATE accounts a SET activated_at = sqlc.arg(minted_at)
FROM minted WHERE a.id = minted.account_id AND a.activated_at IS NULL
  AND sqlc.arg(minted_at) BETWEEN a.created_at AND a.created_at + interval '7 days';

-- name: ListControlPeers :many
SELECT computer.id, computer.name, tunnel.hostname
FROM computers AS computer
JOIN tunnels AS tunnel ON tunnel.computer_id = computer.id AND tunnel.state = 'active'
WHERE computer.account_id = $1 AND computer.unlinked_at IS NULL AND computer.remote_enabled
  AND NOT computer.plan_limited AND computer.id <> sqlc.arg(id)
  AND (tunnel.delete_after IS NULL OR tunnel.delete_after > sqlc.arg(as_of)::timestamptz)
ORDER BY lower(computer.name), computer.id;

-- name: ActiveControlReservation :one
SELECT * FROM slot_reservations
WHERE id = $1 AND account_id = $2 AND device_id = $3 AND expires_at > $4
  AND consumed_at IS NULL AND released_at IS NULL;

-- name: CreateControlReservation :one
INSERT INTO slot_reservations (id, account_id, device_id, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ReleaseControlReservation :exec
UPDATE slot_reservations SET released_at = $4
WHERE id = $1 AND account_id = $2 AND device_id = $3 AND consumed_at IS NULL AND released_at IS NULL;

-- name: ConsumeControlReservation :execrows
UPDATE slot_reservations SET consumed_at = sqlc.arg(as_of)::timestamptz
WHERE id = sqlc.arg(id) AND account_id = sqlc.arg(account_id) AND device_id = sqlc.arg(device_id)
  AND consumed_at IS NULL AND released_at IS NULL AND expires_at > sqlc.arg(as_of);

-- name: ControlLinkToken :one
SELECT * FROM link_tokens WHERE token_sha256 = $1;

-- name: LinkedControlReservation :one
SELECT c.id
FROM link_tokens AS t
JOIN computers AS c ON c.id = t.result_computer_id AND c.account_id = t.account_id
WHERE t.reservation_id = $1 AND t.account_id = $2 AND t.device_id = $3 AND c.unlinked_at IS NULL;

-- name: CreateControlLinkToken :exec
INSERT INTO link_tokens (token_sha256, account_id, device_id, reservation_id, expires_at)
VALUES ($1, $2, $3, $4, $5);

-- name: UseControlLinkToken :exec
UPDATE link_tokens SET used_at = $2, used_by_jkt = $3, result_computer_id = $4
WHERE token_sha256 = $1 AND used_at IS NULL;

-- name: RecordControlGrant :exec
INSERT INTO grants_issued (jti, computer_id, device_id, route, issued_at)
VALUES ($1, $2, $3, $4, $5);

-- name: UseComputerProof :execrows
INSERT INTO computer_proof_replay (jkt, jti, expires_at) VALUES ($1, $2, $3)
ON CONFLICT DO NOTHING;

-- name: PurgeComputerProofs :exec
DELETE FROM computer_proof_replay WHERE expires_at < $1;

-- name: PurgeControlGrants :exec
DELETE FROM grants_issued WHERE issued_at < $1;
