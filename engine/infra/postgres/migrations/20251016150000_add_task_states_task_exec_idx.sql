-- +goose Up
-- +goose StatementBegin
-- Ensure joins on task_states.task_exec_id remain performant for workflow usage summaries.
CREATE INDEX IF NOT EXISTS idx_task_states_task_exec_id ON task_states (task_exec_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_task_states_task_exec_id;
-- +goose StatementEnd
