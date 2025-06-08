-- Updated: 20250603124915_create_task_states.sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS task_states (
    component        text NOT NULL,
    status           text NOT NULL,
    task_exec_id     text NOT NULL PRIMARY KEY,
    task_id          text NOT NULL,
    workflow_exec_id text NOT NULL,
    workflow_id      text NOT NULL,
    execution_type   text NOT NULL DEFAULT 'basic',
    agent_id         text,
    tool_id          text,
    action_id        text,
    input            jsonb,
    output           jsonb,
    error            jsonb,
    parallel_state   jsonb,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),

    -- parent linkage
    CONSTRAINT fk_workflow
      FOREIGN KEY (workflow_exec_id)
      REFERENCES workflow_states (workflow_exec_id)
      ON DELETE CASCADE,

    -- execution type consistency constraint
    CONSTRAINT chk_execution_type_consistency
    CHECK (
        (execution_type = 'basic' AND (
            (agent_id IS NOT NULL AND action_id IS NOT NULL AND tool_id IS NULL AND parallel_state IS NULL) OR
            (tool_id IS NOT NULL AND agent_id IS NULL AND action_id IS NULL AND parallel_state IS NULL) OR
            (agent_id IS NULL AND action_id IS NULL AND tool_id IS NULL AND parallel_state IS NULL)
        )) OR
        (execution_type = 'router' AND agent_id IS NULL AND action_id IS NULL AND tool_id IS NULL AND parallel_state IS NULL) OR
        (execution_type = 'parallel' AND parallel_state IS NOT NULL AND agent_id IS NULL AND action_id IS NULL AND tool_id IS NULL) OR
        (execution_type = 'collection' AND parallel_state IS NOT NULL AND agent_id IS NULL AND action_id IS NULL AND tool_id IS NULL)
    )
);

-- Basic indexes
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

-- Execution type indexes
CREATE INDEX IF NOT EXISTS idx_task_states_execution_type ON task_states (execution_type);
CREATE INDEX IF NOT EXISTS idx_task_states_parallel_state ON task_states USING GIN (parallel_state);

-- Composite indexes for common queries
CREATE INDEX IF NOT EXISTS idx_task_states_workflow_exec_id_task_id ON task_states (workflow_exec_id, task_id);
CREATE INDEX IF NOT EXISTS idx_task_states_workflow_exec_id_agent_id ON task_states (workflow_exec_id, agent_id);
CREATE INDEX IF NOT EXISTS idx_task_states_workflow_exec_id_tool_id ON task_states (workflow_exec_id, tool_id);
CREATE INDEX IF NOT EXISTS idx_task_states_workflow_id_task_id ON task_states (workflow_id, task_id);
CREATE INDEX IF NOT EXISTS idx_task_states_workflow_id_agent_id ON task_states (workflow_id, agent_id);
CREATE INDEX IF NOT EXISTS idx_task_states_workflow_id_tool_id ON task_states (workflow_id, tool_id);
CREATE INDEX IF NOT EXISTS idx_task_states_workflow_id_action_id ON task_states (workflow_id, action_id);

-- New composite indexes for parallel execution
CREATE INDEX IF NOT EXISTS idx_task_states_workflow_exec_execution_type ON task_states (workflow_exec_id, execution_type);
CREATE INDEX IF NOT EXISTS idx_task_states_workflow_exec_status_execution ON task_states (workflow_exec_id, status, execution_type);
CREATE INDEX IF NOT EXISTS idx_task_states_parallel_subtasks ON task_states USING GIN ((parallel_state->'sub_tasks'));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Remove parallel-specific indexes
DROP INDEX IF EXISTS idx_task_states_parallel_subtasks;
DROP INDEX IF EXISTS idx_task_states_workflow_exec_status_execution;
DROP INDEX IF EXISTS idx_task_states_workflow_exec_execution_type;
DROP INDEX IF EXISTS idx_task_states_parallel_state;
DROP INDEX IF EXISTS idx_task_states_execution_type;

-- Remove existing indexes
DROP INDEX IF EXISTS idx_task_states_workflow_id_action_id;
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

-- Drop the table
DROP TABLE IF EXISTS task_states;
-- +goose StatementEnd
