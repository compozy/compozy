package client

import (
	"github.com/compozy/compozy/engine/core"
)

// ExecutionResult summarises the metadata returned when triggering a workflow.
type ExecutionResult struct {
	ExecutionID string
	WorkflowID  string
	Endpoint    string
}

// WorkflowStatus captures the state of a workflow execution.
type WorkflowStatus struct {
	WorkflowID  string
	ExecutionID string
	Status      core.StatusType
	Output      *core.Output
	Error       *core.Error
}
