-- name: InsertTask :exec
INSERT INTO tasks (
  id, identifier, scope, workspace_id, parent_task_id, title, description,
  priority, max_attempts, auto_enqueue_on_ready, status, approval_policy, approval_state,
  owner_kind, owner_ref, created_by_kind, created_by_ref, origin_kind, origin_ref,
  created_at, updated_at, closed_at, paused, paused_by, paused_at, paused_reason, wake_creator,
  metadata_json
) VALUES (
  sqlc.arg(id), sqlc.narg(identifier), sqlc.arg(scope), sqlc.narg(workspace_id),
  sqlc.narg(parent_task_id), sqlc.arg(title), sqlc.narg(description),
  sqlc.arg(priority), sqlc.arg(max_attempts), sqlc.arg(auto_enqueue_on_ready), sqlc.arg(status),
  sqlc.arg(approval_policy), sqlc.arg(approval_state), sqlc.narg(owner_kind), sqlc.narg(owner_ref),
  sqlc.arg(created_by_kind), sqlc.arg(created_by_ref), sqlc.arg(origin_kind), sqlc.arg(origin_ref),
  sqlc.arg(created_at), sqlc.arg(updated_at), sqlc.narg(closed_at), sqlc.arg(paused),
  sqlc.arg(paused_by), sqlc.narg(paused_at), sqlc.arg(paused_reason), sqlc.arg(wake_creator),
  sqlc.narg(metadata_json)
);

-- name: DeleteTask :execrows
DELETE FROM tasks WHERE id = sqlc.arg(id);

-- name: UpdateTask :execrows
UPDATE tasks
SET identifier = sqlc.narg(identifier),
    scope = sqlc.arg(scope),
    workspace_id = sqlc.narg(workspace_id),
    parent_task_id = sqlc.narg(parent_task_id),
    title = sqlc.arg(title),
    description = sqlc.narg(description),
    priority = sqlc.arg(priority),
    max_attempts = sqlc.arg(max_attempts),
    auto_enqueue_on_ready = sqlc.arg(auto_enqueue_on_ready),
    approval_policy = sqlc.arg(approval_policy),
    approval_state = sqlc.arg(approval_state),
    owner_kind = sqlc.narg(owner_kind),
    owner_ref = sqlc.narg(owner_ref),
    created_by_kind = sqlc.arg(created_by_kind),
    created_by_ref = sqlc.arg(created_by_ref),
    origin_kind = sqlc.arg(origin_kind),
    origin_ref = sqlc.arg(origin_ref),
    created_at = sqlc.arg(created_at),
    updated_at = sqlc.arg(updated_at),
    closed_at = sqlc.narg(closed_at),
    paused = sqlc.arg(paused),
    paused_by = sqlc.arg(paused_by),
    paused_at = sqlc.narg(paused_at),
    paused_reason = sqlc.arg(paused_reason),
    needs_attention_reason = sqlc.narg(needs_attention_reason),
    needs_attention_at = sqlc.narg(needs_attention_at),
    needs_attention_by_kind = sqlc.narg(needs_attention_by_kind),
    needs_attention_by_ref = sqlc.narg(needs_attention_by_ref),
    wake_creator = sqlc.arg(wake_creator),
    metadata_json = sqlc.narg(metadata_json)
WHERE id = sqlc.arg(id);

-- name: GetTask :one
SELECT
  id, identifier, scope, workspace_id, parent_task_id, title, description,
  priority, max_attempts, auto_enqueue_on_ready, status, approval_policy, approval_state,
  owner_kind, owner_ref, created_by_kind, created_by_ref, origin_kind, origin_ref,
  created_at, updated_at, closed_at, current_run_id,
  CAST(COALESCE((SELECT MAX(te.event_seq) FROM task_events te WHERE te.task_id = tasks.id), 0) AS INTEGER) AS latest_event_seq,
  paused, paused_by, paused_at, paused_reason, needs_attention_reason, needs_attention_at,
  needs_attention_by_kind, needs_attention_by_ref, wake_creator, metadata_json
FROM tasks
WHERE tasks.id = sqlc.arg(task_id);

-- name: CountDirectTaskChildren :one
SELECT COUNT(1) FROM tasks WHERE parent_task_id = sqlc.arg(parent_task_id);

