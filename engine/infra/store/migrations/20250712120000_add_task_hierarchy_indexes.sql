-- +goose Up
-- +goose StatementBegin
-- Add supporting indexes for the recursive CTE to improve performance
-- These indexes will reduce recursive traversal cost and join lookups

-- Index on workflow_exec_id for faster filtering in the CTE base case
CREATE INDEX IF NOT EXISTS idx_task_states_workflow_exec_id_parent_null 
ON task_states (workflow_exec_id) 
WHERE parent_state_id IS NULL;

-- Index on parent_state_id for faster joins in the recursive case
CREATE INDEX IF NOT EXISTS idx_task_states_parent_state_id 
ON task_states (parent_state_id);

-- Composite index for the recursive join condition
CREATE INDEX IF NOT EXISTS idx_task_states_parent_state_id_task_exec_id 
ON task_states (parent_state_id, task_exec_id);

-- Ensure task_exec_id has an index (should already exist as PK, but explicitly ensuring)
CREATE INDEX IF NOT EXISTS idx_task_states_task_exec_id 
ON task_states (task_exec_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_task_states_workflow_exec_id_parent_null;
DROP INDEX IF EXISTS idx_task_states_parent_state_id;
DROP INDEX IF EXISTS idx_task_states_parent_state_id_task_exec_id;
DROP INDEX IF EXISTS idx_task_states_task_exec_id;
-- +goose StatementEnd
