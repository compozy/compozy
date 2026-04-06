package kinds

import "encoding/json"

// ToolCallStartedPayload describes the start of a tool call.
type ToolCallStartedPayload struct {
	Index      int             `json:"index"`
	ToolCallID string          `json:"tool_call_id"`
	Name       string          `json:"name"`
	Title      string          `json:"title,omitempty"`
	ToolName   string          `json:"tool_name,omitempty"`
	Input      json.RawMessage `json:"input,omitempty"`
	RawInput   json.RawMessage `json:"raw_input,omitempty"`
}

// ToolCallUpdatedPayload describes an update to a tool call.
type ToolCallUpdatedPayload struct {
	Index      int             `json:"index"`
	ToolCallID string          `json:"tool_call_id"`
	State      ToolCallState   `json:"state,omitempty"`
	Input      json.RawMessage `json:"input,omitempty"`
	RawInput   json.RawMessage `json:"raw_input,omitempty"`
}

// ToolCallCompletedPayload describes a completed tool call.
type ToolCallCompletedPayload struct {
	Index      int             `json:"index"`
	ToolCallID string          `json:"tool_call_id"`
	State      ToolCallState   `json:"state,omitempty"`
	Content    string          `json:"content,omitempty"`
	Output     json.RawMessage `json:"output,omitempty"`
}

// ToolCallFailedPayload describes a failed tool call.
type ToolCallFailedPayload struct {
	Index      int           `json:"index"`
	ToolCallID string        `json:"tool_call_id"`
	State      ToolCallState `json:"state,omitempty"`
	Error      string        `json:"error,omitempty"`
}
