package callagents

import (
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/tool/builtin/shared"
)

// AgentExecutionRequest captures the user input needed to execute an agent.
type AgentExecutionRequest struct {
	AgentID   string         `json:"agent_id"   mapstructure:"agent_id"`
	ActionID  string         `json:"action_id"  mapstructure:"action_id"`
	Prompt    string         `json:"prompt"     mapstructure:"prompt"`
	With      map[string]any `json:"with"       mapstructure:"with"`
	TimeoutMs int            `json:"timeout_ms" mapstructure:"timeout_ms"`
}

// handlerInput is the top-level payload accepted by cp__call_agents.
type handlerInput struct {
	Agents []AgentExecutionRequest `json:"agents" mapstructure:"agents"`
}

// ErrorDetails aliases the shared builtin error contract for backward compatibility.
type ErrorDetails = shared.ErrorDetails

// AgentExecutionResult reports the outcome of a single agent invocation.
type AgentExecutionResult struct {
	Success    bool          `json:"success"`
	AgentID    string        `json:"agent_id"`
	ActionID   string        `json:"action_id,omitempty"`
	ExecID     string        `json:"exec_id,omitempty"`
	Response   core.Output   `json:"response,omitempty"`
	Error      *ErrorDetails `json:"error,omitempty"`
	DurationMs int64         `json:"duration_ms"`
}
