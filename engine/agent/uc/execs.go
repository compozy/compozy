package uc

import (
	"context"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
)

// -----------------------------------------------------------------------------
// GetExecution
// -----------------------------------------------------------------------------

type GetExecution struct {
	repo        agent.Repository
	agentExecID core.ID
}

func NewGetExecution(repo agent.Repository, agentExecID core.ID) *GetExecution {
	return &GetExecution{
		repo:        repo,
		agentExecID: agentExecID,
	}
}

func (uc *GetExecution) Execute(ctx context.Context) (*agent.Execution, error) {
	return uc.repo.GetExecution(ctx, uc.agentExecID)
}

// -----------------------------------------------------------------------------
// ListAllExecutions
// -----------------------------------------------------------------------------

type ListAllExecutions struct {
	repo agent.Repository
}

func NewListAllExecutions(repo agent.Repository) *ListAllExecutions {
	return &ListAllExecutions{
		repo: repo,
	}
}

func (uc *ListAllExecutions) Execute(ctx context.Context) ([]agent.Execution, error) {
	return uc.repo.ListExecutions(ctx)
}

// -----------------------------------------------------------------------------
// ListExecutionsByAgentID
// -----------------------------------------------------------------------------

type ListExecutionsByAgentID struct {
	repo    agent.Repository
	agentID string
}

func NewListExecutionsByAgentID(repo agent.Repository, agentID string) *ListExecutionsByAgentID {
	return &ListExecutionsByAgentID{
		repo:    repo,
		agentID: agentID,
	}
}

func (uc *ListExecutionsByAgentID) Execute(ctx context.Context) ([]agent.Execution, error) {
	return uc.repo.ListExecutionsByAgentID(ctx, uc.agentID)
}
