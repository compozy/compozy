package telemetry

import (
	"encoding/json"
	"time"
)

// Severity expresses importance of a telemetry event.
type Severity string

const (
	SeverityDebug Severity = "debug"
	SeverityInfo  Severity = "info"
	SeverityWarn  Severity = "warn"
	SeverityError Severity = "error"
)

// RedactedValue represents redacted content placeholders.
const RedactedValue = "[redacted]"

// Event describes a run-scoped telemetry event.
type Event struct {
	Stage     string         `json:"stage"`
	Iteration int            `json:"iteration,omitempty"`
	Severity  Severity       `json:"severity,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
	RunID     string         `json:"run_id"`
	AgentID   string         `json:"agent_id,omitempty"`
	ActionID  string         `json:"action_id,omitempty"`
	Workflow  string         `json:"workflow_id,omitempty"`
	ExecID    string         `json:"execution_id,omitempty"`
}

// ToolStatus enumerates possible tool execution outcomes.
type ToolStatus string

const (
	ToolStatusSuccess ToolStatus = "success"
	ToolStatusError   ToolStatus = "error"
	ToolStatusSkipped ToolStatus = "skipped"
)

// ToolLogEntry captures a single tool invocation outcome.
type ToolLogEntry struct {
	ToolCallID string         `json:"tool_call_id"`
	ToolName   string         `json:"tool_name"`
	Input      string         `json:"input,omitempty"`
	Output     string         `json:"output,omitempty"`
	Error      string         `json:"error,omitempty"`
	Status     ToolStatus     `json:"status"`
	Duration   time.Duration  `json:"duration"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Redacted   bool           `json:"redacted"`
}

// MessageSnapshot provides a serialisable view of an LLM message.
type MessageSnapshot struct {
	Role        string               `json:"role"`
	Content     string               `json:"content,omitempty"`
	HasParts    bool                 `json:"has_parts,omitempty"`
	ToolCalls   []ToolCallSnapshot   `json:"tool_calls,omitempty"`
	ToolResults []ToolResultSnapshot `json:"tool_results,omitempty"`
}

// ToolCallSnapshot captures assistant-emitted tool call metadata.
type ToolCallSnapshot struct {
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// ToolResultSnapshot captures tool response metadata.
type ToolResultSnapshot struct {
	ID          string          `json:"id,omitempty"`
	Name        string          `json:"name,omitempty"`
	Content     string          `json:"content,omitempty"`
	JSONContent json.RawMessage `json:"json_content,omitempty"`
}

// ContextUsage tracks token utilization relative to a known limit.
type ContextUsage struct {
	PromptTokens     int     `json:"prompt_tokens,omitempty"`
	CompletionTokens int     `json:"completion_tokens,omitempty"`
	TotalTokens      int     `json:"total_tokens,omitempty"`
	ContextLimit     int     `json:"context_limit,omitempty"`
	LimitSource      string  `json:"limit_source,omitempty"`
	PercentOfLimit   float64 `json:"percent_of_limit,omitempty"`
}

// RunMetadata describes identifiers associated with a run.
type RunMetadata struct {
	AgentID     string `json:"agent_id,omitempty"`
	ActionID    string `json:"action_id,omitempty"`
	WorkflowID  string `json:"workflow_id,omitempty"`
	ExecutionID string `json:"execution_id,omitempty"`
}

// RunResult summarizes the final state of a run.
type RunResult struct {
	Success bool
	Error   error
	Summary map[string]any
}
