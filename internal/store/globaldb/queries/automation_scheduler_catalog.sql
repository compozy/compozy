-- name: GetAutomationSchedulerState :one
SELECT job_id, next_run_at, last_run_at, last_scheduled_at, last_fire_id,
       schedule_hash, catch_up_policy, misfire_grace_seconds,
       consecutive_resume_failures, last_misfire_at, misfire_count, updated_at
FROM automation_scheduler_state WHERE job_id = sqlc.arg(job_id);

-- name: ListAutomationSchedulerStates :many
SELECT job_id, next_run_at, last_run_at, last_scheduled_at, last_fire_id,
       schedule_hash, catch_up_policy, misfire_grace_seconds,
       consecutive_resume_failures, last_misfire_at, misfire_count, updated_at
FROM automation_scheduler_state ORDER BY job_id ASC;

-- name: UpsertAutomationSchedulerState :exec
INSERT INTO automation_scheduler_state (
  job_id, next_run_at, last_run_at, last_scheduled_at, last_fire_id,
  schedule_hash, catch_up_policy, misfire_grace_seconds,
  consecutive_resume_failures, last_misfire_at, misfire_count, updated_at
) VALUES (
  sqlc.arg(job_id), sqlc.narg(next_run_at), sqlc.narg(last_run_at),
  sqlc.narg(last_scheduled_at), sqlc.arg(last_fire_id), sqlc.arg(schedule_hash),
  sqlc.arg(catch_up_policy), sqlc.arg(misfire_grace_seconds),
  sqlc.arg(consecutive_resume_failures), sqlc.narg(last_misfire_at),
  sqlc.arg(misfire_count), sqlc.arg(updated_at)
)
ON CONFLICT(job_id) DO UPDATE SET
  next_run_at = excluded.next_run_at, last_run_at = excluded.last_run_at,
  last_scheduled_at = excluded.last_scheduled_at, last_fire_id = excluded.last_fire_id,
  schedule_hash = excluded.schedule_hash, catch_up_policy = excluded.catch_up_policy,
  misfire_grace_seconds = excluded.misfire_grace_seconds,
  consecutive_resume_failures = excluded.consecutive_resume_failures,
  last_misfire_at = excluded.last_misfire_at, misfire_count = excluded.misfire_count,
  updated_at = excluded.updated_at;

-- name: DeleteAutomationSchedulerState :exec
DELETE FROM automation_scheduler_state WHERE job_id = sqlc.arg(job_id);

-- name: RecordAutomationRunDeliveryError :execrows
UPDATE automation_runs
SET delivery_error = sqlc.arg(delivery_error), delivery_error_at = sqlc.arg(delivery_error_at)
WHERE id = sqlc.arg(id);

-- name: UpsertAutomationJobCatalog :exec
INSERT INTO automation_job_catalog_entries (
  job_id, scope, workspace_id, source, source_rank, name, loop_name, enabled,
  search_name, search_agent_name, search_prompt, search_scope, search_source,
  search_schedule_mode, search_schedule_expr, search_schedule_interval, search_schedule_time
) VALUES (
  sqlc.arg(job_id), sqlc.arg(scope), sqlc.arg(workspace_id), sqlc.arg(source),
  sqlc.arg(source_rank), sqlc.arg(name), sqlc.arg(loop_name), sqlc.arg(enabled),
  sqlc.arg(search_name), sqlc.arg(search_agent_name), sqlc.arg(search_prompt),
  sqlc.arg(search_scope), sqlc.arg(search_source), sqlc.arg(search_schedule_mode),
  sqlc.arg(search_schedule_expr), sqlc.arg(search_schedule_interval), sqlc.arg(search_schedule_time)
)
ON CONFLICT(job_id) DO UPDATE SET
  scope = excluded.scope, workspace_id = excluded.workspace_id, source = excluded.source,
  source_rank = excluded.source_rank, name = excluded.name, loop_name = excluded.loop_name,
  enabled = excluded.enabled, search_name = excluded.search_name,
  search_agent_name = excluded.search_agent_name, search_prompt = excluded.search_prompt,
  search_scope = excluded.search_scope, search_source = excluded.search_source,
  search_schedule_mode = excluded.search_schedule_mode,
  search_schedule_expr = excluded.search_schedule_expr,
  search_schedule_interval = excluded.search_schedule_interval,
  search_schedule_time = excluded.search_schedule_time;

