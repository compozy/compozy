-- name: GetTokenUsageReceiptProfile :one
SELECT profile_id FROM token_usage_receipts
WHERE session_id = sqlc.arg(session_id) AND turn_id = sqlc.arg(turn_id);

-- name: GetTokenUsageReceiptSessionOwner :one
SELECT profile_id, COALESCE(instance_id, '') AS instance_id FROM sessions
WHERE id = sqlc.arg(session_id);

-- name: InsertTokenUsageReceipt :execrows
INSERT INTO token_usage_receipts (
    profile_id, session_id, turn_id, instance_id, day,
    total_tokens, usd_amount, cost_status, created_at
) VALUES (
    sqlc.arg(profile_id), sqlc.arg(session_id), sqlc.arg(turn_id),
    sqlc.arg(instance_id), sqlc.arg(day), sqlc.arg(total_tokens),
    sqlc.narg(usd_amount), sqlc.arg(cost_status), sqlc.arg(created_at)
) ON CONFLICT(session_id, turn_id) DO NOTHING;
