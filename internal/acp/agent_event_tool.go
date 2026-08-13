package acp

import (
	"encoding/json"
	"strings"

	commandpkg "github.com/compozy/compozy/internal/command"
)

type agentEventPayload struct {
	eventID          string
	messageID        string
	requestID        string
	toolName         string
	toolKind         string
	toolInput        json.RawMessage
	toolErrorDetail  string
	toolFailed       bool
	toolPrechecked   bool
	hasTool          bool
	promptRuntime    *PromptRuntime
	skillInvocations []commandpkg.Invocation
}

func (e AgentEvent) clonePayload() *agentEventPayload {
	if e.payload == nil {
		return &agentEventPayload{}
	}
	cloned := *e.payload
	cloned.toolInput = CloneRawMessage(e.payload.toolInput)
	cloned.promptRuntime = ClonePromptRuntime(e.payload.promptRuntime)
	cloned.skillInvocations = append([]commandpkg.Invocation(nil), e.payload.skillInvocations...)
	return &cloned
}

func normalizeAgentEventPayload(payload *agentEventPayload) *agentEventPayload {
	if payload == nil || payload.eventID == "" && payload.messageID == "" && payload.requestID == "" &&
		!payload.hasTool && !payload.toolPrechecked && payload.promptRuntime == nil &&
		len(payload.skillInvocations) == 0 {
		return nil
	}
	return payload
}

// WithSkillInvocations returns an event carrying isolated admitted slash metadata.
func (e AgentEvent) WithSkillInvocations(invocations []commandpkg.Invocation) AgentEvent {
	payload := e.clonePayload()
	payload.skillInvocations = append([]commandpkg.Invocation(nil), invocations...)
	e.payload = normalizeAgentEventPayload(payload)
	return e
}

// SkillInvocations returns an isolated copy of admitted slash metadata.
func (e AgentEvent) SkillInvocations() []commandpkg.Invocation {
	if e.payload == nil {
		return nil
	}
	return append([]commandpkg.Invocation(nil), e.payload.skillInvocations...)
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

// WithRequestID returns an event carrying its externally addressable request identity.
func (e AgentEvent) WithRequestID(requestID string) AgentEvent {
	payload := e.clonePayload()
	payload.requestID = strings.TrimSpace(requestID)
	e.payload = normalizeAgentEventPayload(payload)
	return e
}

// RequestIDValue returns the externally addressable request identity.
func (e AgentEvent) RequestIDValue() string {
	if e.payload == nil {
		return ""
	}
	return e.payload.requestID
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

// WithToolPrechecked marks tool admission as already enforced at the ACP boundary.
func (e AgentEvent) WithToolPrechecked() AgentEvent {
	payload := e.clonePayload()
	payload.toolPrechecked = true
	e.payload = normalizeAgentEventPayload(payload)
	return e
}

// ToolPrechecked reports whether tool admission was already enforced.
func (e AgentEvent) ToolPrechecked() bool {
	return e.payload != nil && e.payload.toolPrechecked
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
