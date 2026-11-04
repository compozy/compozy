-- name: GetEventSummarySessionProjection :one
SELECT workspace_id, provider, worktree_id FROM sessions WHERE id = sqlc.arg(session_id);

-- name: HasExpiredObservability :one
SELECT CAST(
  EXISTS(SELECT 1 FROM token_stats WHERE token_stats.updated_at < sqlc.arg(cutoff))
  OR EXISTS(SELECT 1 FROM token_usage_daily WHERE token_usage_daily.day < sqlc.arg(cutoff_day))
  OR EXISTS(SELECT 1 FROM permission_log WHERE permission_log.timestamp < sqlc.arg(cutoff))
AS INTEGER);

-- name: DeleteTokenStatsBefore :execrows
DELETE FROM token_stats WHERE id IN (
 SELECT t.id FROM token_stats AS t WHERE t.updated_at < sqlc.arg(cutoff) LIMIT sqlc.arg(batch_size)
);

-- name: DeletePermissionLogsBefore :execrows
DELETE FROM permission_log WHERE id IN (
 SELECT p.id FROM permission_log AS p WHERE p.timestamp < sqlc.arg(cutoff) LIMIT sqlc.arg(batch_size)
);

-- name: UpsertTokenStats :exec
INSERT INTO token_stats (
  cache_read_tokens, cache_write_tokens,
  id, session_id, agent_name, input_tokens, output_tokens, total_tokens,
  total_cost, cost_currency, cost_status, cost_source, turn_count, updated_at
) VALUES (
  sqlc.narg(cache_read_tokens), sqlc.narg(cache_write_tokens),
  sqlc.arg(id), sqlc.arg(session_id), sqlc.arg(agent_name), sqlc.narg(input_tokens),
  sqlc.narg(output_tokens), sqlc.narg(total_tokens), sqlc.narg(total_cost),
  sqlc.narg(cost_currency), sqlc.arg(cost_status), sqlc.arg(cost_source),
  sqlc.arg(turn_count), sqlc.arg(updated_at)
)
ON CONFLICT(session_id, agent_name) DO UPDATE SET
  cache_read_tokens = CASE
    WHEN excluded.cache_read_tokens IS NULL THEN token_stats.cache_read_tokens
    WHEN token_stats.cache_read_tokens IS NULL THEN excluded.cache_read_tokens
    ELSE token_stats.cache_read_tokens + excluded.cache_read_tokens
  END,
  cache_write_tokens = CASE
    WHEN excluded.cache_write_tokens IS NULL THEN token_stats.cache_write_tokens
    WHEN token_stats.cache_write_tokens IS NULL THEN excluded.cache_write_tokens
    ELSE token_stats.cache_write_tokens + excluded.cache_write_tokens
  END,
  input_tokens = CASE
    WHEN excluded.input_tokens IS NULL THEN token_stats.input_tokens
    WHEN token_stats.input_tokens IS NULL THEN excluded.input_tokens
    ELSE token_stats.input_tokens + excluded.input_tokens
  END,
  output_tokens = CASE
    WHEN excluded.output_tokens IS NULL THEN token_stats.output_tokens
    WHEN token_stats.output_tokens IS NULL THEN excluded.output_tokens
    ELSE token_stats.output_tokens + excluded.output_tokens
  END,
  total_tokens = CASE
    WHEN excluded.total_tokens IS NULL THEN token_stats.total_tokens
    WHEN token_stats.total_tokens IS NULL THEN excluded.total_tokens
    ELSE token_stats.total_tokens + excluded.total_tokens
  END,
  total_cost = CASE
    WHEN token_stats.cost_status != excluded.cost_status
      OR token_stats.cost_source != excluded.cost_source
      OR COALESCE(token_stats.cost_currency, '') != COALESCE(excluded.cost_currency, '')
      OR (token_stats.total_cost IS NOT NULL AND excluded.total_cost IS NOT NULL
        AND token_stats.total_cost + excluded.total_cost > 1.7976931348623157e308)
      THEN NULL
    WHEN excluded.total_cost IS NULL THEN token_stats.total_cost
    WHEN token_stats.total_cost IS NULL THEN excluded.total_cost
    ELSE token_stats.total_cost + excluded.total_cost
  END,
  cost_currency = CASE
    WHEN token_stats.cost_status != excluded.cost_status
      OR token_stats.cost_source != excluded.cost_source
      OR COALESCE(token_stats.cost_currency, '') != COALESCE(excluded.cost_currency, '')
      OR (token_stats.total_cost IS NOT NULL AND excluded.total_cost IS NOT NULL
        AND token_stats.total_cost + excluded.total_cost > 1.7976931348623157e308)
      THEN NULL
    ELSE COALESCE(excluded.cost_currency, token_stats.cost_currency)
  END,
  cost_status = CASE
    WHEN token_stats.cost_status != excluded.cost_status
      OR token_stats.cost_source != excluded.cost_source
      OR COALESCE(token_stats.cost_currency, '') != COALESCE(excluded.cost_currency, '')
      OR (token_stats.total_cost IS NOT NULL AND excluded.total_cost IS NOT NULL
        AND token_stats.total_cost + excluded.total_cost > 1.7976931348623157e308)
      THEN 'unknown'
    ELSE token_stats.cost_status
  END,
  cost_source = CASE
    WHEN token_stats.cost_status != excluded.cost_status
      OR token_stats.cost_source != excluded.cost_source
      OR COALESCE(token_stats.cost_currency, '') != COALESCE(excluded.cost_currency, '')
      OR (token_stats.total_cost IS NOT NULL AND excluded.total_cost IS NOT NULL
        AND token_stats.total_cost + excluded.total_cost > 1.7976931348623157e308)
      THEN 'none'
    ELSE token_stats.cost_source
  END,
  turn_count = token_stats.turn_count + excluded.turn_count,
  updated_at = excluded.updated_at;

-- name: GetTelemetryWriteOwner :one
SELECT state,
 CAST(EXISTS(SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = profiles.id AND status <> 'done') AS INTEGER) AS unavailable,
 CAST(EXISTS(SELECT 1 FROM telemetry_deleted_scopes WHERE kind = 'workspace' AND scope_id = sqlc.arg(workspace_id)) AS INTEGER) AS workspace_deleted
FROM profiles WHERE profiles.id = sqlc.arg(profile_id);

-- name: ListTelemetryDeletedScopes :many
SELECT kind, scope_id FROM telemetry_deleted_scopes ORDER BY kind, scope_id;
