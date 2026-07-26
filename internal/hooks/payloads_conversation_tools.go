package hooks

import "encoding/json"

// TurnPayload is shared by turn start and end events.
type TurnPayload struct {
	PayloadBase
	SessionContext
	TurnContext
	InputClass  string `json:"input_class,omitempty"`
	UserMessage string `json:"user_message,omitempty"`
}

// TurnStartPayload is delivered at turn start.
type TurnStartPayload = TurnPayload

// TurnEndPayload is delivered at turn end.
type TurnEndPayload = TurnPayload

// TurnPatch mutates or denies turn-scoped operations.
type TurnPatch struct {
	ControlPatch
	Labels map[string]string `json:"labels,omitempty"`
}

// TurnStartPatch is the turn-start patch surface.
type TurnStartPatch = TurnPatch

// TurnEndPatch is the turn-end patch surface.
type TurnEndPatch = TurnPatch

// MessagePayload is shared by message start, delta, and end events.
type MessagePayload struct {
	PayloadBase
	SessionContext
	TurnContext
	MessageID string          `json:"message_id,omitempty"`
	Role      string          `json:"role,omitempty"`
	DeltaType string          `json:"delta_type,omitempty"`
	Text      string          `json:"text,omitempty"`
	Raw       json.RawMessage `json:"raw,omitempty"`
}

// MessageStartPayload is delivered when a message begins.
type MessageStartPayload = MessagePayload

// MessageDeltaPayload is delivered for streaming message deltas.
type MessageDeltaPayload = MessagePayload

// MessageEndPayload is delivered when a message finishes.
type MessageEndPayload = MessagePayload

// MessagePatch mutates or denies message-scoped operations.
type MessagePatch struct {
	ControlPatch
	Role      *string `json:"role,omitempty"`
	DeltaType *string `json:"delta_type,omitempty"`
	Text      *string `json:"text,omitempty"`
}

// MessageStartPatch is the message-start patch surface.
type MessageStartPatch = MessagePatch

// MessageDeltaPatch is the message-delta patch surface.
type MessageDeltaPatch = MessagePatch

// MessageEndPatch is the message-end patch surface.
type MessageEndPatch = MessagePatch

// ToolPreCallPayload is delivered before a tool runs.
type ToolPreCallPayload struct {
	PayloadBase
	SessionContext
	TurnContext
	ToolCallRef
	ToolInput json.RawMessage `json:"tool_input,omitempty"`
}

// ToolPostCallPayload is delivered after a tool completes successfully.
type ToolPostCallPayload struct {
	PayloadBase
	SessionContext
	TurnContext
	ToolCallRef
	Title      string          `json:"title,omitempty"`
	ToolInput  json.RawMessage `json:"tool_input,omitempty"`
	ToolResult json.RawMessage `json:"tool_result,omitempty"`
}

// ToolPostErrorPayload is delivered after a tool fails.
type ToolPostErrorPayload struct {
	PayloadBase
	SessionContext
	TurnContext
	ToolCallRef
	Title     string          `json:"title,omitempty"`
	ToolInput json.RawMessage `json:"tool_input,omitempty"`
	Error     string          `json:"error,omitempty"`
}

// ToolCallPatch mutates or denies tool invocation inputs.
type ToolCallPatch struct {
	ControlPatch
	ToolID    *string         `json:"tool_id,omitempty"`
	ReadOnly  *bool           `json:"read_only,omitempty"`
	ToolInput json.RawMessage `json:"tool_input,omitempty"`
}

// ToolResultPatch mutates or denies tool outputs.
type ToolResultPatch struct {
	ControlPatch
	Title      *string         `json:"title,omitempty"`
	ToolResult json.RawMessage `json:"tool_result,omitempty"`
	Error      *string         `json:"error,omitempty"`
}

// ToolPostErrorPatch is the post-error patch surface.
type ToolPostErrorPatch = ToolResultPatch

// PermissionRequestPayload is delivered before a permission decision resolves.
type PermissionRequestPayload struct {
	PayloadBase
	SessionContext
	TurnContext
	RequestID     string             `json:"request_id,omitempty"`
	Action        string             `json:"action,omitempty"`
	Resource      string             `json:"resource,omitempty"`
	Decision      string             `json:"decision,omitempty"`
	DecisionClass string             `json:"decision_class,omitempty"`
	ToolInput     json.RawMessage    `json:"tool_input,omitempty"`
	ToolCall      PermissionToolCall `json:"tool_call"`
	Options       []PermissionOption `json:"options,omitempty"`
}

// PermissionResolutionPayload is shared by resolved and denied events.
type PermissionResolutionPayload struct {
	PayloadBase
	SessionContext
	TurnContext
	RequestID     string             `json:"request_id,omitempty"`
	Action        string             `json:"action,omitempty"`
	Resource      string             `json:"resource,omitempty"`
	Decision      string             `json:"decision,omitempty"`
	DecisionClass string             `json:"decision_class,omitempty"`
	ToolInput     json.RawMessage    `json:"tool_input,omitempty"`
	ToolCall      PermissionToolCall `json:"tool_call"`
}

// PermissionResolvedPayload is delivered after a permission decision resolves.
type PermissionResolvedPayload = PermissionResolutionPayload

// PermissionDeniedPayload is delivered after a permission denial resolves.
type PermissionDeniedPayload = PermissionResolutionPayload

// PermissionRequestPatch mutates or denies the permission-request surface.
type PermissionRequestPatch struct {
	ControlPatch
	Decision      *string `json:"decision,omitempty"`
	DecisionClass *string `json:"decision_class,omitempty"`
	Reason        *string `json:"reason,omitempty"`
}

// PermissionResolvedPatch is the resolved patch surface.
type PermissionResolvedPatch struct{}

// PermissionDeniedPatch is the denied patch surface.
type PermissionDeniedPatch struct{}
