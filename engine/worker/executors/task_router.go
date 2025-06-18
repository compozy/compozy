package executors

import (
	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/task"
	tkacts "github.com/compozy/compozy/engine/task/activities"
)

type TaskRouterExecutor struct {
	*ContextBuilder
}

func NewTaskRouterExecutor(contextBuilder *ContextBuilder) *TaskRouterExecutor {
	return &TaskRouterExecutor{ContextBuilder: contextBuilder}
}

func (e *TaskRouterExecutor) Execute(ctx workflow.Context, taskConfig *task.Config) (task.Response, error) {
	var response *task.MainTaskResponse
	actLabel := tkacts.ExecuteRouterLabel
	actInput := tkacts.ExecuteRouterInput{
		WorkflowID:     e.WorkflowID,
		WorkflowExecID: e.WorkflowExecID,
		TaskConfig:     taskConfig,
	}
	err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &response)
	if err != nil {
		return nil, err
	}
	return response, nil
}
