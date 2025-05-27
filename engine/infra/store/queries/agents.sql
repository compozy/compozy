-- name: GetAgentExecutionByExecID :one
SELECT *
FROM executions
WHERE component_type = 'agent' AND agent_exec_id = :agent_exec_id
ORDER BY created_at DESC;

-- name: ListAgentExecutionsByStatus :many
SELECT *
FROM executions
WHERE component_type = 'agent' AND status = :status
ORDER BY created_at DESC;

-- name: ListAgentExecutionsByWorkflowID :many
SELECT *
FROM executions
WHERE component_type = 'agent' AND workflow_id = :workflow_id
ORDER BY created_at DESC;

-- name: ListAgentExecutionsByWorkflowExecID :many
SELECT *
FROM executions
WHERE component_type = 'agent' AND workflow_exec_id = :workflow_exec_id
ORDER BY created_at DESC;

-- name: ListAgentExecutionsByTaskID :many
SELECT *
FROM executions
WHERE component_type = 'agent' AND task_id = :task_id
ORDER BY created_at DESC;

-- name: ListAgentExecutionsByTaskExecID :many
SELECT *
FROM executions
WHERE component_type = 'agent' AND task_exec_id = :task_exec_id
ORDER BY created_at DESC;

-- name: ListAgentExecutionsByAgentID :many
SELECT *
FROM executions
WHERE component_type = 'agent' AND agent_id = :agent_id
ORDER BY created_at DESC;
