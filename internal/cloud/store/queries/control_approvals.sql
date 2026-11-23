-- name: CloudApproval :one
SELECT approval.*, device.name AS device_name
FROM device_approvals AS approval
JOIN devices AS device ON device.id = approval.device_id
WHERE approval.account_id = $1 AND approval.id = $2;

-- name: ListCloudApprovals :many
SELECT approval.*, device.name AS device_name
FROM device_approvals AS approval
JOIN devices AS device ON device.id = approval.device_id
JOIN computers AS computer ON computer.id = approval.computer_id
WHERE approval.account_id = $1 AND computer.unlinked_at IS NULL
  AND (
      (approval.state = 'pending' AND approval.created_at > sqlc.arg(pending_since)::timestamptz)
      OR approval.decided_at >= sqlc.arg(decided_since)::timestamptz
  )
ORDER BY approval.created_at DESC, approval.id;

-- name: ExpireComputerApprovals :exec
UPDATE device_approvals SET state = 'expired'
WHERE computer_id = $1 AND state = 'pending' AND created_at <= $2;

-- name: RecordPendingCloudApproval :execrows
INSERT INTO device_approvals (id, account_id, computer_id, device_id, jkt, created_at)
SELECT sqlc.arg(approval_id)::text, computer.account_id, computer.id, device.id, key.jkt, sqlc.arg(created_at)
FROM computers AS computer
JOIN devices AS device ON device.account_id = computer.account_id AND device.revoked_at IS NULL
JOIN device_keys AS key ON key.device_id = device.id AND key.state = 'active'
WHERE computer.id = sqlc.arg(computer_id) AND computer.account_id = sqlc.arg(account_id)
  AND computer.unlinked_at IS NULL AND computer.remote_enabled
  AND device.id = sqlc.arg(device_id) AND key.jkt = sqlc.arg(jkt)
ON CONFLICT DO NOTHING;

-- name: ComputerApprovesKey :one
SELECT EXISTS (
    SELECT 1 FROM computer_approved_keys AS approved
    JOIN device_keys AS key ON key.jkt = approved.jkt AND key.state = 'active'
    JOIN devices AS device ON device.id = key.device_id AND device.revoked_at IS NULL
    JOIN computers AS computer ON computer.id = approved.computer_id AND computer.account_id = device.account_id
    WHERE approved.computer_id = $1 AND approved.jkt = $2 AND computer.unlinked_at IS NULL
) AS approved;

-- name: ListComputersApprovingKey :many
SELECT approved.computer_id
FROM computer_approved_keys AS approved
JOIN computers AS computer ON computer.id = approved.computer_id
WHERE computer.account_id = $1 AND computer.unlinked_at IS NULL AND approved.jkt = $2;

-- name: ClearComputerApprovedKeys :exec
DELETE FROM computer_approved_keys WHERE computer_id = $1;

-- name: InsertComputerApprovedKeys :exec
INSERT INTO computer_approved_keys (computer_id, jkt, reported_at)
SELECT computer.id, key.jkt, sqlc.arg(reported_at)::timestamptz
FROM computers AS computer
JOIN devices AS device ON device.account_id = computer.account_id AND device.revoked_at IS NULL
JOIN device_keys AS key ON key.device_id = device.id AND key.state = 'active'
WHERE computer.id = sqlc.arg(computer_id) AND computer.account_id = sqlc.arg(account_id)
  AND computer.unlinked_at IS NULL AND key.jkt = ANY(sqlc.arg(jkts)::text[]);

-- name: SaveCloudApprovalDecision :exec
UPDATE device_approvals
SET decided_by_device = $3, decided_at = $4, decision_jws = $5, applied_result = NULL
WHERE account_id = $1 AND id = $2 AND state = 'pending';

-- name: SaveCloudApprovalResult :execrows
UPDATE device_approvals
SET applied_result = sqlc.arg(result)::text,
    state = CASE WHEN sqlc.arg(result) IN ('approved', 'denied') THEN sqlc.arg(result) ELSE state END,
    decided_at = coalesce(decided_at, sqlc.arg(applied_at)::timestamptz)
WHERE account_id = sqlc.arg(account_id) AND computer_id = sqlc.arg(computer_id) AND id = sqlc.arg(id)
  AND state = 'pending' AND applied_result IS NULL;

-- name: UndeliveredCloudApprovals :many
SELECT decision_jws::text
FROM device_approvals
WHERE account_id = $1 AND computer_id = $2 AND state = 'pending'
  AND decision_jws IS NOT NULL AND applied_result IS NULL AND created_at > $3
ORDER BY decided_at, id;
