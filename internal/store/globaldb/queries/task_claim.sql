-- name: HeartbeatTaskRunLease :execrows
UPDATE task_runs
SET lease_until = sqlc.arg(lease_until),
    heartbeat_at = sqlc.arg(heartbeat_at),
    claim_token = sqlc.arg(claim_token),
    tokens_used = CASE WHEN sqlc.arg(tokens_used) > tokens_used THEN sqlc.arg(tokens_used) ELSE tokens_used END
WHERE id = sqlc.arg(id)
  AND claim_token_hash = sqlc.arg(claim_token_hash)
  AND status IN (sqlc.arg(claimed_status), sqlc.arg(starting_status), sqlc.arg(running_status));

-- name: GetLoopRunWorkspaceID :one
SELECT workspace_id FROM loop_runs WHERE id = sqlc.arg(id);

-- name: CompleteTaskRunLease :execrows
UPDATE task_runs
SET status = sqlc.arg(status), lease_until = NULL, heartbeat_at = NULL, claim_token = NULL,
    ended_at = sqlc.arg(ended_at), tokens_used = sqlc.arg(tokens_used), error = NULL,
    result_json = sqlc.narg(result_json)
WHERE id = sqlc.arg(id) AND claim_token_hash = sqlc.arg(claim_token_hash);

-- name: CompletionCreatedTaskClaimExists :one
SELECT EXISTS(
  SELECT 1 FROM tasks
  WHERE id = sqlc.arg(id)
    AND COALESCE(workspace_id, '') = sqlc.arg(workspace_id)
    AND created_by_kind = sqlc.arg(created_by_kind)
    AND created_by_ref = sqlc.arg(created_by_ref)
);

-- name: UpdateLoopRunNodeTerminal :execrows
UPDATE loop_runs
SET last_progress_at = CASE
      WHEN last_progress_at < CAST(sqlc.arg(terminal_at) AS TEXT) THEN CAST(sqlc.arg(terminal_at) AS TEXT)
      ELSE last_progress_at
    END
WHERE id = sqlc.arg(id);

-- name: UpdateLoopGenerationOutputForTaskRun :execrows
UPDATE loop_generation_outputs
SET status = sqlc.arg(status),
    output_ref = CASE
      WHEN CAST(sqlc.arg(output_ref) AS TEXT) = '' THEN output_ref
      ELSE CAST(sqlc.arg(output_ref) AS TEXT)
    END,
    task_run_id = sqlc.arg(task_run_id)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND task_run_id = sqlc.arg(task_run_id);

-- name: UpsertLoopGenerationOutputForTaskRun :exec
INSERT INTO loop_generation_outputs (
  loop_run_id, generation, node_id, item_index, status, output_ref, task_run_id
) VALUES (
  sqlc.arg(loop_run_id), sqlc.arg(generation), sqlc.arg(node_id), sqlc.arg(item_index),
  sqlc.arg(status), sqlc.narg(output_ref), sqlc.arg(task_run_id)
)
ON CONFLICT(loop_run_id, generation, node_id, item_index) DO UPDATE SET
  status = excluded.status,
  output_ref = excluded.output_ref,
  task_run_id = excluded.task_run_id;

-- name: FailTaskRunLease :execrows
UPDATE task_runs
SET status = sqlc.arg(status), lease_until = NULL, heartbeat_at = NULL, claim_token = NULL,
    ended_at = sqlc.arg(ended_at), tokens_used = sqlc.arg(tokens_used), error = sqlc.arg(error),
    result_json = NULL
WHERE id = sqlc.arg(id) AND claim_token_hash = sqlc.arg(claim_token_hash);

-- name: ListAutonomyLeaseHandles :many
SELECT tr.id, tr.task_id, tr.run_kind, COALESCE(tr.workspace_id, t.workspace_id, '') AS workspace_id,
       tr.network_spec_json, tr.network_mode, tr.network_channel, tr.network_source,
       COALESCE(tr.network_target_session_id, '') AS target_session_id,
       COALESCE(tr.network_owner_key, '') AS owner_key, tr.status,
       COALESCE(tr.session_id, '') AS session_id, tr.claimed_by_kind, tr.claimed_by_ref,
       COALESCE(tr.claim_token, '') AS claim_token, COALESCE(tr.claim_token_hash, '') AS claim_token_hash,
       tr.lease_until, tr.heartbeat_at
