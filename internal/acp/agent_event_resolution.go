package acp

import "strings"

// WithResolvedBy returns an event carrying the normalized interaction actor.
func (e AgentEvent) WithResolvedBy(resolvedBy string) AgentEvent {
	payload := e.clonePayload()
	payload.resolvedBy = strings.TrimSpace(resolvedBy)
	e.payload = normalizeAgentEventPayload(payload)
	return e
}

// ResolvedByValue returns the normalized actor that resolved an interaction.
func (e AgentEvent) ResolvedByValue() string {
	if e.payload == nil {
		return ""
	}
	return e.payload.resolvedBy
}
