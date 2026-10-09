package acp

// WithProviderToolMetadata preserves provider attribution independently of tool display names.
func (e AgentEvent) WithProviderToolMetadata(parentToolCallID, providerToolName, status string) AgentEvent {
	payload := e.clonePayload()
	payload.parentToolCallID = parentToolCallID
	payload.providerToolName = providerToolName
	payload.toolStatus = status
	e.payload = normalizeAgentEventPayload(payload)
	return e
}

// ParentToolCallID identifies the provider tool whose subagent produced this event.
func (e AgentEvent) ParentToolCallID() string {
	if e.payload == nil {
		return ""
	}
	return e.payload.parentToolCallID
}

// ProviderToolName returns the provider's tool identity, independent of its title.
func (e AgentEvent) ProviderToolName() string {
	if e.payload == nil {
		return ""
	}
	return e.payload.providerToolName
}

// ToolStatus returns the provider's observed tool lifecycle status.
func (e AgentEvent) ToolStatus() string {
	if e.payload == nil {
		return ""
	}
	return e.payload.toolStatus
}
