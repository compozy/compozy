package calltasks

import "github.com/compozy/compozy/engine/core"

// TaskExecutionRequest captures user-provided task execution parameters.
type TaskExecutionRequest struct {
	TaskID    string         `json:"task_id"    mapstructure:"task_id"`
	With      map[string]any `json:"with"       mapstructure:"with"`
	TimeoutMs int            `json:"timeout_ms" mapstructure:"timeout_ms"`
}

// handlerInput wraps the top-level payload accepted by cp__call_tasks.
type handlerInput struct {
	Tasks []TaskExecutionRequest `json:"tasks" mapstructure:"tasks"`
}

// ErrorDetails reports information for a failed task execution.
type ErrorDetails struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

// TaskExecutionResult describes the outcome of an individual task invocation.
type TaskExecutionResult struct {
	Success    bool          `json:"success"`
	TaskID     string        `json:"task_id"`
	ExecID     string        `json:"exec_id,omitempty"`
	Output     core.Output   `json:"output,omitempty"`
	Error      *ErrorDetails `json:"error,omitempty"`
	DurationMs int64         `json:"duration_ms"`
}
