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

func NewGetExecution(repo workflow.Repository, workflowExecID core.ID) *GetExecution {
	return &GetExecution{
		repo:           repo,
		WorkflowExecID: workflowExecID,
	}
}

func (uc *GetExecution) Execute(ctx context.Context) (*map[core.ID]any, error) {
	exec, err := uc.repo.LoadExecution(ctx, uc.WorkflowExecID)
	if err != nil {
		return nil, err
	}
	execMap := exec.AsMap()
	return &execMap, nil
}
