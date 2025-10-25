package toolenv

import (
	"context"
	"time"

	"github.com/compozy/compozy/engine/core"
)

// TaskRequest captures the parameters required to execute a task synchronously.
type TaskRequest struct {
	TaskID  string
	With    core.Input
	Timeout time.Duration
}

// TaskResult represents the outcome of a task execution.
type TaskResult struct {
	ExecID core.ID
	Output *core.Output
}

// TaskExecutor exposes synchronous task execution capabilities to tools.
type TaskExecutor interface {
	ExecuteTask(context.Context, TaskRequest) (*TaskResult, error)
}
