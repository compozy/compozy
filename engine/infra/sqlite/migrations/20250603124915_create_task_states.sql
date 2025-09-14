-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS task_states (
    component        TEXT NOT NULL,
    status           TEXT NOT NULL,
    task_exec_id     TEXT PRIMARY KEY NOT NULL,
    task_id          TEXT NOT NULL,
    workflow_exec_id TEXT NOT NULL,
    workflow_id      TEXT NOT NULL,
    execution_type   TEXT NOT NULL DEFAULT 'basic',
    agent_id         TEXT,
    tool_id          TEXT,
    action_id        TEXT,
    parent_state_id  TEXT,
    input            TEXT,
    output           TEXT,
    error            TEXT,
    created_at       DATETIME NOT NULL DEFAULT (CURRENT_TIMESTAMP),
    updated_at       DATETIME NOT NULL DEFAULT (CURRENT_TIMESTAMP),
    FOREIGN KEY (workflow_exec_id) REFERENCES workflow_states(workflow_exec_id) ON DELETE CASCADE,
    FOREIGN KEY (parent_state_id)  REFERENCES task_states(task_exec_id)       ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_task_states_status ON task_states(status);
CREATE INDEX IF NOT EXISTS idx_task_states_workflow_id ON task_states(workflow_id);
CREATE INDEX IF NOT EXISTS idx_task_states_workflow_exec_id ON task_states(workflow_exec_id);
CREATE INDEX IF NOT EXISTS idx_task_states_task_id ON task_states(task_id);
CREATE INDEX IF NOT EXISTS idx_task_states_component ON task_states(component);
CREATE INDEX IF NOT EXISTS idx_task_states_agent_id ON task_states(agent_id);
CREATE INDEX IF NOT EXISTS idx_task_states_tool_id ON task_states(tool_id);
CREATE INDEX IF NOT EXISTS idx_task_states_action_id ON task_states(action_id);
CREATE INDEX IF NOT EXISTS idx_task_states_created_at ON task_states(created_at);
CREATE INDEX IF NOT EXISTS idx_task_states_updated_at ON task_states(updated_at);
CREATE INDEX IF NOT EXISTS idx_task_states_execution_type ON task_states(execution_type);
CREATE INDEX IF NOT EXISTS idx_task_states_parent_id ON task_states(parent_state_id);
CREATE INDEX IF NOT EXISTS idx_task_states_workflow_exec_id_task_id ON task_states(workflow_exec_id, task_id);

-- Trigger to auto-update updated_at
CREATE TRIGGER IF NOT EXISTS trg_task_states_updated_at
AFTER UPDATE ON task_states
FOR EACH ROW
WHEN NEW.updated_at = OLD.updated_at
BEGIN
    UPDATE task_states SET updated_at = CURRENT_TIMESTAMP WHERE task_exec_id = NEW.task_exec_id;
END;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_task_states_updated_at;
DROP INDEX IF EXISTS idx_task_states_workflow_exec_id_task_id;
DROP INDEX IF EXISTS idx_task_states_parent_id;
DROP INDEX IF EXISTS idx_task_states_execution_type;
DROP INDEX IF EXISTS idx_task_states_updated_at;
DROP INDEX IF EXISTS idx_task_states_created_at;
DROP INDEX IF EXISTS idx_task_states_action_id;
DROP INDEX IF EXISTS idx_task_states_tool_id;
DROP INDEX IF EXISTS idx_task_states_agent_id;
DROP INDEX IF EXISTS idx_task_states_component;
DROP INDEX IF EXISTS idx_task_states_task_id;
DROP INDEX IF EXISTS idx_task_states_workflow_exec_id;
DROP INDEX IF EXISTS idx_task_states_workflow_id;
DROP INDEX IF EXISTS idx_task_states_status;
DROP TABLE IF EXISTS task_states;
-- +goose StatementEnd
