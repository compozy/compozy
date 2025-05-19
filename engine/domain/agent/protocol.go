package agent

import (
	"encoding/json"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/domain/tool"
	"github.com/google/uuid"
)

// Request represents a request to execute an agent
type Request struct {
	ID           string         `json:"id"`
	AgentID      string         `json:"agent_id"`
	Instructions string         `json:"instructions"`
	Action       ActionRequest  `json:"action"`
	Config       map[string]any `json:"config"`
	Tools        []tool.Request `json:"tools"`
}

// NewAgentRequest creates a new agent request
func NewAgentRequest(
	agentID, instructions string,
	action ActionRequest,
	config map[string]any,
	tools []tool.Request,
) *Request {
	return &Request{
		ID:           uuid.New().String(),
		AgentID:      agentID,
		Instructions: instructions,
		Action:       action,
		Config:       config,
		Tools:        tools,
	}
}

// ActionRequest represents a request to execute an agent action
type ActionRequest struct {
	ActionID     string         `json:"action_id"`
	Prompt       string         `json:"prompt"`
	OutputSchema map[string]any `json:"output_schema,omitempty"`
}

// Response represents a response from an agent execution
type Response struct {
	ID      string                `json:"id"`
	AgentID string                `json:"agent_id"`
	Output  json.RawMessage       `json:"output"`
	Status  common.ResponseStatus `json:"status"`
}
