-- +goose Up
-- +goose StatementBegin
-- Ensure query performance for high-frequency task state lookups used by repository queries.
CREATE INDEX IF NOT EXISTS idx_task_states_workflow_exec_id_status ON task_states (workflow_exec_id, status);
CREATE INDEX IF NOT EXISTS idx_task_states_parent_state_output_not_null ON task_states (parent_state_id) WHERE output IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_task_states_parent_state_task_created_desc ON task_states (parent_state_id, task_id, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_task_states_parent_state_task_created_desc;
DROP INDEX IF EXISTS idx_task_states_parent_state_output_not_null;
DROP INDEX IF EXISTS idx_task_states_workflow_exec_id_status;
-- +goose StatementEnd
