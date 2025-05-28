package uc

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/tool"
)

// -----------------------------------------------------------------------------
// GetExecution
// -----------------------------------------------------------------------------

type GetExecution struct {
	repo       tool.Repository
	toolExecID core.ID
}

func NewGetExecution(repo tool.Repository, toolExecID core.ID) *GetExecution {
	return &GetExecution{
		repo:       repo,
		toolExecID: toolExecID,
	}
}

func (uc *GetExecution) Execute(ctx context.Context) (*tool.Execution, error) {
	return uc.repo.GetExecution(ctx, uc.toolExecID)
}

// -----------------------------------------------------------------------------
// ListAllExecutions
// -----------------------------------------------------------------------------

type ListAllExecutions struct {
	repo tool.Repository
}

func NewListAllExecutions(repo tool.Repository) *ListAllExecutions {
	return &ListAllExecutions{
		repo: repo,
	}
}

func (uc *ListAllExecutions) Execute(ctx context.Context) ([]tool.Execution, error) {
	return uc.repo.ListExecutions(ctx)
}

// -----------------------------------------------------------------------------
// ListExecutionsByToolID
// -----------------------------------------------------------------------------

type ListExecutionsByToolID struct {
	repo   tool.Repository
	toolID string
}

func NewListExecutionsByToolID(repo tool.Repository, toolID string) *ListExecutionsByToolID {
	return &ListExecutionsByToolID{
		repo:   repo,
		toolID: toolID,
	}
}

func (uc *ListExecutionsByToolID) Execute(ctx context.Context) ([]tool.Execution, error) {
	return uc.repo.ListExecutionsByToolID(ctx, uc.toolID)
}
