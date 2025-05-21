package agent

import (
	"encoding/json"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/domain/tool"
)

type Request struct {
	AgentExecID  string         `json:"agent_exec_id"`
	AgentID      string         `json:"agent_id"`
	Instructions string         `json:"instructions"`
	Action       ActionRequest  `json:"action"`
	Config       map[string]any `json:"config"`
	Tools        []tool.Request `json:"tools"`
}

func NewAgentRequest(
	agExecID common.ID,
	agID, instructions string,
	action ActionRequest,
	config map[string]any,
	tools []tool.Request,
) *Request {
	return &Request{
		AgentExecID:  agExecID.String(),
		AgentID:      agID,
		Instructions: instructions,
		Action:       action,
		Config:       config,
		Tools:        tools,
	}
}

type ActionRequest struct {
	ActionID     string         `json:"action_id"`
	Prompt       string         `json:"prompt"`
	OutputSchema map[string]any `json:"output_schema,omitempty"`
}

type Response struct {
	AgentExecID string                `json:"agent_exec_id"`
	Output      json.RawMessage       `json:"output"`
	Status      common.ResponseStatus `json:"status"`
}
