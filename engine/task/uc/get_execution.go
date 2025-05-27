package uc

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
)

type GetExecution struct {
	repo       task.Repository
	taskExecID core.ID
}

func NewGetExecutionUC(repo task.Repository, taskExecID core.ID) *GetExecution {
	return &GetExecution{
		repo:       repo,
		taskExecID: taskExecID,
	}
}

func (uc *GetExecution) Execute(ctx context.Context) (*task.Execution, error) {
	return uc.repo.LoadExecution(ctx, uc.taskExecID)
}
