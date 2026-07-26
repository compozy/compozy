-- name: UpsertTokenUsageDaily :exec
INSERT INTO token_usage_daily (
  day, workspace_id, agent_name, input_tokens, output_tokens, total_tokens,
  total_cost, cost_currency, cost_status, cost_source, turn_count, updated_at
) VALUES (
  sqlc.arg(day), sqlc.arg(workspace_id), sqlc.arg(agent_name), sqlc.arg(input_tokens),
  sqlc.arg(output_tokens), sqlc.arg(total_tokens), sqlc.narg(total_cost),
  sqlc.narg(cost_currency), sqlc.arg(cost_status), sqlc.arg(cost_source),
  sqlc.arg(turn_count), sqlc.arg(updated_at)
)
-- Cost provenance mismatch (keep identical across total_cost/cost_currency/cost_status/cost_source):
--   token_usage_daily.cost_status != excluded.cost_status
--   OR token_usage_daily.cost_source != excluded.cost_source
--   OR COALESCE(token_usage_daily.cost_currency, '') != COALESCE(excluded.cost_currency, '')
--   OR float64 overflow on the additive total_cost (mirrors token_stats)
-- On mismatch: total_cost/cost_currency -> NULL, cost_status -> 'unknown', cost_source -> 'none'.
ON CONFLICT(day, workspace_id, agent_name) DO UPDATE SET
  input_tokens = token_usage_daily.input_tokens + excluded.input_tokens,
  output_tokens = token_usage_daily.output_tokens + excluded.output_tokens,
  total_tokens = token_usage_daily.total_tokens + excluded.total_tokens,
  total_cost = CASE
    WHEN token_usage_daily.cost_status != excluded.cost_status
      OR token_usage_daily.cost_source != excluded.cost_source
      OR COALESCE(token_usage_daily.cost_currency, '') != COALESCE(excluded.cost_currency, '')
      OR (token_usage_daily.total_cost IS NOT NULL AND excluded.total_cost IS NOT NULL
        AND token_usage_daily.total_cost + excluded.total_cost > 1.7976931348623157e308)
      THEN NULL
    WHEN excluded.total_cost IS NULL THEN token_usage_daily.total_cost
    WHEN token_usage_daily.total_cost IS NULL THEN excluded.total_cost
    ELSE token_usage_daily.total_cost + excluded.total_cost
  END,
  cost_currency = CASE
    WHEN token_usage_daily.cost_status != excluded.cost_status
      OR token_usage_daily.cost_source != excluded.cost_source
      OR COALESCE(token_usage_daily.cost_currency, '') != COALESCE(excluded.cost_currency, '')
      OR (token_usage_daily.total_cost IS NOT NULL AND excluded.total_cost IS NOT NULL
        AND token_usage_daily.total_cost + excluded.total_cost > 1.7976931348623157e308)
      THEN NULL
    ELSE COALESCE(excluded.cost_currency, token_usage_daily.cost_currency)
  END,
  cost_status = CASE
    WHEN token_usage_daily.cost_status != excluded.cost_status
      OR token_usage_daily.cost_source != excluded.cost_source
      OR COALESCE(token_usage_daily.cost_currency, '') != COALESCE(excluded.cost_currency, '')
      OR (token_usage_daily.total_cost IS NOT NULL AND excluded.total_cost IS NOT NULL
        AND token_usage_daily.total_cost + excluded.total_cost > 1.7976931348623157e308)
      THEN 'unknown'
    ELSE token_usage_daily.cost_status
  END,
  cost_source = CASE
    WHEN token_usage_daily.cost_status != excluded.cost_status
      OR token_usage_daily.cost_source != excluded.cost_source
      OR COALESCE(token_usage_daily.cost_currency, '') != COALESCE(excluded.cost_currency, '')
      OR (token_usage_daily.total_cost IS NOT NULL AND excluded.total_cost IS NOT NULL
        AND token_usage_daily.total_cost + excluded.total_cost > 1.7976931348623157e308)
      THEN 'none'
    ELSE token_usage_daily.cost_source
  END,
  turn_count = token_usage_daily.turn_count + excluded.turn_count,
  updated_at = excluded.updated_at;

-- name: DeleteTokenUsageDailyBefore :execrows
DELETE FROM token_usage_daily WHERE day < sqlc.arg(cutoff_day);

-- name: ListTokenUsageDailyByDay :many
SELECT day,
  CAST(SUM(input_tokens) AS INTEGER) AS input_tokens,
  CAST(SUM(output_tokens) AS INTEGER) AS output_tokens,
  CAST(SUM(total_tokens) AS INTEGER) AS total_tokens
FROM token_usage_daily
WHERE day >= sqlc.arg(since_day)
  AND (CAST(sqlc.arg(workspace_id) AS TEXT) = '' OR workspace_id = CAST(sqlc.arg(workspace_id) AS TEXT))
GROUP BY day
ORDER BY day;

-- name: ListTokenUsageDailyByAgent :many
SELECT agent_name,
  CAST(SUM(total_tokens) AS INTEGER) AS total_tokens
