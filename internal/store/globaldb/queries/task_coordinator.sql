-- name: CompleteCoordinatorRun :execrows
UPDATE task_runs
SET status = sqlc.arg(status), lease_until = NULL, heartbeat_at = NULL, claim_token = NULL,
    ended_at = sqlc.arg(ended_at), error = NULL, result_json = sqlc.narg(result_json)
WHERE id = sqlc.arg(id) AND claim_token_hash = sqlc.arg(claim_token_hash);

-- name: SumLoopRunTaskTokens :one
SELECT CAST(COALESCE(SUM(tokens_used), 0) AS INTEGER)
FROM task_runs
WHERE loop_run_id = sqlc.arg(loop_run_id);

-- name: UpdateLoopRunTokens :execrows
UPDATE loop_runs SET tokens_used = sqlc.arg(tokens_used) WHERE id = sqlc.arg(id);

-- name: TransitionLoopCoordinatorBoundary :execrows
UPDATE loop_runs
SET status = sqlc.arg(to_status),
    pause_requested = 0,
    generation = CASE
      WHEN sqlc.arg(generation) > generation THEN sqlc.arg(generation)
      ELSE generation
    END,
    active_gate_id = CASE
      WHEN sqlc.arg(to_status) != CAST(sqlc.arg(needs_approval_status) AS TEXT) THEN ''
      ELSE active_gate_id
    END,
		active_human_criteria_json = CASE
      WHEN sqlc.arg(to_status) != CAST(sqlc.arg(needs_approval_status) AS TEXT) THEN '[]'
      ELSE active_human_criteria_json
		END,
	completion_state = CASE
		WHEN sqlc.arg(completion_partial) = 1 THEN 'partial'
		ELSE completion_state
	END,
	completed_at = NULLIF(CAST(sqlc.arg(completed_at) AS TEXT), '')
WHERE id = sqlc.arg(id) AND status = sqlc.arg(from_status);

-- name: UpdateLoopRunGeneration :execrows
UPDATE loop_runs
SET generation = CASE
  WHEN sqlc.arg(generation) > generation THEN sqlc.arg(generation)
  ELSE generation
END
WHERE id = sqlc.arg(id);

-- name: ConsumeLoopBudgetApproval :execrows
UPDATE loop_runs
SET budget_approval_seq = budget_approval_seq - 1
WHERE id = sqlc.arg(id) AND budget_approval_seq > 0;

-- name: ListLiveLoopTaskRunIDs :many
SELECT id
FROM task_runs
WHERE loop_run_id = sqlc.arg(loop_run_id)
  AND status IN (
    sqlc.arg(queued_status),
    sqlc.arg(claimed_status),
    sqlc.arg(starting_status),
    sqlc.arg(running_status),
    sqlc.arg(needs_attention_status)
  )
ORDER BY id;

-- name: ListOpenTaskDescendantIDs :many
WITH RECURSIVE descendants(id) AS (
  SELECT root.id
  FROM tasks AS root
  WHERE root.parent_task_id = sqlc.arg(root_task_id)
  UNION ALL
  SELECT task.id
  FROM tasks AS task
  JOIN descendants AS parent ON task.parent_task_id = parent.id
)
SELECT task.id
FROM tasks AS task
JOIN descendants ON descendants.id = task.id
WHERE task.status NOT IN (
  sqlc.arg(completed_status),
  sqlc.arg(failed_status),
  sqlc.arg(canceled_status)
)
ORDER BY task.id;
