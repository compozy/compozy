-- name: MarkTaskRunNeedsAttention :execrows
UPDATE task_runs
SET status = 'needs_attention', error = sqlc.narg(error), claim_token = NULL
WHERE id = sqlc.arg(id)
	AND COALESCE(task_id, '') = COALESCE(sqlc.narg(expected_task_id), '')
	AND COALESCE(workspace_id, '') = COALESCE(sqlc.narg(expected_workspace_id), '')
  AND status = sqlc.arg(expected_status)
  AND COALESCE(session_id, '') = COALESCE(sqlc.arg(expected_session_id), '')
  AND COALESCE(claim_token_hash, '') = COALESCE(sqlc.arg(expected_claim_token_hash), '')
  AND COALESCE(lease_until, '') = COALESCE(sqlc.arg(expected_lease_until), '');

-- name: ForceReleaseClaimedTaskRun :execrows
UPDATE task_runs
SET status = 'queued',
    failure_kind = '',
    claimed_by_kind = NULL,
    claimed_by_ref = NULL,
    session_id = NULL,
    claim_token = NULL,
    claim_token_hash = NULL,
    lease_until = NULL,
    heartbeat_at = NULL,
    claimed_at = NULL,
    started_at = NULL,
    ended_at = NULL,
    error = NULL,
    result_json = NULL
WHERE id = sqlc.arg(id)
	AND COALESCE(task_id, '') = COALESCE(sqlc.narg(expected_task_id), '')
	AND COALESCE(workspace_id, '') = COALESCE(sqlc.narg(expected_workspace_id), '')
  AND status = 'claimed'
  AND COALESCE(session_id, '') = COALESCE(sqlc.arg(expected_session_id), '')
  AND COALESCE(claim_token_hash, '') = COALESCE(sqlc.arg(expected_claim_token_hash), '')
  AND COALESCE(lease_until, '') = COALESCE(sqlc.arg(expected_lease_until), '');

-- name: ForceFailEligibleTaskRun :execrows
UPDATE task_runs
SET status = 'failed',
    failure_kind = 'operator_forced',
    claim_token = NULL,
    claim_token_hash = NULL,
    lease_until = NULL,
    heartbeat_at = NULL,
    ended_at = sqlc.narg(ended_at),
    error = sqlc.narg(error),
    result_json = NULL
WHERE id = sqlc.arg(id)
	AND COALESCE(task_id, '') = COALESCE(sqlc.narg(expected_task_id), '')
	AND COALESCE(workspace_id, '') = COALESCE(sqlc.narg(expected_workspace_id), '')
  AND status = sqlc.arg(expected_status)
  AND status IN ('queued', 'claimed')
  AND COALESCE(session_id, '') = COALESCE(sqlc.arg(expected_session_id), '')
  AND COALESCE(claim_token_hash, '') = COALESCE(sqlc.arg(expected_claim_token_hash), '')
  AND COALESCE(lease_until, '') = COALESCE(sqlc.arg(expected_lease_until), '');

-- name: FailNeedsAttentionTaskRunForRecovery :execrows
UPDATE task_runs
SET status = 'failed',
    failure_kind = 'operator_forced',
    claim_token = NULL,
    claim_token_hash = NULL,
    lease_until = NULL,
    heartbeat_at = NULL,
    ended_at = sqlc.narg(ended_at),
    error = sqlc.narg(error),
    result_json = NULL
WHERE id = sqlc.arg(id)
	AND COALESCE(task_id, '') = COALESCE(sqlc.narg(expected_task_id), '')
	AND COALESCE(workspace_id, '') = COALESCE(sqlc.narg(expected_workspace_id), '')
  AND status = 'needs_attention'
  AND COALESCE(session_id, '') = COALESCE(sqlc.arg(expected_session_id), '')
  AND COALESCE(claim_token_hash, '') = COALESCE(sqlc.arg(expected_claim_token_hash), '')
  AND COALESCE(lease_until, '') = COALESCE(sqlc.arg(expected_lease_until), '');

-- name: CompleteParentRollupTaskRun :execrows
UPDATE task_runs
SET status = 'completed',
    result_json = sqlc.narg(result_json),
    error = NULL,
    ended_at = sqlc.narg(ended_at),
    lease_until = NULL,
    heartbeat_at = NULL
WHERE id = sqlc.arg(id)
	AND COALESCE(task_id, '') = COALESCE(sqlc.narg(expected_task_id), '')
	AND COALESCE(workspace_id, '') = COALESCE(sqlc.narg(expected_workspace_id), '')
  AND status = 'needs_attention'
  AND COALESCE(session_id, '') = COALESCE(sqlc.arg(expected_session_id), '')
  AND COALESCE(claim_token_hash, '') = COALESCE(sqlc.arg(expected_claim_token_hash), '')
  AND COALESCE(lease_until, '') = COALESCE(sqlc.arg(expected_lease_until), '');

-- name: ListTaskRunIDsForTask :many
SELECT id FROM task_runs WHERE task_id = sqlc.arg(task_id);

-- name: GetRetryChildRunID :one
SELECT id
FROM task_runs
WHERE previous_run_id = sqlc.arg(previous_run_id)
ORDER BY queued_at DESC, id DESC
LIMIT 1;
