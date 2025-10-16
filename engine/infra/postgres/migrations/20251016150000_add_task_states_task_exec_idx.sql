-- +goose Up
-- +goose NO TRANSACTION
-- Ensure joins on task_states.task_exec_id remain performant for workflow usage summaries.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_task_states_task_exec_id ON task_states (task_exec_id);

-- +goose Down
-- +goose NO TRANSACTION
DROP INDEX CONCURRENTLY IF EXISTS idx_task_states_task_exec_id;
