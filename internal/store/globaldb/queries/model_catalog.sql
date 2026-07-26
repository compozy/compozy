-- name: DeleteModelCatalogReasoningEfforts :exec
DELETE FROM model_catalog_reasoning_efforts
WHERE source_id = sqlc.arg(source_id) AND provider_id = sqlc.arg(provider_id);

-- name: DeleteModelCatalogRows :exec
DELETE FROM model_catalog_rows
WHERE source_id = sqlc.arg(source_id) AND provider_id = sqlc.arg(provider_id);

-- name: UpsertModelCatalogSourceStatus :exec
INSERT INTO model_catalog_sources (
  source_id, provider_id, source_kind, priority, refresh_state,
  last_refresh_at, next_refresh_at, last_success_at, last_error, row_count, stale
) VALUES (
  sqlc.arg(source_id), sqlc.arg(provider_id), sqlc.arg(source_kind), sqlc.arg(priority),
  sqlc.arg(refresh_state), sqlc.arg(last_refresh_at), sqlc.arg(next_refresh_at),
  sqlc.arg(last_success_at), sqlc.arg(last_error), sqlc.arg(row_count), sqlc.arg(stale)
)
ON CONFLICT(source_id, provider_id) DO UPDATE SET
  source_kind = excluded.source_kind, priority = excluded.priority,
  refresh_state = excluded.refresh_state, last_refresh_at = excluded.last_refresh_at,
  next_refresh_at = excluded.next_refresh_at, last_success_at = excluded.last_success_at,
  last_error = excluded.last_error, row_count = excluded.row_count, stale = excluded.stale;

-- name: InsertModelCatalogRow :exec
INSERT INTO model_catalog_rows (
  source_id, provider_id, model_id, source_kind, priority, available, stale,
  refreshed_at, expires_at, display_name, context_window, max_input_tokens,
  max_output_tokens, supports_tools, supports_reasoning, default_reasoning_effort,
  cost_input_per_million, cost_output_per_million, cost_cache_read_per_million,
  cost_cache_write_per_million, cost_reasoning_per_million, explicitly_curated,
  deprecated, hidden, featured, deprecated_set, hidden_set, featured_set,
  release_date, last_error
) VALUES (
  sqlc.arg(source_id), sqlc.arg(provider_id), sqlc.arg(model_id), sqlc.arg(source_kind),
  sqlc.arg(priority), sqlc.narg(available), sqlc.arg(stale), sqlc.arg(refreshed_at),
  sqlc.arg(expires_at), sqlc.arg(display_name), sqlc.narg(context_window),
  sqlc.narg(max_input_tokens), sqlc.narg(max_output_tokens), sqlc.narg(supports_tools),
  sqlc.narg(supports_reasoning), sqlc.narg(default_reasoning_effort),
  sqlc.narg(cost_input_per_million), sqlc.narg(cost_output_per_million),
  sqlc.narg(cost_cache_read_per_million), sqlc.narg(cost_cache_write_per_million),
  sqlc.narg(cost_reasoning_per_million),
  sqlc.arg(explicitly_curated), sqlc.arg(deprecated), sqlc.arg(hidden), sqlc.arg(featured),
  sqlc.arg(deprecated_set), sqlc.arg(hidden_set), sqlc.arg(featured_set),
  sqlc.narg(release_date), sqlc.arg(last_error)
);

-- name: InsertModelCatalogReasoningEffort :exec
INSERT INTO model_catalog_reasoning_efforts (source_id, provider_id, model_id, effort, rank)
VALUES (sqlc.arg(source_id), sqlc.arg(provider_id), sqlc.arg(model_id), sqlc.arg(effort), sqlc.arg(rank));
