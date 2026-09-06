package session

import (
	"context"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
)

// SetWorkSignalSources installs the daemon's authoritative inspection adapters.
func (m *Manager) SetWorkSignalSources(sources ...WorkSignalSource) {
	m.workSignals.Register(sources...)
}

func (m *Manager) progressSignals(_ context.Context, id string) ([]WorkSignal, error) {
	target, ok := m.Get(id)
	if !ok {
		return nil, ErrSessionNotFound
	}
	target.mu.RLock()
	at := target.supervisionProgressAt
	target.mu.RUnlock()
	if at.IsZero() {
		return nil, nil
	}
	return []WorkSignal{{
		Kind: WorkSignalAgentProgress, Since: at,
		ValidUntil: at.Add(2 * m.supervision.ActivityHeartbeatInterval),
	}}, nil
}

func (m *Manager) recordWorkProgress(target *Session, at time.Time) {
	if target == nil {
		return
	}
	now := m.now().UTC()
	if at.IsZero() || at.After(now) {
		at = now
	}
	target.mu.Lock()
	if !at.After(target.supervisionProgressAt) {
		target.mu.Unlock()
		return
	}
	target.supervisionProgressAt = at
	target.supervisionQuietSince = time.Time{}
	target.supervisionStopAt = time.Time{}
	cleared := target.supervisionState != nil && target.supervisionState.QuietWarning != nil
	if cleared {
		target.supervisionState = CloneSupervisionState(target.supervisionState)
		target.supervisionState.QuietWarning = nil
	}
	target.mu.Unlock()
	if cleared {
		m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventUpserted, target.Info()))
	}
}

func (m *Manager) recordPersistedWorkProgress(target *Session, event acp.AgentEvent) {
	if strings.HasPrefix(event.Type, "session.supervision_") {
		return
	}
	if kind, _, _, _, _ := activityFromEvent(event); kind != "" {
		m.recordWorkProgress(target, m.now())
	}
}
