-- name: CreatePendingToolApproval :one
INSERT INTO tool_approval_pending (
  profile_id,
  approval_id, workspace_id, invocation_id, target_kind, tool_id, target_json,
  command_id, args_json, approval_status, requested_at, expires_at
) VALUES (
  sqlc.arg(profile_id),
  sqlc.arg(approval_id), sqlc.arg(workspace_id), sqlc.arg(invocation_id),
  sqlc.arg(target_kind), sqlc.narg(tool_id), sqlc.arg(target_json),
  sqlc.narg(command_id), sqlc.arg(args_json), 'pending',
  sqlc.arg(requested_at), sqlc.arg(expires_at)
)
RETURNING *;

-- name: GetPendingToolApproval :one
SELECT *
FROM tool_approval_pending
WHERE approval_id = sqlc.arg(approval_id);

-- name: ResolvePendingToolApproval :one
UPDATE tool_approval_pending
SET approval_status = sqlc.arg(approval_status),
    resolved_at = sqlc.arg(resolved_at),
    execution_status = CASE
      WHEN sqlc.arg(approval_status) = 'approved' THEN 'dispatching'
      ELSE execution_status
    END,
    resume_fence = CASE
      WHEN sqlc.arg(approval_status) = 'approved' THEN 1
      ELSE resume_fence
    END
WHERE approval_id = sqlc.arg(approval_id)
  AND approval_status = 'pending'
RETURNING *;

-- name: CompleteToolApprovalExecution :one
UPDATE tool_approval_pending
SET execution_status = sqlc.arg(execution_status),
    result_json = sqlc.narg(result_json),
    error_json = sqlc.narg(error_json),
    executed_at = sqlc.arg(executed_at)
WHERE approval_id = sqlc.arg(approval_id)
  AND approval_status = 'approved'
  AND execution_status = 'dispatching'
  AND resume_fence = 1
RETURNING *;

-- name: ExpirePendingToolApprovals :many
UPDATE tool_approval_pending
SET approval_status = 'timeout', resolved_at = sqlc.arg(resolved_at)
WHERE approval_status = 'pending'
  AND expires_at <= sqlc.arg(resolved_at)
RETURNING *;

-- name: RecoverDispatchingToolApprovals :many
UPDATE tool_approval_pending
SET execution_status = 'uncertain', executed_at = sqlc.arg(executed_at)
WHERE approval_status = 'approved'
  AND execution_status = 'dispatching'
  AND resume_fence = 1
RETURNING *;

-- name: ListPendingToolApprovals :many
SELECT *
FROM tool_approval_pending
WHERE approval_status = 'pending'
ORDER BY expires_at, approval_id;
