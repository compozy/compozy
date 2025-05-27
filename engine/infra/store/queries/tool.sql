-- name: GetToolExecutionByExecID :one
SELECT *
FROM executions
WHERE component_type = 'tool' AND tool_exec_id = :tool_exec_id
ORDER BY created_at DESC;

-- name: ListToolExecutionsByStatus :many
SELECT *
FROM executions
WHERE component_type = 'tool' AND status = :status
ORDER BY created_at DESC;

-- name: ListToolExecutionsByWorkflowID :many
SELECT *
FROM executions
WHERE component_type = 'tool' AND workflow_id = :workflow_id
ORDER BY created_at DESC;

-- name: ListToolExecutionsByWorkflowExecID :many
SELECT *
FROM executions
WHERE component_type = 'tool' AND workflow_exec_id = :workflow_exec_id
ORDER BY created_at DESC;

-- name: ListToolExecutionsByTaskID :many
SELECT *
FROM executions
WHERE component_type = 'tool' AND task_id = :task_id
ORDER BY created_at DESC;

-- name: ListToolExecutionsByTaskExecID :many
SELECT *
FROM executions
WHERE component_type = 'tool' AND task_exec_id = :task_exec_id
ORDER BY created_at DESC;

-- name: ListToolExecutionsByToolID :many
SELECT *
FROM executions
WHERE component_type = 'tool' AND tool_id = :tool_id
ORDER BY created_at DESC;
