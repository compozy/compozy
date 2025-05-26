package uc

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/workflow"
)

type GetExecution struct {
	repo           workflow.Repository
	WorkflowExecID core.ID
}

func NewGetExecutionUC(repo workflow.Repository, workflowExecID core.ID) *GetExecution {
	return &GetExecution{
		repo:           repo,
		WorkflowExecID: workflowExecID,
	}
}

func (uc *GetExecution) Execute(ctx context.Context) (*core.ExecutionMap, error) {
	return uc.repo.LoadExecutionMap(ctx, uc.WorkflowExecID)
}
