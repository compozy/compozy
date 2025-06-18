package executors

import (
	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/task"
	tkacts "github.com/compozy/compozy/engine/task/activities"
)

type TaskAggregateExecutor struct {
	*ContextBuilder
}

func NewTaskAggregateExecutor(contextBuilder *ContextBuilder) *TaskAggregateExecutor {
	return &TaskAggregateExecutor{ContextBuilder: contextBuilder}
}

func (e *TaskAggregateExecutor) Execute(ctx workflow.Context, taskConfig *task.Config) (task.Response, error) {
	var response *task.MainTaskResponse
	actLabel := tkacts.ExecuteAggregateLabel
	actInput := tkacts.ExecuteAggregateInput{
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