-- name: GetWorkspaceID :one
SELECT id FROM workspaces WHERE workspaces.id = sqlc.arg(workspace_id);

-- name: GetTaskID :one
SELECT id FROM tasks WHERE tasks.id = sqlc.arg(task_id);

-- name: InsertTaskRun :exec
INSERT INTO task_runs (
  id, task_id, workspace_id, run_kind, loop_run_id, status, attempt, recovery_count, previous_run_id, failure_kind,
  claimed_by_kind, claimed_by_ref, session_id, origin_kind, origin_ref, idempotency_key,
  network_spec_json, network_mode, network_channel, network_source, designation_group_id,
  claim_token, claim_token_hash, lease_until, heartbeat_at, queued_at, claimed_at, started_at, ended_at,
  tokens_used, error, metadata_json, result_json, review_required, review_request_round,
  review_policy_snapshot, review_request_id, parent_run_id, review_id, review_round,
  continuation_reason, missing_work_json, next_round_guidance,
  network_wake_id, network_target_session_id, network_owner_key
) VALUES (
  sqlc.arg(id), sqlc.narg(task_id), sqlc.narg(workspace_id), sqlc.arg(run_kind), sqlc.narg(loop_run_id), sqlc.arg(status),
  sqlc.arg(attempt), sqlc.arg(recovery_count), sqlc.narg(previous_run_id), sqlc.arg(failure_kind), sqlc.narg(claimed_by_kind),
  sqlc.narg(claimed_by_ref), sqlc.narg(session_id), sqlc.arg(origin_kind), sqlc.arg(origin_ref),
  sqlc.narg(idempotency_key), sqlc.arg(network_spec_json), sqlc.arg(network_mode),
  sqlc.narg(network_channel), sqlc.arg(network_source), sqlc.arg(designation_group_id),
  NULL, sqlc.narg(claim_token_hash), sqlc.narg(lease_until), sqlc.narg(heartbeat_at),
  sqlc.arg(queued_at), sqlc.narg(claimed_at), sqlc.narg(started_at), sqlc.narg(ended_at),
  sqlc.arg(tokens_used), sqlc.narg(error),
  sqlc.narg(metadata_json), sqlc.narg(result_json), sqlc.arg(review_required),
  sqlc.arg(review_request_round), sqlc.arg(review_policy_snapshot), sqlc.narg(review_request_id),
  sqlc.narg(parent_run_id), sqlc.narg(review_id), sqlc.arg(review_round),
  sqlc.arg(continuation_reason), sqlc.arg(missing_work_json), sqlc.arg(next_round_guidance),
  sqlc.narg(network_wake_id), sqlc.narg(network_target_session_id), sqlc.narg(network_owner_key)
);

-- name: UpdateTaskRun :execrows
UPDATE task_runs
SET task_id = sqlc.narg(task_id),
    workspace_id = sqlc.narg(workspace_id),
    run_kind = sqlc.arg(run_kind),
    loop_run_id = sqlc.narg(loop_run_id),
    status = sqlc.arg(status),
    attempt = sqlc.arg(attempt),
    recovery_count = sqlc.arg(recovery_count),
    previous_run_id = sqlc.narg(previous_run_id),
    failure_kind = sqlc.arg(failure_kind),
    claimed_by_kind = sqlc.narg(claimed_by_kind),
    claimed_by_ref = sqlc.narg(claimed_by_ref),
    session_id = sqlc.narg(session_id),
    origin_kind = sqlc.arg(origin_kind),
    origin_ref = sqlc.arg(origin_ref),
    idempotency_key = sqlc.narg(idempotency_key),
    network_spec_json = sqlc.arg(network_spec_json),
    network_mode = sqlc.arg(network_mode),
    network_channel = sqlc.narg(network_channel),
    network_source = sqlc.arg(network_source),
    designation_group_id = sqlc.arg(designation_group_id),
    claim_token = CASE WHEN sqlc.narg(claim_token_hash) IS NOT NULL AND claim_token_hash = sqlc.narg(claim_token_hash) THEN claim_token ELSE NULL END,
    claim_token_hash = sqlc.narg(claim_token_hash),
    lease_until = sqlc.narg(lease_until),
    heartbeat_at = sqlc.narg(heartbeat_at),
    queued_at = sqlc.arg(queued_at),
    claimed_at = sqlc.narg(claimed_at),
    started_at = sqlc.narg(started_at),
    ended_at = sqlc.narg(ended_at),
    tokens_used = sqlc.arg(tokens_used),
    error = sqlc.narg(error),
    metadata_json = sqlc.narg(metadata_json),
    result_json = sqlc.narg(result_json),
    review_required = sqlc.arg(review_required),
    review_request_round = sqlc.arg(review_request_round),
    review_policy_snapshot = sqlc.arg(review_policy_snapshot),
    review_request_id = sqlc.narg(review_request_id),
    parent_run_id = sqlc.narg(parent_run_id),
    review_id = sqlc.narg(review_id),
    review_round = sqlc.arg(review_round),
    continuation_reason = sqlc.arg(continuation_reason),
    missing_work_json = sqlc.arg(missing_work_json),
    next_round_guidance = sqlc.arg(next_round_guidance),
    network_wake_id = sqlc.narg(network_wake_id),
    network_target_session_id = sqlc.narg(network_target_session_id),
    network_owner_key = sqlc.narg(network_owner_key)
