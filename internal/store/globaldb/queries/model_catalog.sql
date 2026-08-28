-- name: UpsertModelCatalogExecutionContext :exec
INSERT INTO model_catalog_execution_contexts (
  context_id, scope, profile_id, workspace_id, command_fingerprint
) VALUES (
  sqlc.arg(context_id), sqlc.arg(scope), sqlc.arg(profile_id),
  sqlc.arg(workspace_id), sqlc.arg(command_fingerprint)
)
ON CONFLICT(context_id) DO NOTHING;

-- name: DeleteModelCatalogReasoningEfforts :exec
DELETE FROM model_catalog_reasoning_efforts
WHERE context_id = sqlc.arg(context_id)
  AND source_id = sqlc.arg(source_id) AND provider_id = sqlc.arg(provider_id);

-- name: DeleteModelCatalogRows :exec
DELETE FROM model_catalog_rows
WHERE context_id = sqlc.arg(context_id)
  AND source_id = sqlc.arg(source_id) AND provider_id = sqlc.arg(provider_id);

-- name: DeleteModelCatalogTransportBindings :exec
DELETE FROM model_catalog_transport_bindings
WHERE context_id = sqlc.arg(context_id)
  AND source_id = sqlc.arg(source_id) AND provider_id = sqlc.arg(provider_id);

-- name: DeleteModelCatalogTransportBindingSelections :exec
DELETE FROM model_catalog_transport_binding_selections
WHERE context_id = sqlc.arg(context_id)
  AND source_id = sqlc.arg(source_id) AND provider_id = sqlc.arg(provider_id);

-- name: DeleteModelCatalogOptionValues :exec
DELETE FROM model_catalog_option_values
WHERE context_id = sqlc.arg(context_id)
  AND source_id = sqlc.arg(source_id) AND provider_id = sqlc.arg(provider_id);

-- name: DeleteModelCatalogOptions :exec
DELETE FROM model_catalog_options
WHERE context_id = sqlc.arg(context_id)
  AND source_id = sqlc.arg(source_id) AND provider_id = sqlc.arg(provider_id);

-- name: UpsertModelCatalogSourceStatus :exec
INSERT INTO model_catalog_sources (
  context_id, source_id, provider_id, source_kind, priority, refresh_state,
  last_refresh_at, next_refresh_at, last_success_at, last_error, row_count, stale
) VALUES (
  sqlc.arg(context_id), sqlc.arg(source_id), sqlc.arg(provider_id), sqlc.arg(source_kind), sqlc.arg(priority),
  sqlc.arg(refresh_state), sqlc.arg(last_refresh_at), sqlc.arg(next_refresh_at),
  sqlc.arg(last_success_at), sqlc.arg(last_error), sqlc.arg(row_count), sqlc.arg(stale)
)
ON CONFLICT(context_id, source_id, provider_id) DO UPDATE SET
  source_kind = excluded.source_kind, priority = excluded.priority,
  refresh_state = excluded.refresh_state, last_refresh_at = excluded.last_refresh_at,
  next_refresh_at = excluded.next_refresh_at, last_success_at = excluded.last_success_at,
  last_error = excluded.last_error, row_count = excluded.row_count, stale = excluded.stale;

