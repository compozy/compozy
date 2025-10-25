package callworkflows

import (
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/tool/builtin/shared"
)

// WorkflowExecutionRequest captures user-provided workflow parameters.
type WorkflowExecutionRequest struct {
	WorkflowID    string         `json:"workflow_id"     mapstructure:"workflow_id"`
	Input         map[string]any `json:"input"           mapstructure:"input"`
	InitialTaskID string         `json:"initial_task_id" mapstructure:"initial_task_id"`
	TimeoutMs     int            `json:"timeout_ms"      mapstructure:"timeout_ms"`
}

// handlerInput wraps the top-level payload accepted by cp__call_workflows.
type handlerInput struct {
	Workflows []WorkflowExecutionRequest `json:"workflows" mapstructure:"workflows"`
}

// WorkflowExecutionResult describes the outcome of an individual workflow invocation.
type WorkflowExecutionResult struct {
	Success        bool                 `json:"success"`
	WorkflowID     string               `json:"workflow_id"`
	WorkflowExecID string               `json:"workflow_exec_id,omitempty"`
	Status         string               `json:"status"`
	Output         core.Output          `json:"output,omitempty"`
	Error          *shared.ErrorDetails `json:"error,omitempty"`
	DurationMs     int64                `json:"duration_ms"`
}