WHERE id = sqlc.arg(id);

-- name: GetTaskRun :one
SELECT
  id, task_id, workspace_id, run_kind, loop_run_id, status, attempt, recovery_count, previous_run_id, failure_kind,
  claimed_by_kind, claimed_by_ref, session_id, origin_kind, origin_ref, idempotency_key,
  network_spec_json, network_mode, network_channel, network_source, designation_group_id,
  '' AS claim_token, claim_token_hash, lease_until, heartbeat_at, queued_at, claimed_at, started_at, ended_at,
  tokens_used, error, metadata_json, result_json, review_required, review_request_round,
  review_policy_snapshot, review_request_id, parent_run_id, review_id, review_round,
  continuation_reason, missing_work_json, next_round_guidance,
  network_wake_id, network_target_session_id, network_owner_key
FROM task_runs
WHERE id = sqlc.arg(id);

-- name: CountActiveTaskRunSessionBindings :one
SELECT COUNT(1)
FROM task_runs
WHERE session_id = sqlc.arg(session_id)
  AND status IN (sqlc.arg(claimed_status), sqlc.arg(starting_status), sqlc.arg(running_status));

-- name: ListTaskRunsByStatus :many
SELECT
  id, task_id, workspace_id, run_kind, loop_run_id, status, attempt, recovery_count, previous_run_id, failure_kind,
  claimed_by_kind, claimed_by_ref, session_id, origin_kind, origin_ref, idempotency_key,
  network_spec_json, network_mode, network_channel, network_source, designation_group_id,
  '' AS claim_token, claim_token_hash, lease_until, heartbeat_at, queued_at, claimed_at, started_at, ended_at,
  tokens_used, error, metadata_json, result_json, review_required, review_request_round,
  review_policy_snapshot, review_request_id, parent_run_id, review_id, review_round,
  continuation_reason, missing_work_json, next_round_guidance,
  network_wake_id, network_target_session_id, network_owner_key
FROM task_runs
WHERE status IN (sqlc.slice(statuses))
ORDER BY queued_at ASC, id ASC;

-- name: DeleteRequiredTaskRunCapabilities :exec
DELETE FROM task_run_required_capabilities WHERE run_id = sqlc.arg(run_id);

-- name: DeletePreferredTaskRunCapabilities :exec
DELETE FROM task_run_preferred_capabilities WHERE run_id = sqlc.arg(run_id);

-- name: InsertRequiredTaskRunCapability :exec
INSERT INTO task_run_required_capabilities (run_id, capability_id)
VALUES (sqlc.arg(run_id), sqlc.arg(capability_id));

-- name: InsertPreferredTaskRunCapability :exec
INSERT INTO task_run_preferred_capabilities (run_id, capability_id)
VALUES (sqlc.arg(run_id), sqlc.arg(capability_id));

-- name: ListRequiredTaskRunCapabilities :many
SELECT run_id, capability_id
FROM task_run_required_capabilities
WHERE run_id IN (sqlc.slice(run_ids))
ORDER BY run_id ASC, capability_id ASC;

-- name: ListPreferredTaskRunCapabilities :many
SELECT run_id, capability_id
FROM task_run_preferred_capabilities
WHERE run_id IN (sqlc.slice(run_ids))
ORDER BY run_id ASC, capability_id ASC;
