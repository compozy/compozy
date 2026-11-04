package contract

import "github.com/compozy/compozy/internal/agentinstance"

type AgentActivityResponse agentinstance.Activity

// AgentActivityBatchRequest retains each item's scope independently, including same-named definitions.
type AgentActivityBatchRequest struct {
	Days  int                      `json:"days,omitempty"`
	Items []AgentActivitySelection `json:"items"`
}

type AgentActivitySelection struct {
	Name       string   `json:"name"`
	Workspaces []string `json:"workspaces"`
}

// AgentActivityBatchResponse returns one unchanged activity payload per request item, in order.
type AgentActivityBatchResponse struct {
	Activities []AgentActivityResponse `json:"activities"`
}
