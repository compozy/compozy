package executors

import (
	"github.com/compozy/compozy/engine/task"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type TaskSleepExecutor struct {
	*ContextBuilder
}

func NewTaskSleepExecutor(contextBuilder *ContextBuilder) *TaskSleepExecutor {
	return &TaskSleepExecutor{ContextBuilder: contextBuilder}
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
	timer := workflow.NewTimer(ctx, sleepDuration)
	if err := timer.Get(ctx, nil); err != nil {
		return nil, err
	}
	return nil, nil
}