-- name: InsertModelCatalogRow :exec
INSERT INTO model_catalog_rows (
  context_id, source_id, provider_id, model_id, source_kind, priority, available, stale,
  refreshed_at, expires_at, display_name, context_window, max_input_tokens,
  max_output_tokens, supports_tools, supports_reasoning, default_reasoning_effort,
  cost_input_per_million, cost_output_per_million, cost_cache_read_per_million,
  cost_cache_write_per_million, cost_reasoning_per_million, explicitly_curated,
  deprecated, hidden, featured, deprecated_set, hidden_set, featured_set,
  release_date, last_error
) VALUES (
  sqlc.arg(context_id), sqlc.arg(source_id), sqlc.arg(provider_id), sqlc.arg(model_id), sqlc.arg(source_kind),
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
INSERT INTO model_catalog_reasoning_efforts (context_id, source_id, provider_id, model_id, effort, rank)
VALUES (
  sqlc.arg(context_id), sqlc.arg(source_id), sqlc.arg(provider_id),
  sqlc.arg(model_id), sqlc.arg(effort), sqlc.arg(rank)
);

-- name: InsertModelCatalogTransportBinding :exec
INSERT INTO model_catalog_transport_bindings (
  context_id, source_id, provider_id, model_id, transport_model_id, label,
  reasoning_effort, fast, thinking, rank
) VALUES (
  sqlc.arg(context_id), sqlc.arg(source_id), sqlc.arg(provider_id), sqlc.arg(model_id),
  sqlc.arg(transport_model_id), sqlc.arg(label), sqlc.narg(reasoning_effort),
  sqlc.narg(fast), sqlc.narg(thinking), sqlc.arg(rank)
);

-- name: ListModelCatalogTransportBindings :many
SELECT
  b.context_id,
  b.source_id,
  b.provider_id,
  b.model_id,
  b.transport_model_id,
  b.label,
  b.reasoning_effort,
  b.fast,
  b.thinking,
  b.rank
FROM model_catalog_transport_bindings b
JOIN model_catalog_rows r
  ON r.context_id = b.context_id
  AND r.source_id = b.source_id
  AND r.provider_id = b.provider_id
  AND r.model_id = b.model_id
WHERE b.context_id = CAST(sqlc.arg(context_id) AS TEXT)
  AND (CAST(sqlc.arg(provider_id) AS TEXT) = '' OR b.provider_id = CAST(sqlc.arg(provider_id) AS TEXT))
  AND (CAST(sqlc.arg(source_id) AS TEXT) = '' OR b.source_id = CAST(sqlc.arg(source_id) AS TEXT))
  AND (CAST(sqlc.arg(include_stale) AS INTEGER) = 1
    OR CAST(sqlc.arg(include_all) AS INTEGER) = 1
    OR r.stale = 0)
ORDER BY b.source_id ASC, b.provider_id ASC, b.model_id ASC, b.rank ASC, b.transport_model_id ASC;

-- name: InsertModelCatalogOption :exec
INSERT INTO model_catalog_options (
  context_id, source_id, provider_id, model_id, option_id, label, description,
  category, kind, current_value_id, current_bool
) VALUES (
  sqlc.arg(context_id), sqlc.arg(source_id), sqlc.arg(provider_id), sqlc.arg(model_id), sqlc.arg(option_id),
  sqlc.arg(label), sqlc.arg(description), sqlc.arg(category), sqlc.arg(kind),
  sqlc.narg(current_value_id), sqlc.narg(current_bool)
);

-- name: InsertModelCatalogOptionValue :exec
INSERT INTO model_catalog_option_values (
  context_id, source_id, provider_id, model_id, option_id, value_id, label, description,
  group_id, group_label, rank
) VALUES (
  sqlc.arg(context_id), sqlc.arg(source_id), sqlc.arg(provider_id), sqlc.arg(model_id), sqlc.arg(option_id),
  sqlc.arg(value_id), sqlc.arg(label), sqlc.arg(description), sqlc.arg(group_id),
  sqlc.arg(group_label), sqlc.arg(rank)
);

-- name: ListModelCatalogOptions :many
SELECT
  o.context_id,
  o.source_id,
  o.provider_id,
  o.model_id,
  o.option_id,
  o.label,
  o.description,
  o.category,
  o.kind,
  o.current_value_id,
  o.current_bool
FROM model_catalog_options o
JOIN model_catalog_rows r
  ON r.context_id = o.context_id
  AND r.source_id = o.source_id
  AND r.provider_id = o.provider_id
  AND r.model_id = o.model_id
WHERE o.context_id = CAST(sqlc.arg(context_id) AS TEXT)
  AND (CAST(sqlc.arg(provider_id) AS TEXT) = '' OR o.provider_id = CAST(sqlc.arg(provider_id) AS TEXT))
  AND (CAST(sqlc.arg(source_id) AS TEXT) = '' OR o.source_id = CAST(sqlc.arg(source_id) AS TEXT))
  AND (CAST(sqlc.arg(include_stale) AS INTEGER) = 1
    OR CAST(sqlc.arg(include_all) AS INTEGER) = 1
    OR r.stale = 0)
ORDER BY o.source_id ASC, o.provider_id ASC, o.model_id ASC, o.option_id ASC;

-- name: ListModelCatalogOptionValues :many
SELECT
  v.context_id,
  v.source_id,
  v.provider_id,
  v.model_id,
  v.option_id,
  v.value_id,
  v.label,
  v.description,
  v.group_id,
  v.group_label,
  v.rank
FROM model_catalog_option_values v
JOIN model_catalog_rows r
  ON r.context_id = v.context_id
  AND r.source_id = v.source_id
  AND r.provider_id = v.provider_id
  AND r.model_id = v.model_id
WHERE v.context_id = CAST(sqlc.arg(context_id) AS TEXT)
  AND (CAST(sqlc.arg(provider_id) AS TEXT) = '' OR v.provider_id = CAST(sqlc.arg(provider_id) AS TEXT))
  AND (CAST(sqlc.arg(source_id) AS TEXT) = '' OR v.source_id = CAST(sqlc.arg(source_id) AS TEXT))
  AND (CAST(sqlc.arg(include_stale) AS INTEGER) = 1
    OR CAST(sqlc.arg(include_all) AS INTEGER) = 1
    OR r.stale = 0)
ORDER BY v.source_id ASC, v.provider_id ASC, v.model_id ASC, v.option_id ASC, v.rank ASC, v.value_id ASC;

-- name: InsertModelCatalogTransportBindingSelection :exec
INSERT INTO model_catalog_transport_binding_selections (
  context_id, source_id, provider_id, model_id, transport_model_id, option_id, value_id, bool_value
) VALUES (
  sqlc.arg(context_id), sqlc.arg(source_id), sqlc.arg(provider_id), sqlc.arg(model_id),
  sqlc.arg(transport_model_id), sqlc.arg(option_id), sqlc.narg(value_id), sqlc.narg(bool_value)
);

-- name: ListModelCatalogTransportBindingSelections :many
SELECT
  s.context_id,
  s.source_id,
  s.provider_id,
  s.model_id,
  s.transport_model_id,
  s.option_id,
  s.value_id,
  s.bool_value
FROM model_catalog_transport_binding_selections s
JOIN model_catalog_rows r
  ON r.context_id = s.context_id
  AND r.source_id = s.source_id
  AND r.provider_id = s.provider_id
  AND r.model_id = s.model_id
WHERE s.context_id = CAST(sqlc.arg(context_id) AS TEXT)
  AND (CAST(sqlc.arg(provider_id) AS TEXT) = '' OR s.provider_id = CAST(sqlc.arg(provider_id) AS TEXT))
  AND (CAST(sqlc.arg(source_id) AS TEXT) = '' OR s.source_id = CAST(sqlc.arg(source_id) AS TEXT))
  AND (CAST(sqlc.arg(include_stale) AS INTEGER) = 1
    OR CAST(sqlc.arg(include_all) AS INTEGER) = 1
    OR r.stale = 0)
ORDER BY s.source_id ASC, s.provider_id ASC, s.model_id ASC,
  s.transport_model_id ASC, s.option_id ASC;