-- name: UpsertAutomationTriggerCatalog :exec
INSERT INTO automation_trigger_catalog_entries (
  trigger_id, scope, workspace_id, event, source, source_rank, name, loop_name, enabled,
  search_name, search_agent_name, search_prompt, search_scope, search_source,
  search_event, search_endpoint_slug, search_webhook_id
) VALUES (
  sqlc.arg(trigger_id), sqlc.arg(scope), sqlc.arg(workspace_id), sqlc.arg(event),
  sqlc.arg(source), sqlc.arg(source_rank), sqlc.arg(name), sqlc.arg(loop_name),
  sqlc.arg(enabled), sqlc.arg(search_name), sqlc.arg(search_agent_name),
  sqlc.arg(search_prompt), sqlc.arg(search_scope), sqlc.arg(search_source),
  sqlc.arg(search_event), sqlc.arg(search_endpoint_slug), sqlc.arg(search_webhook_id)
)
ON CONFLICT(trigger_id) DO UPDATE SET
  scope = excluded.scope, workspace_id = excluded.workspace_id, event = excluded.event,
  source = excluded.source, source_rank = excluded.source_rank, name = excluded.name,
  loop_name = excluded.loop_name, enabled = excluded.enabled, search_name = excluded.search_name,
  search_agent_name = excluded.search_agent_name, search_prompt = excluded.search_prompt,
  search_scope = excluded.search_scope, search_source = excluded.search_source,
  search_event = excluded.search_event, search_endpoint_slug = excluded.search_endpoint_slug,
  search_webhook_id = excluded.search_webhook_id;

-- name: DeleteAutomationTriggerCatalogTerms :exec
DELETE FROM automation_trigger_catalog_filter_terms WHERE trigger_id = sqlc.arg(trigger_id);

-- name: InsertAutomationTriggerCatalogTerms :exec
INSERT INTO automation_trigger_catalog_filter_terms (trigger_id, value)
SELECT sqlc.arg(trigger_id), CAST(value AS TEXT) FROM json_each(sqlc.arg(terms_json));

-- name: HydrateAutomationJobCatalog :many
WITH requested(job_id, ordinal) AS (
  SELECT CAST(value AS TEXT), CAST(key AS INTEGER) FROM json_each(sqlc.arg(ids_json))
)
SELECT j.id, j.scope, j.name, j.agent_name, j.workspace_id, j.prompt, j.schedule, j.task,
  CASE WHEN j.source IN ('config', 'package') AND o.enabled_override IS NOT NULL
    THEN o.enabled_override ELSE j.enabled END AS enabled,
  j.retry, j.fire_limit, j.source, j.target_kind, j.loop_workspace_id,
	j.loop_name, j.loop_inputs, j.loop_input_mapping, j.loop_network_participation,
	j.created_at, j.updated_at
FROM requested
JOIN automation_jobs AS j ON j.id = requested.job_id
LEFT JOIN automation_job_overlays AS o ON o.job_id = j.id
ORDER BY requested.ordinal;

-- name: HydrateAutomationTriggerCatalog :many
WITH requested(trigger_id, ordinal) AS (
  SELECT CAST(value AS TEXT), CAST(key AS INTEGER) FROM json_each(sqlc.arg(ids_json))
)
SELECT t.id, t.scope, t.name, t.agent_name, t.workspace_id, t.prompt, t.event, t.filter,
  CASE WHEN t.source IN ('config', 'package') AND o.enabled_override IS NOT NULL
    THEN o.enabled_override ELSE t.enabled END AS enabled,
  t.retry, t.fire_limit, t.source, t.webhook_id, t.endpoint_slug, t.webhook_secret_ref,
  t.target_kind, t.loop_workspace_id, t.loop_name, t.loop_inputs, t.loop_input_mapping,
	t.loop_network_participation, t.created_at, t.updated_at
FROM requested
JOIN automation_triggers AS t ON t.id = requested.trigger_id
LEFT JOIN automation_trigger_overlays AS o ON o.trigger_id = t.id
ORDER BY requested.ordinal;
