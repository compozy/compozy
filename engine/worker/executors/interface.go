package executors

import (
	"github.com/compozy/compozy/engine/task"
	"go.temporal.io/sdk/workflow"
)

type Executor interface {
	Execute(ctx workflow.Context, taskConfig *task.Config) (task.Response, error)
}
