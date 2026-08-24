-- name: UpsertTokenUsageDaily :exec
INSERT INTO token_usage_daily (
  profile_id,
  day, workspace_id, agent_name, input_tokens, output_tokens, total_tokens,
  total_cost, cost_currency, cost_status, cost_source, turn_count, updated_at
) VALUES (
  sqlc.arg(profile_id),
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
ON CONFLICT(day, profile_id, workspace_id, agent_name) DO UPDATE SET
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
