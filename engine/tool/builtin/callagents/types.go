package callagents

import "github.com/compozy/compozy/engine/core"

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

// ErrorDetails describes a failure returned for a single agent execution.
type ErrorDetails struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

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
