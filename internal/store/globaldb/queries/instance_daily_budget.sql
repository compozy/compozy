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

-- name: HasDailyBudgetEnforcementForSession :one
SELECT CAST(EXISTS (
    SELECT 1 FROM agent_instances instance
    LEFT JOIN agent_instance_daily_budget_limits limits ON limits.instance_id = instance.id
    LEFT JOIN agent_instance_daily_budget_recovery recovery ON recovery.instance_id = instance.id
    WHERE instance.profile_id = session.profile_id AND instance.archived_at IS NULL
      AND (limits.token_cap IS NOT NULL OR limits.usd_cap IS NOT NULL OR recovery.latched = 1
           OR (instance.state = 'paused' AND instance.pause_reason IN ('daily_budget', 'daily_budget_recovery')))
) AS INTEGER) AS requires_enforcement
FROM sessions session WHERE session.id = sqlc.arg(session_id);

-- name: ReserveAgentInstanceDailyBudgetNotice :execrows
INSERT INTO agent_instance_daily_budget_notices (instance_id, day, issued_at)
VALUES (sqlc.arg(instance_id), sqlc.arg(day), sqlc.arg(issued_at))
ON CONFLICT(instance_id, day) DO NOTHING;

-- name: GetAgentInstanceDailyBudgetNotices :many
SELECT day, issued_at FROM agent_instance_daily_budget_notices
WHERE instance_id = sqlc.arg(instance_id)
ORDER BY day DESC;

-- name: ListInstanceDailyBudgetReceiptPage :many
SELECT session_id, turn_id,
       CAST(CASE WHEN typeof(total_tokens) = 'integer' THEN total_tokens ELSE -1 END AS INTEGER) AS total_tokens,
       usd_amount, cost_status
FROM token_usage_receipts
WHERE profile_id = sqlc.arg(profile_id)
  AND instance_id = sqlc.arg(instance_id)
  AND day = sqlc.arg(day)
  AND (session_id, turn_id) > (sqlc.arg(after_session_id), sqlc.arg(after_turn_id))
ORDER BY session_id, turn_id
LIMIT sqlc.arg(page_size);

-- name: ReadInstanceDailyBudgetTotal :one
SELECT CAST(CASE WHEN typeof(total_tokens) = 'integer' THEN total_tokens ELSE -1 END AS INTEGER) AS total_tokens,
       usd_total, has_unpriced_usage, invalid
FROM agent_instance_daily_budget_usage
WHERE instance_id = sqlc.arg(instance_id) AND profile_id = sqlc.arg(profile_id) AND day = sqlc.arg(day);

-- name: WriteInstanceDailyBudgetTotal :exec
INSERT INTO agent_instance_daily_budget_usage(
    instance_id, profile_id, day, total_tokens, usd_total, has_unpriced_usage, invalid
) VALUES (sqlc.arg(instance_id), sqlc.arg(profile_id), sqlc.arg(day), sqlc.arg(total_tokens),
          sqlc.narg(usd_total), sqlc.arg(has_unpriced_usage), sqlc.arg(invalid))
ON CONFLICT(instance_id, profile_id, day) DO UPDATE SET total_tokens = excluded.total_tokens,
    usd_total = excluded.usd_total, has_unpriced_usage = excluded.has_unpriced_usage, invalid = excluded.invalid;

-- name: HasInstanceDailyBudgetNotice :one
SELECT count(*) FROM agent_instance_daily_budget_notices
WHERE instance_id = sqlc.arg(instance_id) AND day = sqlc.arg(day);
