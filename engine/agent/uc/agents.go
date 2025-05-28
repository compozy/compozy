package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/workflow"
)

// -----------------------------------------------------------------------------
// GetAgent
// -----------------------------------------------------------------------------

type GetAgent struct {
	workflows []*workflow.Config
	agentID   string
}

func NewGetAgent(workflows []*workflow.Config, agentID string) *GetAgent {
	return &GetAgent{
		workflows: workflows,
		agentID:   agentID,
	}
}

func (uc *GetAgent) Execute(_ context.Context) (*agent.Config, error) {
	for _, wf := range uc.workflows {
		for i := range wf.Agents {
			if wf.Agents[i].ID == uc.agentID {
				return &wf.Agents[i], nil
			}
		}
	}
	return nil, fmt.Errorf("agent not found")
}

// -----------------------------------------------------------------------------
// ListAgents
// -----------------------------------------------------------------------------

type ListAgents struct {
	workflows []*workflow.Config
}

func NewListAgents(workflows []*workflow.Config) *ListAgents {
	return &ListAgents{
		workflows: workflows,
	}
}

func (uc *ListAgents) Execute(_ context.Context) ([]agent.Config, error) {
	agents := make([]agent.Config, 0)
	seen := make(map[string]bool)

	for _, wf := range uc.workflows {
		for i := range wf.Agents {
			if !seen[wf.Agents[i].ID] {
				agents = append(agents, wf.Agents[i])
				seen[wf.Agents[i].ID] = true
			}
		}
	}

	return agents, nil
}
