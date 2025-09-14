-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS workflow_states (
    workflow_exec_id TEXT PRIMARY KEY NOT NULL,
    workflow_id      TEXT NOT NULL,
    status           TEXT NOT NULL,
    input            TEXT,
    output           TEXT,
    error            TEXT,
    created_at       DATETIME NOT NULL DEFAULT (CURRENT_TIMESTAMP),
    updated_at       DATETIME NOT NULL DEFAULT (CURRENT_TIMESTAMP)
);
CREATE INDEX IF NOT EXISTS idx_workflow_states_status ON workflow_states(status);
CREATE INDEX IF NOT EXISTS idx_workflow_states_workflow_id ON workflow_states(workflow_id);
CREATE INDEX IF NOT EXISTS idx_workflow_states_workflow_status ON workflow_states(workflow_id, status);
CREATE INDEX IF NOT EXISTS idx_workflow_states_created_at ON workflow_states(created_at);
CREATE INDEX IF NOT EXISTS idx_workflow_states_updated_at ON workflow_states(updated_at);

-- Trigger to auto-update updated_at
CREATE TRIGGER IF NOT EXISTS trg_workflow_states_updated_at
AFTER UPDATE ON workflow_states
FOR EACH ROW
WHEN NEW.updated_at = OLD.updated_at
BEGIN
    UPDATE workflow_states SET updated_at = CURRENT_TIMESTAMP WHERE workflow_exec_id = NEW.workflow_exec_id;
END;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_workflow_states_updated_at;
DROP INDEX IF EXISTS idx_workflow_states_updated_at;
DROP INDEX IF EXISTS idx_workflow_states_created_at;
DROP INDEX IF EXISTS idx_workflow_states_workflow_status;
DROP INDEX IF EXISTS idx_workflow_states_workflow_id;
DROP INDEX IF EXISTS idx_workflow_states_status;
DROP TABLE IF EXISTS workflow_states;
-- +goose StatementEnd
