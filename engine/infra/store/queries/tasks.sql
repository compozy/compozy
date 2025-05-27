-- name: GetTaskExecutionByExecID :one
SELECT *
FROM executions
WHERE component_type = 'task' AND task_exec_id = :task_exec_id
ORDER BY created_at DESC;

-- name: ListTaskExecutionsByStatus :many
SELECT *
FROM executions
WHERE component_type = 'task' AND status = :status
ORDER BY created_at DESC;

-- name: ListTaskExecutionsByWorkflowID :many
SELECT *
FROM executions
WHERE component_type = 'task' AND workflow_id = :workflow_id
ORDER BY created_at DESC;

-- name: ListTaskExecutionsByWorkflowExecID :many
SELECT *
FROM executions
WHERE component_type = 'task' AND workflow_exec_id = :workflow_exec_id
ORDER BY created_at DESC;

-- name: ListTaskExecutionsByTaskID :many
SELECT *
FROM executions
WHERE component_type = 'task' AND task_id = :task_id
ORDER BY created_at DESC;
