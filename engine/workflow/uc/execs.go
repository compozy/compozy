package uc

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/workflow"
)

// -----------------------------------------------------------------------------
// GetExecution
// -----------------------------------------------------------------------------

type GetExecution struct {
	repo           workflow.Repository
	workflowExecID core.ID
}

func NewGetExecution(repo workflow.Repository, workflowExecID core.ID) *GetExecution {
	return &GetExecution{repo: repo, workflowExecID: workflowExecID}
}

func (uc *GetExecution) Execute(ctx context.Context) (*workflow.State, error) {
	exec, err := uc.repo.GetState(ctx, uc.workflowExecID)
	if err != nil {
		return nil, err
	}
	return exec, nil
}

// -----------------------------------------------------------------------------
// ListAllExecutions
// -----------------------------------------------------------------------------

type ListAllExecutions struct {
	repo workflow.Repository
}

func NewListAllExecutions(repo workflow.Repository) *ListAllExecutions {
	return &ListAllExecutions{repo: repo}
}

func (uc *ListAllExecutions) Execute(ctx context.Context) ([]*workflow.State, error) {
	execs, err := uc.repo.ListStates(ctx, &workflow.StateFilter{})
	if err != nil {
		return nil, err
	}
	return execs, nil
}

// -----------------------------------------------------------------------------
// ListExecutionsByID
// -----------------------------------------------------------------------------

type ListExecutionsByID struct {
	repo       workflow.Repository
	workflowID string
}

func NewListExecutionsByID(repo workflow.Repository, workflowID string) *ListExecutionsByID {
	return &ListExecutionsByID{
		repo:       repo,
		workflowID: workflowID,
	}
}

func (uc *ListExecutionsByID) Execute(ctx context.Context) ([]*workflow.State, error) {
	filter := &workflow.StateFilter{WorkflowID: &uc.workflowID}
	execs, err := uc.repo.ListStates(ctx, filter)
	if err != nil {
		return nil, err
	}
	return execs, nil
}