FROM token_usage_daily
WHERE day >= sqlc.arg(since_day)
  AND (CAST(sqlc.arg(workspace_id) AS TEXT) = '' OR workspace_id = CAST(sqlc.arg(workspace_id) AS TEXT))
GROUP BY agent_name
ORDER BY SUM(total_tokens) DESC, agent_name;

-- name: SumTokenUsageDailyCost :many
SELECT cost_status, cost_source, COALESCE(cost_currency, '') AS cost_currency,
  CAST(SUM(COALESCE(total_cost, 0)) AS REAL) AS total_cost,
  CAST(SUM(CASE WHEN total_cost IS NULL THEN 1 ELSE 0 END) AS INTEGER) AS rows_without_cost,
  COUNT(1) AS rows_total
FROM token_usage_daily
WHERE day >= sqlc.arg(since_day)
  AND (CAST(sqlc.arg(workspace_id) AS TEXT) = '' OR workspace_id = CAST(sqlc.arg(workspace_id) AS TEXT))
GROUP BY cost_status, cost_source, COALESCE(cost_currency, '');

-- name: CountTaskRunOutcomesByDay :many
SELECT CAST(date(ended_at, 'localtime') AS TEXT) AS day,
  CAST(SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) AS INTEGER) AS completed,
  CAST(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) AS INTEGER) AS failed,
  CAST(SUM(CASE WHEN status = 'canceled' THEN 1 ELSE 0 END) AS INTEGER) AS canceled
FROM task_runs
WHERE ended_at IS NOT NULL
  AND ended_at >= CAST(sqlc.arg(since) AS TEXT)
  AND status IN ('completed', 'failed', 'canceled')
  AND run_kind = 'worker'
  AND (CAST(sqlc.arg(workspace_id) AS TEXT) = '' OR COALESCE(workspace_id, '') = CAST(sqlc.arg(workspace_id) AS TEXT))
GROUP BY date(ended_at, 'localtime')
ORDER BY day;

-- name: CountTasksClosedByDay :many
SELECT CAST(date(closed_at, 'localtime') AS TEXT) AS day, COUNT(1) AS closed
FROM tasks
WHERE closed_at IS NOT NULL
  AND closed_at >= CAST(sqlc.arg(since) AS TEXT)
  AND status = 'completed'
  AND (CAST(sqlc.arg(workspace_id) AS TEXT) = '' OR COALESCE(workspace_id, '') = CAST(sqlc.arg(workspace_id) AS TEXT))
GROUP BY date(closed_at, 'localtime')
ORDER BY day;

-- name: CountEventSummariesByHourWeekday :many
SELECT CAST(strftime('%w', timestamp, 'localtime') AS INTEGER) AS weekday,
  CAST(strftime('%H', timestamp, 'localtime') AS INTEGER) AS hour,
  COUNT(1) AS events
FROM event_summaries
WHERE timestamp >= sqlc.arg(since)
  AND (CAST(sqlc.arg(workspace_id) AS TEXT) = '' OR workspace_id = CAST(sqlc.arg(workspace_id) AS TEXT))
GROUP BY weekday, hour
ORDER BY weekday, hour;

-- name: MaxEventSummaryTimestamp :one
SELECT CAST(COALESCE(MAX(timestamp), '') AS TEXT) AS latest
FROM event_summaries
WHERE (CAST(sqlc.arg(workspace_id) AS TEXT) = '' OR workspace_id = CAST(sqlc.arg(workspace_id) AS TEXT));

-- name: CountNetworkAuditSince :one
SELECT COUNT(1) AS messages
FROM network_audit_log
WHERE timestamp >= sqlc.arg(since)
  AND (CAST(sqlc.arg(workspace_id) AS TEXT) = '' OR workspace_id = CAST(sqlc.arg(workspace_id) AS TEXT));

-- name: CountHookDispatchSince :one
SELECT COUNT(1) AS runs,
  CAST(COALESCE(SUM(CASE WHEN outcome = 'failure' THEN 1 ELSE 0 END), 0) AS INTEGER) AS failures
FROM event_summaries
WHERE type = 'hook.dispatch.complete'
  AND timestamp >= sqlc.arg(since)
  AND (CAST(sqlc.arg(workspace_id) AS TEXT) = '' OR workspace_id = CAST(sqlc.arg(workspace_id) AS TEXT));

-- name: LongestUserSessionSince :many
SELECT id, agent_name, created_at,
  CAST(MAX(0, CAST(strftime('%s', CASE WHEN state = 'active' THEN CAST(sqlc.arg(now) AS TEXT) ELSE updated_at END) AS INTEGER) -
    CAST(strftime('%s', created_at) AS INTEGER)) AS INTEGER) AS runtime_seconds
FROM sessions
WHERE session_type = 'user'
  AND created_at >= sqlc.arg(since)
  AND (CAST(sqlc.arg(workspace_id) AS TEXT) = '' OR workspace_id = CAST(sqlc.arg(workspace_id) AS TEXT))
ORDER BY runtime_seconds DESC, created_at DESC
LIMIT 1;
