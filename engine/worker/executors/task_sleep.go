package executors

import (
	"github.com/compozy/compozy/engine/task"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type TaskSleepExecutor struct {
	*ContextBuilder
	taskConfig *task.Config
}

func NewTaskSleepExecutor(contextBuilder *ContextBuilder, taskConfig *task.Config) *TaskSleepExecutor {
	return &TaskSleepExecutor{ContextBuilder: contextBuilder, taskConfig: taskConfig}
}

func (e *TaskSleepExecutor) Execute(ctx workflow.Context, taskConfig *task.Config) (*task.Response, error) {
	log := workflow.GetLogger(ctx)
	if temporal.IsCanceledError(ctx.Err()) {
		log.Info("Sleep skipped due to cancellation")
		return nil, workflow.ErrCanceled
	}
	taskID := taskConfig.ID
	sleepDuration, err := taskConfig.GetSleepDuration()
	if err != nil {
		log.Error("Invalid sleep duration format", "task_id", taskID, "sleep", taskConfig.Sleep, "error", err)
		return nil, err
	}
	if sleepDuration == 0 {
		log.Info("Sleep skipped due to zero duration")
		return nil, nil
	}
	timerDone := false
	timer := workflow.NewTimer(ctx, sleepDuration)
	for !timerDone {
		// Check cancellation before each iteration
		if temporal.IsCanceledError(ctx.Err()) {
			log.Info("Sleep interrupted by cancellation")
			return nil, workflow.ErrCanceled
		}
		sel := workflow.NewSelector(ctx)
		sel.AddFuture(timer, func(workflow.Future) { timerDone = true })
		sel.Select(ctx)
		// Check again after select
		if temporal.IsCanceledError(ctx.Err()) {
			log.Info("Sleep interrupted by cancellation")
			return nil, workflow.ErrCanceled
		}
	}
	return nil, nil
}
