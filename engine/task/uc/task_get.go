package uc

import (
	"context"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
)

type GetTask struct {
	workflows  []*workflow.Config
	workflowID string
	taskID     string
}

func NewGetTask(workflows []*workflow.Config, workflowID, taskID string) *GetTask {
	return &GetTask{
		workflows:  workflows,
		workflowID: workflowID,
		taskID:     taskID,
	}
}

func (uc *GetTask) Execute(_ context.Context) (*task.Config, error) {
	wConfig, err := workflow.FindConfig(uc.workflows, uc.workflowID)
	if err != nil {
		return nil, err
	}
	return task.FindConfig(wConfig.Tasks, uc.taskID)
}
