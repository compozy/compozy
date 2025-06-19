package executors

import (
	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/task"
	tkacts "github.com/compozy/compozy/engine/task/activities"
)

type TaskSignalExecutor struct {
	*ContextBuilder
}

func NewTaskSignalExecutor(contextBuilder *ContextBuilder) *TaskSignalExecutor {
	return &TaskSignalExecutor{ContextBuilder: contextBuilder}
}

func (e *TaskSignalExecutor) Execute(ctx workflow.Context, taskConfig *task.Config) (task.Response, error) {
	var response *task.MainTaskResponse
	actLabel := tkacts.ExecuteSignalLabel
	actInput := tkacts.ExecuteSignalInput{
		WorkflowID:     e.WorkflowID,
		WorkflowExecID: e.WorkflowExecID,
		TaskConfig:     taskConfig,
		ProjectName:    e.ProjectConfig.Name,
	}
	err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &response)
	if err != nil {
		return nil, err
	}
	return response, nil
}
