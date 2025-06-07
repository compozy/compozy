package uc

import (
	"context"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
)

type ListTasks struct {
	workflows  []*workflow.Config
	workflowID string
}

func NewListTasks(workflows []*workflow.Config, workflowID string) *ListTasks {
	return &ListTasks{
		workflows:  workflows,
		workflowID: workflowID,
	}
}

func (uc *ListTasks) Execute(_ context.Context) ([]task.Config, error) {
	wConfig, err := workflow.FindConfig(uc.workflows, uc.workflowID)
	if err != nil {
		return nil, err
	}
	return wConfig.Tasks, nil
}
