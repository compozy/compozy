package uc

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/workflow"
)

// -----------------------------------------------------------------------------
// ListExecutions
// -----------------------------------------------------------------------------

type ListExecutions struct {
	repo workflow.Repository
}

func NewListExecutions(repo workflow.Repository) *ListExecutions {
	return &ListExecutions{repo: repo}
}

func (uc *ListExecutions) Execute(ctx context.Context) ([]map[core.ID]any, error) {
	return uc.repo.ListExecutionsMap(ctx)
}

// -----------------------------------------------------------------------------
// ListExecutionsByWorkflowID
// -----------------------------------------------------------------------------

type ListExecutionsByWorkflowID struct {
	repo       workflow.Repository
	workflowID core.ID
}

func NewListExecutionsByWorkflowID(repo workflow.Repository, workflowID core.ID) *ListExecutionsByWorkflowID {
	return &ListExecutionsByWorkflowID{
		repo:       repo,
		workflowID: workflowID,
	}
}

func (uc *ListExecutionsByWorkflowID) Execute(ctx context.Context) ([]map[core.ID]any, error) {
	return uc.repo.ListExecutionsMapByWorkflowID(ctx, uc.workflowID)
}
