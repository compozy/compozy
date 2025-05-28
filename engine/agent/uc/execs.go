package uc

import (
	"context"

	"github.com/compozy/compozy/engine/agent"
)

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