FROM task_runs tr
LEFT JOIN tasks t ON t.id = tr.task_id
WHERE tr.session_id = sqlc.arg(session_id)
  AND COALESCE(tr.claim_token_hash, '') <> ''
  AND tr.run_kind <> 'network_wake'
ORDER BY COALESCE(tr.lease_until, '') DESC, tr.id ASC;

-- name: RequeueTaskRunLease :execrows
UPDATE task_runs
SET status = sqlc.arg(status), claimed_by_kind = NULL, claimed_by_ref = NULL, session_id = NULL,
    claim_token = NULL, claim_token_hash = NULL, lease_until = NULL, heartbeat_at = NULL,
    claimed_at = NULL, started_at = NULL, ended_at = NULL, error = NULL, result_json = NULL
WHERE id = sqlc.arg(id);

-- name: ListExpiredTaskRunLeaseIDs :many
SELECT id
FROM task_runs
WHERE status IN (sqlc.arg(claimed_status), sqlc.arg(starting_status), sqlc.arg(running_status))
  AND lease_until IS NOT NULL
  AND lease_until <= sqlc.arg(now)
ORDER BY lease_until ASC, id ASC
LIMIT sqlc.arg(result_limit);

-- name: RequeueExpiredTaskRunLease :execrows
UPDATE task_runs
SET status = sqlc.arg(queued_status), claimed_by_kind = NULL, claimed_by_ref = NULL, session_id = NULL,
    claim_token = NULL, claim_token_hash = NULL, lease_until = NULL, heartbeat_at = NULL,
    claimed_at = NULL, started_at = NULL, ended_at = NULL, error = NULL, result_json = NULL,
    recovery_count = recovery_count + sqlc.arg(recovery_increment)
WHERE id = sqlc.arg(id)
  AND status = sqlc.arg(previous_status)
  AND COALESCE(session_id, '') = sqlc.arg(session_id)
  AND claim_token_hash = sqlc.arg(claim_token_hash)
  AND lease_until = sqlc.arg(lease_until);

-- name: ExhaustExpiredTaskRunLease :execrows
UPDATE task_runs
SET status = sqlc.arg(needs_attention_status), claimed_by_kind = NULL, claimed_by_ref = NULL,
    session_id = NULL, claim_token = NULL, claim_token_hash = NULL, lease_until = NULL,
    heartbeat_at = NULL, ended_at = sqlc.arg(ended_at), error = sqlc.arg(error), result_json = NULL
WHERE id = sqlc.arg(id)
  AND status = sqlc.arg(previous_status)
  AND COALESCE(session_id, '') = sqlc.arg(session_id)
  AND claim_token_hash = sqlc.arg(claim_token_hash)
  AND lease_until = sqlc.arg(lease_until);

-- name: ClaimSelectedTaskRun :execrows
UPDATE task_runs
SET status = sqlc.arg(claimed_status),
    claimed_by_kind = sqlc.narg(claimed_by_kind),
    claimed_by_ref = sqlc.narg(claimed_by_ref),
    session_id = sqlc.narg(session_id),
    claim_token = sqlc.narg(claim_token),
    claim_token_hash = sqlc.narg(claim_token_hash),
    lease_until = sqlc.narg(lease_until),
    heartbeat_at = sqlc.narg(heartbeat_at),
    claimed_at = sqlc.narg(claimed_at),
    started_at = NULL,
    ended_at = NULL,
    error = NULL,
    metadata_json = sqlc.narg(metadata_json),
    result_json = NULL
WHERE id = sqlc.arg(id) AND status = sqlc.arg(queued_status);

-- name: GetTaskRunMetadataForClaim :one
SELECT metadata_json FROM task_runs WHERE id = sqlc.arg(id);

-- name: CountActiveTaskRunLeasesForSession :one
SELECT COUNT(1)
FROM task_runs
WHERE session_id = sqlc.arg(session_id)
  AND status IN (sqlc.arg(claimed_status), sqlc.arg(starting_status), sqlc.arg(running_status))
  AND (lease_until IS NULL OR lease_until > sqlc.arg(now));

-- name: CountActiveTaskRunLeasesForWorkspace :one
SELECT COUNT(1)
FROM task_runs
WHERE workspace_id = sqlc.arg(workspace_id)
  AND run_kind IN ('worker', 'coordinator')
  AND status IN (sqlc.arg(claimed_status), sqlc.arg(starting_status), sqlc.arg(running_status))
  AND (lease_until IS NULL OR lease_until > sqlc.arg(now));
