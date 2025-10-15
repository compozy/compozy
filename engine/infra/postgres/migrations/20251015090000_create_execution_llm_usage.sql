-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS execution_llm_usage (
    id                   bigserial PRIMARY KEY,
    workflow_exec_id     text REFERENCES workflow_states (workflow_exec_id) ON DELETE CASCADE,
    task_exec_id         text REFERENCES task_states (task_exec_id) ON DELETE CASCADE,
    component            text NOT NULL,
    agent_id             text,
    provider             text NOT NULL,
    model                text NOT NULL,
    prompt_tokens        integer NOT NULL DEFAULT 0,
    completion_tokens    integer NOT NULL DEFAULT 0,
    total_tokens         integer NOT NULL DEFAULT 0,
    reasoning_tokens     integer,
    cached_prompt_tokens integer,
    input_audio_tokens   integer,
    output_audio_tokens  integer,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT check_exec_id_not_null CHECK (
        workflow_exec_id IS NOT NULL OR task_exec_id IS NOT NULL
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_execution_usage_task_component
    ON execution_llm_usage (task_exec_id, component)
    WHERE task_exec_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_execution_usage_workflow_component
    ON execution_llm_usage (workflow_exec_id, component)
    WHERE task_exec_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_execution_usage_workflow_exec_id ON execution_llm_usage (workflow_exec_id);
CREATE INDEX IF NOT EXISTS idx_execution_usage_task_exec_id ON execution_llm_usage (task_exec_id);
CREATE INDEX IF NOT EXISTS idx_execution_usage_component_created_at ON execution_llm_usage (component, created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS uq_execution_usage_workflow_component;
DROP INDEX IF EXISTS uq_execution_usage_task_component;
DROP INDEX IF EXISTS idx_execution_usage_component_created_at;
DROP INDEX IF EXISTS idx_execution_usage_task_exec_id;
DROP INDEX IF EXISTS idx_execution_usage_workflow_exec_id;
DROP TABLE IF EXISTS execution_llm_usage;
-- +goose StatementEnd
