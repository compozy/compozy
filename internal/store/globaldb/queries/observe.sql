-- name: InsertEventSummary :exec
INSERT INTO event_summaries (
  id, session_id, workspace_id, type, agent_name, content_json, task_id, run_id,
  workflow_id, claim_token_hash, lease_until, coordinator_session_id, scheduler_reason,
  hook_event, hook_name, actor_kind, actor_id, release_reason, parent_session_id,
  root_session_id, spawn_depth, provider, outcome, summary, timestamp
) VALUES (
  sqlc.arg(id), sqlc.arg(session_id), sqlc.arg(workspace_id), sqlc.arg(type),
  sqlc.arg(agent_name), sqlc.arg(content_json), sqlc.arg(task_id), sqlc.arg(run_id),
  sqlc.arg(workflow_id), sqlc.arg(claim_token_hash), sqlc.arg(lease_until),
  sqlc.arg(coordinator_session_id), sqlc.arg(scheduler_reason), sqlc.arg(hook_event),
  sqlc.arg(hook_name), sqlc.arg(actor_kind), sqlc.arg(actor_id), sqlc.arg(release_reason),
  sqlc.arg(parent_session_id), sqlc.arg(root_session_id), sqlc.arg(spawn_depth),
  sqlc.arg(provider), sqlc.arg(outcome), sqlc.narg(summary), sqlc.arg(timestamp)
);

-- name: GetEventSummarySessionProjection :one
SELECT workspace_id, provider FROM sessions WHERE id = sqlc.arg(session_id);

-- name: DeleteEventSummariesBefore :execrows
DELETE FROM event_summaries WHERE timestamp < sqlc.arg(cutoff);

-- name: DeleteTokenStatsBefore :execrows
DELETE FROM token_stats WHERE updated_at < sqlc.arg(cutoff);

-- name: DeletePermissionLogsBefore :execrows
DELETE FROM permission_log WHERE timestamp < sqlc.arg(cutoff);

-- name: UpsertTokenStats :exec
INSERT INTO token_stats (
  id, session_id, agent_name, input_tokens, output_tokens, total_tokens,
  total_cost, cost_currency, cost_status, cost_source, turn_count, updated_at
) VALUES (
  sqlc.arg(id), sqlc.arg(session_id), sqlc.arg(agent_name), sqlc.narg(input_tokens),
  sqlc.narg(output_tokens), sqlc.narg(total_tokens), sqlc.narg(total_cost),
  sqlc.narg(cost_currency), sqlc.arg(cost_status), sqlc.arg(cost_source),
  sqlc.arg(turn_count), sqlc.arg(updated_at)
)
ON CONFLICT(session_id, agent_name) DO UPDATE SET
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
