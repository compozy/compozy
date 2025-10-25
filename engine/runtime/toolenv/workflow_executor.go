package toolenv

import (
	"context"
	"time"

	"github.com/compozy/compozy/engine/core"
)

// WorkflowRequest captures the parameters required to execute a workflow synchronously.
type WorkflowRequest struct {
	WorkflowID    string
	Input         core.Input
	InitialTaskID string
	Timeout       time.Duration
}

// WorkflowResult represents the outcome of a workflow execution.
type WorkflowResult struct {
	WorkflowExecID core.ID
	Output         *core.Output
	Status         string
}

// WorkflowExecutor exposes synchronous workflow execution capabilities to tools.
type WorkflowExecutor interface {
	ExecuteWorkflow(context.Context, WorkflowRequest) (*WorkflowResult, error)
}
