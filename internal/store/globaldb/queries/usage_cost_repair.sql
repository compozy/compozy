-- name: UsageCostRepairPending :one
SELECT CAST(COALESCE((SELECT pending FROM usage_cost_repair_state WHERE singleton = 1), 0) AS INTEGER) AS pending;

-- name: ListUsageCostRepairSessions :many
SELECT t.id, t.session_id, t.agent_name, t.turn_count, s.workspace_id, s.profile_id
FROM token_stats t JOIN sessions s ON s.id = t.session_id
WHERE t.cost_source = 'agent_reported' ORDER BY t.session_id, t.agent_name;

-- name: ListUsageCostRepairDays :many
SELECT day, profile_id, workspace_id, agent_name FROM token_usage_daily
WHERE cost_source = 'agent_reported' ORDER BY day, profile_id, workspace_id, agent_name;

-- name: InvalidateReportedSessionCosts :exec
UPDATE token_stats SET total_cost = NULL, cost_currency = NULL, cost_status = 'unknown', cost_source = 'none'
WHERE cost_source = 'agent_reported';

-- name: InvalidateReportedDailyCosts :exec
UPDATE token_usage_daily SET total_cost = NULL, cost_currency = NULL, cost_status = 'unknown', cost_source = 'none'
WHERE cost_source = 'agent_reported';

-- name: RepairSessionUsageCost :exec
UPDATE token_stats SET total_cost = sqlc.arg(amount), cost_currency = sqlc.arg(currency),
    cost_status = 'actual', cost_source = 'agent_reported' WHERE id = sqlc.arg(id);

-- name: RepairDailyUsageCost :exec
UPDATE token_usage_daily SET total_cost = sqlc.arg(amount), cost_currency = sqlc.arg(currency),
    cost_status = 'actual', cost_source = 'agent_reported'
WHERE day = sqlc.arg(day) AND profile_id = sqlc.arg(profile_id)
    AND workspace_id = sqlc.arg(workspace_id) AND agent_name = sqlc.arg(agent_name)
    AND turn_count = sqlc.arg(turn_count);

-- name: CompleteUsageCostRepair :exec
UPDATE usage_cost_repair_state SET pending = 0 WHERE singleton = 1;
