-- name: GetRemoteIdentity :one
SELECT * FROM remote_identity WHERE id = 1;

-- name: PutRemoteIdentity :exec
INSERT INTO remote_identity (
    id, computer_id, account_id, computer_key_ref, tls_root_ref, tls_root_sha256, tls_leaf_not_after, linked_at
) VALUES (1, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO NOTHING;

-- name: UpdateRemoteLeaf :exec
UPDATE remote_identity SET tls_leaf_not_after = ? WHERE id = 1 AND computer_id = ?;

-- name: DeleteRemoteIdentity :exec
DELETE FROM remote_identity;

-- name: AdvanceRemoteLeaseSequence :execrows
UPDATE remote_identity SET lease_sequence = sqlc.arg(sequence)
WHERE id = 1 AND computer_id = sqlc.arg(computer_id) AND lease_sequence < sqlc.arg(sequence);

-- name: RecordRemoteLastMint :exec
UPDATE remote_identity SET
    last_mint_at = sqlc.arg(minted_at),
    last_mint_device_id = sqlc.arg(device_id),
    last_mint_jkt = sqlc.arg(jkt)
WHERE id = 1 AND last_mint_at <= sqlc.arg(minted_at);

-- name: GetRemoteSecurityState :one
SELECT * FROM remote_security_state WHERE id = 1;

-- name: PutRemoteSecurityState :exec
INSERT INTO remote_security_state (id, applied_epoch, devices_json, applied_at)
VALUES (1, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET
    applied_epoch = excluded.applied_epoch, devices_json = excluded.devices_json, applied_at = excluded.applied_at;

-- name: DeleteRemoteSecurityState :exec
DELETE FROM remote_security_state;

-- name: InsertRemoteSession :exec
INSERT INTO remote_sessions (
    id, credential_sha256, device_id, device_jkt, scopes, route, browser, created_at, expires_at, signing_key_id
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetRemoteSession :one
SELECT * FROM remote_sessions WHERE credential_sha256 = ?;

-- name: ActiveRemoteSessions :many
SELECT * FROM remote_sessions WHERE revoked_at IS NULL AND expires_at > ?;

-- name: RevokeRemoteSession :exec
UPDATE remote_sessions SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL;

-- name: PurgeRemoteSessions :exec
DELETE FROM remote_sessions;

-- name: SweepRemoteSessions :exec
DELETE FROM remote_sessions WHERE expires_at <= ?;

-- name: RecordRemoteDeviceConnection :exec
INSERT INTO remote_device_connections (device_id, last_seen_at) VALUES (?, ?)
ON CONFLICT(device_id) DO UPDATE SET last_seen_at = MAX(last_seen_at, excluded.last_seen_at);

-- name: ListRemoteDeviceConnections :many
SELECT device_id, last_seen_at FROM remote_device_connections;

-- name: PurgeRemoteDeviceConnections :exec
DELETE FROM remote_device_connections;

-- name: HasRemoteGrant :one
SELECT EXISTS(SELECT 1 FROM remote_grant_replay WHERE jti = ?);

-- name: UseRemoteGrant :execrows
INSERT INTO remote_grant_replay (jti, expires_at) VALUES (?, ?) ON CONFLICT(jti) DO NOTHING;

-- name: PurgeRemoteGrants :exec
DELETE FROM remote_grant_replay;

-- name: SweepRemoteGrants :exec
DELETE FROM remote_grant_replay WHERE expires_at <= ?;

-- name: GetRemoteApproval :one
SELECT * FROM remote_device_approvals WHERE device_id = ? AND jkt = ?;

-- name: ListRemoteApprovals :many
SELECT * FROM remote_device_approvals ORDER BY created_at, id;

-- name: GetRemoteApprovalByID :one
SELECT * FROM remote_device_approvals WHERE id = ?;

-- name: PutRemoteApproval :exec
INSERT INTO remote_device_approvals (id, device_id, jkt, name, state, created_at, expires_at, decided_at, decision_jti)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(device_id, jkt) DO UPDATE SET
    id = excluded.id, name = excluded.name, state = excluded.state,
    created_at = excluded.created_at, expires_at = excluded.expires_at, decided_at = excluded.decided_at,
    decision_jti = excluded.decision_jti;

-- name: PurgeRemoteApprovals :exec
DELETE FROM remote_device_approvals;

-- name: UseRemoteApprovalDecision :execrows
INSERT INTO remote_approval_replay (jti, expires_at) VALUES (?, ?) ON CONFLICT(jti) DO NOTHING;

-- name: SweepRemoteApprovalDecisions :exec
DELETE FROM remote_approval_replay WHERE expires_at <= ?;

-- name: PurgeRemoteApprovalDecisions :exec
DELETE FROM remote_approval_replay;
