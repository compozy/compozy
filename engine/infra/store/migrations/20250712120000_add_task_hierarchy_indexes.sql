-- +goose Up
-- +goose StatementBegin
-- Add supporting indexes for the recursive CTE to improve performance
-- These indexes will reduce recursive traversal cost and join lookups

-- Index on workflow_exec_id for faster filtering in the CTE base case
CREATE INDEX IF NOT EXISTS idx_task_states_workflow_exec_id_parent_null 
ON task_states (workflow_exec_id) 
WHERE parent_state_id IS NULL;

-- Composite index for the recursive join condition matching the CTE predicates
-- The recursive CTE joins on parent_state_id and filters by workflow_exec_id
CREATE INDEX IF NOT EXISTS idx_task_states_parent_state_id_workflow_exec_id 
ON task_states (parent_state_id, workflow_exec_id);


-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_task_states_workflow_exec_id_parent_null;
DROP INDEX IF EXISTS idx_task_states_parent_state_id_workflow_exec_id;
-- +goose StatementEnd
