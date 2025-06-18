package executors

import (
	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/task"
	tkacts "github.com/compozy/compozy/engine/task/activities"
)

type TaskBasicExecutor struct {
	*ContextBuilder
}

func NewTaskBasicExecutor(contextBuilder *ContextBuilder) *TaskBasicExecutor {
	return &TaskBasicExecutor{ContextBuilder: contextBuilder}
}

func (e *TaskBasicExecutor) Execute(ctx workflow.Context, taskConfig *task.Config) (task.Response, error) {
	var response *task.MainTaskResponse
	actLabel := tkacts.ExecuteBasicLabel
	actInput := tkacts.ExecuteBasicInput{
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
