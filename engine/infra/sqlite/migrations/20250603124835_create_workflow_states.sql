-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS workflow_states (
    workflow_exec_id TEXT NOT NULL PRIMARY KEY,
    workflow_id      TEXT NOT NULL,
    status           TEXT NOT NULL,
    usage            TEXT,
    input            TEXT,
    output           TEXT,
    error            TEXT,
    created_at       TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at       TEXT NOT NULL DEFAULT (datetime('now')),
    CHECK (usage IS NULL OR json_type(usage) = 'array')
);

CREATE INDEX IF NOT EXISTS idx_workflow_states_status ON workflow_states (status);
CREATE INDEX IF NOT EXISTS idx_workflow_states_workflow_status ON workflow_states (workflow_id, status);
CREATE INDEX IF NOT EXISTS idx_workflow_states_created_at ON workflow_states (created_at);
CREATE INDEX IF NOT EXISTS idx_workflow_states_updated_at ON workflow_states (updated_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS workflow_states;
-- +goose StatementEnd
