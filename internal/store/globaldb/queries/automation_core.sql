-- name: GetAutomationTriggerByWebhookID :one
SELECT id, scope, name, agent_name, workspace_id, prompt, event, filter,
       enabled, retry, fire_limit, source, webhook_id, endpoint_slug,
       webhook_secret_ref, target_kind, loop_workspace_id, loop_name,
	   loop_inputs, loop_input_mapping, loop_network_participation, created_at, updated_at
FROM automation_triggers WHERE webhook_id = sqlc.arg(webhook_id);

-- name: InsertAutomationRun :exec
INSERT INTO automation_runs (
  id, job_id, trigger_id, session_id, task_id, task_run_id, fire_id,
  status, attempt, scheduled_at, started_at, ended_at, error,
	delivery_error, delivery_error_at, loop_run_id, network_participation, metadata_json
) VALUES (
  sqlc.arg(id), sqlc.narg(job_id), sqlc.narg(trigger_id), sqlc.narg(session_id),
  sqlc.narg(task_id), sqlc.narg(task_run_id), sqlc.narg(fire_id), sqlc.arg(status),
  sqlc.arg(attempt), sqlc.narg(scheduled_at), sqlc.narg(started_at), sqlc.narg(ended_at),
  sqlc.narg(error), sqlc.narg(delivery_error), sqlc.narg(delivery_error_at),
	sqlc.narg(loop_run_id), sqlc.narg(network_participation), sqlc.arg(metadata_json)
);

-- name: UpdateAutomationRun :execrows
UPDATE automation_runs SET
  job_id = sqlc.narg(job_id), trigger_id = sqlc.narg(trigger_id),
  session_id = sqlc.narg(session_id), task_id = sqlc.narg(task_id),
  task_run_id = sqlc.narg(task_run_id), fire_id = sqlc.narg(fire_id),
  status = sqlc.arg(status), attempt = sqlc.arg(attempt),
  scheduled_at = sqlc.narg(scheduled_at), started_at = sqlc.narg(started_at),
  ended_at = sqlc.narg(ended_at), error = sqlc.narg(error),
  delivery_error = sqlc.narg(delivery_error), delivery_error_at = sqlc.narg(delivery_error_at),
	loop_run_id = sqlc.narg(loop_run_id), network_participation = sqlc.narg(network_participation),
	metadata_json = sqlc.arg(metadata_json)
WHERE id = sqlc.arg(id);

-- name: DeleteAutomationRun :execrows
DELETE FROM automation_runs WHERE id = sqlc.arg(id);

-- name: GetAutomationRun :one
SELECT id, job_id, trigger_id, session_id, task_id, task_run_id, fire_id,
       status, attempt, scheduled_at, started_at, ended_at, error,
	   delivery_error, delivery_error_at, loop_run_id, network_participation, metadata_json
FROM automation_runs WHERE id = sqlc.arg(id);

-- name: UpsertAutomationJobOverlay :exec
INSERT INTO automation_job_overlays (job_id, enabled_override, updated_at)
VALUES (sqlc.arg(job_id), sqlc.arg(enabled_override), sqlc.arg(updated_at))
ON CONFLICT(job_id) DO UPDATE SET
  enabled_override = excluded.enabled_override, updated_at = excluded.updated_at;

-- name: GetAutomationJobOverlay :one
SELECT job_id, enabled_override, updated_at
FROM automation_job_overlays WHERE job_id = sqlc.arg(job_id);

-- name: ListAutomationJobOverlays :many
SELECT job_id, enabled_override, updated_at
FROM automation_job_overlays ORDER BY job_id ASC;

-- name: DeleteAutomationJobOverlay :exec
DELETE FROM automation_job_overlays WHERE job_id = sqlc.arg(job_id);

-- name: UpsertAutomationTriggerOverlay :exec
INSERT INTO automation_trigger_overlays (trigger_id, enabled_override, updated_at)
VALUES (sqlc.arg(trigger_id), sqlc.arg(enabled_override), sqlc.arg(updated_at))
ON CONFLICT(trigger_id) DO UPDATE SET
  enabled_override = excluded.enabled_override, updated_at = excluded.updated_at;

-- name: GetAutomationTriggerOverlay :one
SELECT trigger_id, enabled_override, updated_at
FROM automation_trigger_overlays WHERE trigger_id = sqlc.arg(trigger_id);

-- name: ListAutomationTriggerOverlays :many
SELECT trigger_id, enabled_override, updated_at
FROM automation_trigger_overlays ORDER BY trigger_id ASC;

-- name: DeleteAutomationTriggerOverlay :exec
DELETE FROM automation_trigger_overlays WHERE trigger_id = sqlc.arg(trigger_id);

