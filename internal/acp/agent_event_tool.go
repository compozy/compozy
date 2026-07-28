package acp

import (
	"encoding/json"
	"strings"
)

type agentEventPayload struct {
	clientMessageID string
	toolName        string
	toolKind        string
	toolInput       json.RawMessage
	toolErrorDetail string
	toolFailed      bool
	hasTool         bool
}

func (e AgentEvent) clonePayload() *agentEventPayload {
	if e.payload == nil {
		return &agentEventPayload{}
	}
	cloned := *e.payload
	cloned.toolInput = CloneRawMessage(e.payload.toolInput)
	return &cloned
}

func normalizeAgentEventPayload(payload *agentEventPayload) *agentEventPayload {
	if payload == nil || payload.clientMessageID == "" && !payload.hasTool {
		return nil
	}
	return payload
}

// WithClientMessageID returns an event carrying the normalized authored-message identity.
func (e AgentEvent) WithClientMessageID(clientMessageID string) AgentEvent {
	payload := e.clonePayload()
	payload.clientMessageID = strings.TrimSpace(clientMessageID)
	e.payload = normalizeAgentEventPayload(payload)
	return e
}

// ClientMessageIDValue returns the normalized client identity for an authored
// user message, or an empty string when the event has no client identity.
func (e *AgentEvent) ClientMessageIDValue() string {
	if e == nil || e.payload == nil {
		return ""
	}
	return e.payload.clientMessageID
}

// WithTool returns an event carrying an isolated optional tool payload.
func (e AgentEvent) WithTool(name string, input json.RawMessage, failed bool) AgentEvent {
	return e.WithToolDetail(name, input, failed, "")
}

// WithToolDetail returns an event carrying isolated tool metadata and an optional result error.
func (e AgentEvent) WithToolDetail(
	name string,
	input json.RawMessage,
	failed bool,
	errorDetail string,
) AgentEvent {
	payload := e.clonePayload()
	payload.toolName = name
	payload.toolInput = CloneRawMessage(input)
	payload.toolFailed = failed || errorDetail != ""
	payload.toolErrorDetail = errorDetail
	payload.hasTool = name != "" || len(input) > 0 || failed || errorDetail != ""
	e.payload = normalizeAgentEventPayload(payload)
	return e
}

// WithToolKind returns an event carrying the normalized tool kind.
func (e AgentEvent) WithToolKind(kind string) AgentEvent {
	payload := e.clonePayload()
	payload.toolKind = strings.TrimSpace(kind)
	payload.hasTool = payload.hasTool || payload.toolKind != ""
	e.payload = normalizeAgentEventPayload(payload)
	return e
}

// ToolName returns the optional tool name.
func (e AgentEvent) ToolName() string {
	if e.payload == nil || !e.payload.hasTool {
		return ""
	}
	return e.payload.toolName
}

// ToolKind returns the optional normalized tool kind.
func (e AgentEvent) ToolKind() string {
	if e.payload == nil || !e.payload.hasTool {
		return ""
	}
	return e.payload.toolKind
}

// ToolInput returns an isolated copy of the optional tool input.
func (e AgentEvent) ToolInput() json.RawMessage {
	if e.payload == nil || !e.payload.hasTool {
		return nil
	}
	return CloneRawMessage(e.payload.toolInput)
}

// ToolError reports whether the optional tool result failed.
func (e AgentEvent) ToolError() bool {
	return e.payload != nil && e.payload.hasTool && e.payload.toolFailed
}

// ToolErrorDetail returns the optional tool-result failure detail.
func (e AgentEvent) ToolErrorDetail() string {
	if e.payload == nil || !e.payload.hasTool {
		return ""
	}
	return e.payload.toolErrorDetail
}

// HasToolPayload reports whether typed tool metadata is present on the event.
func (e AgentEvent) HasToolPayload() bool {
	return e.payload != nil && e.payload.hasTool
}
