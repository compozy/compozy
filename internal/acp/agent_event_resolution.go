package acp

import (
	"strings"
	"time"
)

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

// WithExpiresAt returns an event carrying an interaction expiry.
func (e AgentEvent) WithExpiresAt(expiresAt time.Time) AgentEvent {
	payload := e.clonePayload()
	payload.expiresAt = new(expiresAt)
	e.payload = normalizeAgentEventPayload(payload)
	return e
}

// ExpiresAtValue returns an isolated interaction expiry.
func (e AgentEvent) ExpiresAtValue() *time.Time {
	if e.payload == nil || e.payload.expiresAt == nil {
		return nil
	}
	return new(*e.payload.expiresAt)
}