-- name: InsertAutomationJob :exec
INSERT INTO automation_jobs (
  id, scope, name, agent_name, workspace_id, prompt, schedule, task,
  enabled, retry, fire_limit, source, target_kind, loop_workspace_id,
	loop_name, loop_inputs, loop_input_mapping, loop_network_participation, created_at, updated_at
) VALUES (
  sqlc.arg(id), sqlc.arg(scope), sqlc.arg(name), sqlc.arg(agent_name),
  sqlc.narg(workspace_id), sqlc.arg(prompt), sqlc.narg(schedule), sqlc.narg(task),
  sqlc.arg(enabled), sqlc.arg(retry), sqlc.arg(fire_limit), sqlc.arg(source),
  sqlc.arg(target_kind), sqlc.narg(loop_workspace_id), sqlc.narg(loop_name),
	sqlc.arg(loop_inputs), sqlc.arg(loop_input_mapping), sqlc.narg(loop_network_participation),
	sqlc.arg(created_at), sqlc.arg(updated_at)
);

-- name: UpdateAutomationJob :execrows
UPDATE automation_jobs SET
  scope = sqlc.arg(scope), name = sqlc.arg(name), agent_name = sqlc.arg(agent_name),
  workspace_id = sqlc.narg(workspace_id), prompt = sqlc.arg(prompt),
  schedule = sqlc.narg(schedule), task = sqlc.narg(task), enabled = sqlc.arg(enabled),
  retry = sqlc.arg(retry), fire_limit = sqlc.arg(fire_limit), source = sqlc.arg(source),
  target_kind = sqlc.arg(target_kind), loop_workspace_id = sqlc.narg(loop_workspace_id),
  loop_name = sqlc.narg(loop_name), loop_inputs = sqlc.arg(loop_inputs),
	loop_input_mapping = sqlc.arg(loop_input_mapping),
	loop_network_participation = sqlc.narg(loop_network_participation), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);

-- name: DeleteAutomationJob :execrows
DELETE FROM automation_jobs WHERE id = sqlc.arg(id);

-- name: GetAutomationJob :one
SELECT id, scope, name, agent_name, workspace_id, prompt, schedule, task,
       enabled, retry, fire_limit, source, target_kind, loop_workspace_id,
	   loop_name, loop_inputs, loop_input_mapping, loop_network_participation, created_at, updated_at
FROM automation_jobs WHERE id = sqlc.arg(id);

-- name: InsertAutomationTrigger :exec
INSERT INTO automation_triggers (
  id, scope, name, agent_name, workspace_id, prompt, event, filter,
  enabled, retry, fire_limit, source, webhook_id, endpoint_slug, webhook_secret_ref,
  target_kind, loop_workspace_id, loop_name, loop_inputs, loop_input_mapping,
	loop_network_participation, created_at, updated_at
) VALUES (
  sqlc.arg(id), sqlc.arg(scope), sqlc.arg(name), sqlc.arg(agent_name),
  sqlc.narg(workspace_id), sqlc.arg(prompt), sqlc.arg(event), sqlc.arg(filter),
  sqlc.arg(enabled), sqlc.arg(retry), sqlc.arg(fire_limit), sqlc.arg(source),
  sqlc.narg(webhook_id), sqlc.narg(endpoint_slug), sqlc.narg(webhook_secret_ref),
  sqlc.arg(target_kind), sqlc.narg(loop_workspace_id), sqlc.narg(loop_name),
	sqlc.arg(loop_inputs), sqlc.arg(loop_input_mapping), sqlc.narg(loop_network_participation),
	sqlc.arg(created_at), sqlc.arg(updated_at)
);

-- name: UpdateAutomationTrigger :execrows
UPDATE automation_triggers SET
  scope = sqlc.arg(scope), name = sqlc.arg(name), agent_name = sqlc.arg(agent_name),
  workspace_id = sqlc.narg(workspace_id), prompt = sqlc.arg(prompt), event = sqlc.arg(event),
  filter = sqlc.arg(filter), enabled = sqlc.arg(enabled), retry = sqlc.arg(retry),
  fire_limit = sqlc.arg(fire_limit), source = sqlc.arg(source),
  webhook_id = sqlc.narg(webhook_id), endpoint_slug = sqlc.narg(endpoint_slug),
  webhook_secret_ref = sqlc.narg(webhook_secret_ref), target_kind = sqlc.arg(target_kind),
  loop_workspace_id = sqlc.narg(loop_workspace_id), loop_name = sqlc.narg(loop_name),
	loop_inputs = sqlc.arg(loop_inputs), loop_input_mapping = sqlc.arg(loop_input_mapping),
	loop_network_participation = sqlc.narg(loop_network_participation),
  updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);

-- name: DeleteAutomationTrigger :execrows
DELETE FROM automation_triggers WHERE id = sqlc.arg(id);

-- name: GetAutomationTrigger :one
SELECT id, scope, name, agent_name, workspace_id, prompt, event, filter,
       enabled, retry, fire_limit, source, webhook_id, endpoint_slug,
       webhook_secret_ref, target_kind, loop_workspace_id, loop_name,
	   loop_inputs, loop_input_mapping, loop_network_participation, created_at, updated_at
FROM automation_triggers WHERE id = sqlc.arg(id);
