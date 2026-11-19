-- name: UpsertAgentInstanceDailyBudgetLimits :exec
INSERT INTO agent_instance_daily_budget_limits (instance_id, token_cap, usd_cap, updated_at)
VALUES (sqlc.arg(instance_id), sqlc.narg(token_cap), sqlc.narg(usd_cap), sqlc.arg(updated_at))
ON CONFLICT(instance_id) DO UPDATE SET
    token_cap = excluded.token_cap,
    usd_cap = excluded.usd_cap,
    updated_at = excluded.updated_at;

-- name: GetAgentInstanceDailyBudgetLimits :one
SELECT token_cap, usd_cap FROM agent_instance_daily_budget_limits
WHERE instance_id = sqlc.arg(instance_id);

-- name: ReserveAgentInstanceDailyBudgetNotice :execrows
INSERT INTO agent_instance_daily_budget_notices (instance_id, day, issued_at)
VALUES (sqlc.arg(instance_id), sqlc.arg(day), sqlc.arg(issued_at))
ON CONFLICT(instance_id, day) DO NOTHING;

-- name: GetAgentInstanceDailyBudgetNotices :many
SELECT day, issued_at FROM agent_instance_daily_budget_notices
WHERE instance_id = sqlc.arg(instance_id)
ORDER BY day DESC;

-- name: ListInstanceDailyBudgetReceiptPage :many
SELECT session_id, turn_id, total_tokens, usd_amount, cost_status
FROM token_usage_receipts
WHERE profile_id = sqlc.arg(profile_id)
  AND instance_id = sqlc.arg(instance_id)
  AND day = sqlc.arg(day)
  AND (session_id, turn_id) > (sqlc.arg(after_session_id), sqlc.arg(after_turn_id))
ORDER BY session_id, turn_id
LIMIT sqlc.arg(page_size);
