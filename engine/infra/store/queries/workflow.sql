-- name: GetWorkflowExecutionByExecID :one
SELECT *
FROM executions
WHERE component_type = 'workflow' AND workflow_exec_id = :workflow_exec_id
ORDER BY created_at DESC;

-- name: ListWorkflowExecutions :many
SELECT *
FROM executions
WHERE component_type = 'workflow'
ORDER BY created_at DESC;

-- name: ListWorkflowExecutionsByWorkflowID :many
SELECT *
FROM executions
WHERE component_type = 'workflow' AND workflow_id = :workflow_id
ORDER BY created_at DESC;

-- name: ListWorkflowExecutionsByStatus :many
SELECT *
FROM executions
WHERE component_type = 'workflow' AND status = :status
ORDER BY created_at DESC;

-- name: ListWorkflowChildrenExecutionsByWorkflowExecID :many
SELECT *
FROM executions
WHERE component_type != 'workflow' AND workflow_exec_id = :workflow_exec_id
ORDER BY created_at DESC;

-- name: ListWorkflowChildrenExecutionsByWorkflowID :many
SELECT *
FROM executions
WHERE component_type != 'workflow' AND workflow_id = :workflow_id
ORDER BY created_at DESC;
