CREATE TABLE IF NOT EXISTS executions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT NOT NULL UNIQUE,
    component_type TEXT NOT NULL CHECK(component_type IN ('workflow', 'task', 'agent', 'tool')),
    workflow_id TEXT NOT NULL,
    workflow_exec_id TEXT NOT NULL,
    task_id TEXT,
    task_exec_id TEXT,
    agent_id TEXT,
    agent_exec_id TEXT,
    tool_id TEXT,
    tool_exec_id TEXT,
    status TEXT NOT NULL,
    data JSONB NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_component_type ON executions (component_type);
CREATE INDEX IF NOT EXISTS idx_key ON executions (key);
CREATE INDEX IF NOT EXISTS idx_workflow_id ON executions (workflow_id);
CREATE INDEX IF NOT EXISTS idx_workflow_exec_id ON executions (workflow_exec_id);
CREATE INDEX IF NOT EXISTS idx_task_id ON executions (task_id);
CREATE INDEX IF NOT EXISTS idx_task_exec_id ON executions (task_exec_id);
CREATE INDEX IF NOT EXISTS idx_agent_id ON executions (agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_exec_id ON executions (agent_exec_id);
CREATE INDEX IF NOT EXISTS idx_tool_id ON executions (tool_id);
CREATE INDEX IF NOT EXISTS idx_tool_exec_id ON executions (tool_exec_id);
CREATE INDEX IF NOT EXISTS idx_status ON executions (status);
CREATE INDEX IF NOT EXISTS idx_status_component_type ON executions (status, component_type);
CREATE INDEX IF NOT EXISTS idx_workflow_id_component_type ON executions (workflow_id, component_type);
CREATE INDEX IF NOT EXISTS idx_workflow_exec_id_component_type ON executions (workflow_exec_id, component_type);
CREATE INDEX IF NOT EXISTS idx_task_id_component_type ON executions (task_id, component_type);
CREATE INDEX IF NOT EXISTS idx_task_exec_id_component_type ON executions (task_exec_id, component_type);
CREATE INDEX IF NOT EXISTS idx_agent_id_component_type ON executions (agent_id, component_type);
CREATE INDEX IF NOT EXISTS idx_agent_exec_id_component_type ON executions (agent_exec_id, component_type);
CREATE INDEX IF NOT EXISTS idx_tool_id_component_type ON executions (tool_id, component_type);
CREATE INDEX IF NOT EXISTS idx_tool_exec_id_component_type ON executions (tool_exec_id, component_type);
CREATE INDEX IF NOT EXISTS idx_created_at ON executions (created_at);
