package acp

import (
	"encoding/json"
	"strings"
)

type agentEventPayload struct {
	eventID         string
	messageID       string
	toolName        string
	toolKind        string
	toolInput       json.RawMessage
	toolErrorDetail string
	toolFailed      bool
	hasTool         bool
	promptRuntime   *PromptRuntime
}

func (e AgentEvent) clonePayload() *agentEventPayload {
	if e.payload == nil {
		return &agentEventPayload{}
	}
	cloned := *e.payload
	cloned.toolInput = CloneRawMessage(e.payload.toolInput)
	cloned.promptRuntime = ClonePromptRuntime(e.payload.promptRuntime)
	return &cloned
}

func normalizeAgentEventPayload(payload *agentEventPayload) *agentEventPayload {
	if payload == nil || payload.eventID == "" && payload.messageID == "" &&
		!payload.hasTool && payload.promptRuntime == nil {
		return nil
	}
	return payload
}

// WithEventID returns an event carrying its stable durable event identity.
func (e AgentEvent) WithEventID(eventID string) AgentEvent {
	payload := e.clonePayload()
	payload.eventID = strings.TrimSpace(eventID)
	e.payload = normalizeAgentEventPayload(payload)
	return e
}

// EventIDValue returns the stable durable identity reserved for the event.
func (e *AgentEvent) EventIDValue() string {
	if e == nil || e.payload == nil {
		return ""
	}
	return e.payload.eventID
}

// WithMessageID returns an event carrying the normalized authored-message identity.
func (e AgentEvent) WithMessageID(messageID string) AgentEvent {
	payload := e.clonePayload()
	payload.messageID = strings.TrimSpace(messageID)
	e.payload = normalizeAgentEventPayload(payload)
	return e
}

// MessageIDValue returns the normalized identity for an authored user message.
func (e *AgentEvent) MessageIDValue() string {
	if e == nil || e.payload == nil {
		return ""
	}
	return e.payload.messageID
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
