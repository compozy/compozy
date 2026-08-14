-- name: AdmitTaskRunDirectExecution :execrows
UPDATE task_runs
SET status = 'claimed',
    claimed_by_kind = sqlc.arg(claimed_by_kind),
    claimed_by_ref = sqlc.arg(claimed_by_ref),
    claimed_at = sqlc.arg(claimed_at),
    claim_token = NULL,
    claim_token_hash = NULL,
    lease_until = NULL,
    heartbeat_at = NULL
WHERE id = sqlc.arg(id)
	AND COALESCE(task_id, '') = COALESCE(sqlc.narg(expected_task_id), '')
	AND COALESCE(workspace_id, '') = COALESCE(sqlc.narg(expected_workspace_id), '')
  AND status = 'queued'
  AND COALESCE(session_id, '') = COALESCE(sqlc.arg(expected_session_id), '')
  AND COALESCE(claim_token_hash, '') = COALESCE(sqlc.arg(expected_claim_token_hash), '')
  AND COALESCE(lease_until, '') = COALESCE(sqlc.arg(expected_lease_until), '');

-- name: TransitionTaskRunStarting :execrows
UPDATE task_runs
SET status = 'starting'
WHERE id = sqlc.arg(id)
	AND COALESCE(task_id, '') = COALESCE(sqlc.narg(expected_task_id), '')
	AND COALESCE(workspace_id, '') = COALESCE(sqlc.narg(expected_workspace_id), '')
  AND status = 'claimed'
  AND COALESCE(session_id, '') = COALESCE(sqlc.arg(expected_session_id), '')
  AND COALESCE(claim_token_hash, '') = COALESCE(sqlc.arg(expected_claim_token_hash), '')
  AND COALESCE(lease_until, '') = COALESCE(sqlc.arg(expected_lease_until), '');

-- name: BindTaskRunSession :execrows
UPDATE task_runs
SET status = 'starting',
    session_id = sqlc.arg(session_id),
    worktree_id = sqlc.narg(worktree_id)
WHERE task_runs.id = sqlc.arg(id)
	AND COALESCE(task_runs.task_id, '') = COALESCE(sqlc.narg(expected_task_id), '')
	AND COALESCE(task_runs.workspace_id, '') = COALESCE(sqlc.narg(expected_workspace_id), '')
  AND task_runs.status = sqlc.arg(expected_status)
  AND task_runs.status IN ('claimed', 'starting')
  AND COALESCE(task_runs.session_id, '') = COALESCE(sqlc.arg(expected_session_id), '')
  AND COALESCE(task_runs.claim_token_hash, '') = COALESCE(sqlc.arg(expected_claim_token_hash), '')
  AND COALESCE(task_runs.lease_until, '') = COALESCE(sqlc.arg(expected_lease_until), '')
  AND NOT EXISTS (
    SELECT 1
    FROM task_runs AS bound
    WHERE bound.id <> sqlc.arg(id)
      AND bound.session_id = sqlc.arg(session_id)
      AND bound.status IN ('claimed', 'starting', 'running')
  );

-- name: TransitionTaskRunRunning :execrows
UPDATE task_runs
SET status = 'running',
    started_at = sqlc.arg(started_at)
WHERE id = sqlc.arg(id)
	AND COALESCE(task_id, '') = COALESCE(sqlc.narg(expected_task_id), '')
	AND COALESCE(workspace_id, '') = COALESCE(sqlc.narg(expected_workspace_id), '')
  AND status = 'starting'
  AND COALESCE(session_id, '') = COALESCE(sqlc.arg(expected_session_id), '')
  AND COALESCE(claim_token_hash, '') = COALESCE(sqlc.arg(expected_claim_token_hash), '')
  AND COALESCE(lease_until, '') = COALESCE(sqlc.arg(expected_lease_until), '');

-- name: RecoverTaskRunByRequeue :execrows
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
  AND status = sqlc.arg(expected_status)
  AND status IN ('claimed', 'starting', 'running')
  AND COALESCE(session_id, '') = COALESCE(sqlc.arg(expected_session_id), '')
  AND COALESCE(claim_token_hash, '') = COALESCE(sqlc.arg(expected_claim_token_hash), '')
  AND COALESCE(lease_until, '') = COALESCE(sqlc.arg(expected_lease_until), '');

-- name: RecoverTaskRunByMarkRunning :execrows
UPDATE task_runs
SET status = 'running',
    started_at = sqlc.arg(started_at)
WHERE id = sqlc.arg(id)
	AND COALESCE(task_id, '') = COALESCE(sqlc.narg(expected_task_id), '')
	AND COALESCE(workspace_id, '') = COALESCE(sqlc.narg(expected_workspace_id), '')
  AND status = sqlc.arg(expected_status)
  AND status IN ('claimed', 'starting')
  AND COALESCE(session_id, '') = COALESCE(sqlc.arg(expected_session_id), '')
  AND COALESCE(claim_token_hash, '') = COALESCE(sqlc.arg(expected_claim_token_hash), '')
  AND COALESCE(lease_until, '') = COALESCE(sqlc.arg(expected_lease_until), '');
