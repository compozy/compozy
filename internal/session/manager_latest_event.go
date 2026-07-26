package session

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/agh/internal/store"
)

// LatestSessionEventByType returns the newest persisted event for one session event type.
func (m *Manager) LatestSessionEventByType(
	ctx context.Context,
	sessionID string,
	eventType string,
) (*store.SessionEvent, error) {
	if ctx == nil {
		return nil, errors.New("session: latest event context is required")
	}
	targetType := strings.TrimSpace(eventType)
	if targetType == "" {
		return nil, errors.New("session: latest event type is required")
	}
	events, err := m.Events(ctx, sessionID, store.EventQuery{Type: targetType, Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, nil
	}
	event := events[0]
	return &event, nil
}
