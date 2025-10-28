-- +goose Up
-- +goose StatementBegin
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS task_states (
    component        TEXT NOT NULL,
    status           TEXT NOT NULL,
    task_exec_id     TEXT NOT NULL PRIMARY KEY,
    task_id          TEXT NOT NULL,
    workflow_exec_id TEXT NOT NULL,
    workflow_id      TEXT NOT NULL,
    execution_type   TEXT NOT NULL DEFAULT 'basic',
    usage            TEXT,
    agent_id         TEXT,
    tool_id          TEXT,
    action_id        TEXT,
    parent_state_id  TEXT,
    input            TEXT,
    output           TEXT,
    error            TEXT,
    created_at       TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at       TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (workflow_exec_id)
        REFERENCES workflow_states (workflow_exec_id)
        ON DELETE CASCADE,
    FOREIGN KEY (parent_state_id)
        REFERENCES task_states (task_exec_id)
        ON DELETE CASCADE,
    CHECK (execution_type IN ('basic','router','parallel','collection','composite')),
    CHECK (
        (execution_type = 'basic' AND (
            (agent_id IS NOT NULL AND action_id IS NOT NULL AND tool_id IS NULL) OR
            (tool_id IS NOT NULL AND agent_id IS NULL AND action_id IS NULL) OR
            (agent_id IS NULL AND action_id IS NULL AND tool_id IS NULL)
        )) OR
        (execution_type = 'router' AND agent_id IS NULL AND action_id IS NULL AND tool_id IS NULL) OR
        (execution_type IN ('parallel', 'collection', 'composite'))
    ),
    CHECK (usage IS NULL OR json_type(usage) = 'array')
);

CREATE INDEX IF NOT EXISTS idx_task_states_status ON task_states (status);
CREATE INDEX IF NOT EXISTS idx_task_states_workflow_id ON task_states (workflow_id);
CREATE INDEX IF NOT EXISTS idx_task_states_workflow_exec_id ON task_states (workflow_exec_id);
CREATE INDEX IF NOT EXISTS idx_task_states_task_id ON task_states (task_id);
CREATE INDEX IF NOT EXISTS idx_task_states_parent_state_id ON task_states (parent_state_id);
-- Keep updated_at in sync on row updates.
CREATE TRIGGER IF NOT EXISTS trg_task_states_updated_at
AFTER UPDATE OF component, status, task_id, workflow_exec_id, workflow_id,
  execution_type, usage, agent_id, tool_id, action_id, parent_state_id, input, output, error
ON task_states
FOR EACH ROW
BEGIN
  UPDATE task_states
  SET updated_at = datetime('now')
  WHERE task_exec_id = NEW.task_exec_id;
END;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_task_states_updated_at;
DROP TABLE IF EXISTS task_states;
-- +goose StatementEnd
