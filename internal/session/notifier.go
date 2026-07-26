package session

import (
	"context"

	"github.com/compozy/agh/internal/acp"
)

func (m *Manager) notifyAgentEvent(ctx context.Context, session *Session, event any) {
	if m == nil || m.notifier == nil || session == nil {
		return
	}

	enriched := m.enrichNotifiedAgentEvent(session, event)
	if aware, ok := m.notifier.(AgentEventNotifier); ok {
		aware.OnAgentEventForSession(ctx, session, enriched)
		return
	}

	m.notifier.OnAgentEvent(ctx, session.ID, enriched)
}

func (m *Manager) enrichNotifiedAgentEvent(session *Session, event any) any {
	switch typed := event.(type) {
	case acp.AgentEvent:
		return m.enrichRecordedAgentEvent(session, typed)
	case *acp.AgentEvent:
		if typed == nil {
			return event
		}
		enriched := m.enrichRecordedAgentEvent(session, *typed)
		return &enriched
	default:
		return event
	}
}
