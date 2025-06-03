-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS workflow_states (
    workflow_exec_id text NOT NULL PRIMARY KEY,
    workflow_id      text NOT NULL,
    status           text NOT NULL,
    input            jsonb,
    output           jsonb,
    error            jsonb,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_workflow_states_status ON workflow_states (status);
CREATE INDEX IF NOT EXISTS idx_workflow_states_workflow_id ON workflow_states (workflow_id);
CREATE INDEX IF NOT EXISTS idx_workflow_states_workflow_status ON workflow_states (workflow_id, status);
CREATE INDEX IF NOT EXISTS idx_workflow_states_created_at ON workflow_states (created_at);
CREATE INDEX IF NOT EXISTS idx_workflow_states_updated_at ON workflow_states (updated_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_workflow_states_updated_at;
DROP INDEX IF EXISTS idx_workflow_states_created_at;
DROP INDEX IF EXISTS idx_workflow_states_workflow_status;
DROP INDEX IF EXISTS idx_workflow_states_workflow_id;
DROP INDEX IF EXISTS idx_workflow_states_status;
DROP TABLE IF EXISTS workflow_states;
-- +goose StatementEnd
