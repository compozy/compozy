-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS task_states (
    component        text NOT NULL,
    status           text NOT NULL,
    task_exec_id     text NOT NULL PRIMARY KEY,
    task_id          text NOT NULL,
    workflow_exec_id text NOT NULL,
    workflow_id      text NOT NULL,
    agent_id         text,
    tool_id          text,
    action_id        text,
    input            jsonb,
    output           jsonb,
    error            jsonb,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),

    -- parent linkage
    CONSTRAINT fk_workflow
      FOREIGN KEY (workflow_exec_id)
      REFERENCES workflow_states (workflow_exec_id)
      ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_task_states_status ON task_states (status);
CREATE INDEX IF NOT EXISTS idx_task_states_workflow_id ON task_states (workflow_id);
CREATE INDEX IF NOT EXISTS idx_task_states_workflow_exec_id ON task_states (workflow_exec_id);
CREATE INDEX IF NOT EXISTS idx_task_states_task_id ON task_states (task_id);
CREATE INDEX IF NOT EXISTS idx_task_states_component ON task_states (component);
CREATE INDEX IF NOT EXISTS idx_task_states_agent_id ON task_states (agent_id);
CREATE INDEX IF NOT EXISTS idx_task_states_tool_id ON task_states (tool_id);
CREATE INDEX IF NOT EXISTS idx_task_states_action_id ON task_states (action_id);
CREATE INDEX IF NOT EXISTS idx_task_states_created_at ON task_states (created_at);
CREATE INDEX IF NOT EXISTS idx_task_states_updated_at ON task_states (updated_at);

CREATE INDEX IF NOT EXISTS idx_task_states_workflow_exec_id_task_id ON task_states (workflow_exec_id, task_id);
CREATE INDEX IF NOT EXISTS idx_task_states_workflow_exec_id_agent_id ON task_states (workflow_exec_id, agent_id);
CREATE INDEX IF NOT EXISTS idx_task_states_workflow_exec_id_tool_id ON task_states (workflow_exec_id, tool_id);
CREATE INDEX IF NOT EXISTS idx_task_states_workflow_id_task_id ON task_states (workflow_id, task_id);
CREATE INDEX IF NOT EXISTS idx_task_states_workflow_id_agent_id ON task_states (workflow_id, agent_id);
CREATE INDEX IF NOT EXISTS idx_task_states_workflow_id_tool_id ON task_states (workflow_id, tool_id);
CREATE INDEX IF NOT EXISTS idx_task_states_workflow_id_action_id ON task_states (workflow_id, action_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_task_states_workflow_id_tool_id;
DROP INDEX IF EXISTS idx_task_states_workflow_id_agent_id;
DROP INDEX IF EXISTS idx_task_states_workflow_id_task_id;
DROP INDEX IF EXISTS idx_task_states_workflow_exec_id_tool_id;
DROP INDEX IF EXISTS idx_task_states_workflow_exec_id_agent_id;
DROP INDEX IF EXISTS idx_task_states_workflow_exec_id_task_id;
DROP INDEX IF EXISTS idx_task_states_updated_at;
DROP INDEX IF EXISTS idx_task_states_created_at;
DROP INDEX IF EXISTS idx_task_states_component;
DROP INDEX IF EXISTS idx_task_states_tool_id;
DROP INDEX IF EXISTS idx_task_states_agent_id;
DROP INDEX IF EXISTS idx_task_states_task_id;
DROP INDEX IF EXISTS idx_task_states_workflow_exec_id;
DROP INDEX IF EXISTS idx_task_states_workflow_id;
DROP INDEX IF EXISTS idx_task_states_status;
DROP TABLE IF EXISTS task_states;
-- +goose StatementEnd
