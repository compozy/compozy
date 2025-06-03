package uc

import (
	"context"

	"github.com/compozy/compozy/engine/workflow"
)

// -----------------------------------------------------------------------------
// GetExecution
// -----------------------------------------------------------------------------

type GetExecution struct {
	repo    workflow.Repository
	stateID string
}

func NewGetExecution(repo workflow.Repository, stateID string) *GetExecution {
	return &GetExecution{
		repo:    repo,
		stateID: stateID,
	}
}

func (uc *GetExecution) Execute(ctx context.Context) (*workflow.State, error) {
	stateID, err := workflow.StateIDFromString(uc.stateID)
	if err != nil {
		return nil, err
	}
	exec, err := uc.repo.GetState(ctx, *stateID)
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
