-- name: UpsertExecution :exec
INSERT INTO executions (
    component_type,
    workflow_id,
    workflow_exec_id,
    task_id,
    task_exec_id,
    agent_id,
    agent_exec_id,
    tool_id,
    tool_exec_id,
    key,
    status,
    data,
    updated_at
) VALUES (
    :component_type,
    :workflow_id,
    :workflow_exec_id,
    :task_id,
    :task_exec_id,
    :agent_id,
    :agent_exec_id,
    :tool_id,
    :tool_exec_id,
    :key,
    :status,
    :data,
    CURRENT_TIMESTAMP
) ON CONFLICT(key) DO UPDATE SET
    status = excluded.status,
    data = excluded.data,
    updated_at = CURRENT_TIMESTAMP;

-- name: DeleteExecution :exec
DELETE FROM executions WHERE key = :key;

-- name: GetExecutionByKey :one
SELECT *
FROM executions
WHERE key = :key;
