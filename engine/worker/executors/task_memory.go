package executors

import (
	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/task"
	tkacts "github.com/compozy/compozy/engine/task/activities"
)

type TaskMemoryExecutor struct {
	*ContextBuilder
}

func NewTaskMemoryExecutor(contextBuilder *ContextBuilder) *TaskMemoryExecutor {
	return &TaskMemoryExecutor{ContextBuilder: contextBuilder}
}

func (e *TaskMemoryExecutor) Execute(ctx workflow.Context, taskConfig *task.Config) (task.Response, error) {
	var response *task.MainTaskResponse
	actLabel := tkacts.ExecuteMemoryLabel
	actInput := &tkacts.ExecuteMemoryInput{
		WorkflowID:     e.WorkflowID,
		WorkflowExecID: e.WorkflowExecID,
		TaskConfig:     taskConfig,
		MergedInput:    e.Input,
	}
	err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &response)
	if err != nil {
		return nil, err
	}
	return response, nil
}
