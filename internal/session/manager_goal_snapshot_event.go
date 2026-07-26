package session

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/agh/internal/store"
)

// AppendSessionEventIfAbsent persists and publishes one deterministic Goal projection event.
func (m *Manager) AppendSessionEventIfAbsent(ctx context.Context, event GoalEvent) error {
	if m == nil {
		return errors.New("session: manager is required")
	}
	if ctx == nil {
		return errors.New("session: append Goal event context is required")
	}
	if err := event.Validate(); err != nil {
		return err
	}
	target := strings.TrimSpace(event.SessionID)
	if _, err := m.Status(ctx, target); err != nil {
		return err
	}
	persisted, err := m.appendDurableSessionEvent(ctx, target, store.SessionEvent{
		ID:        strings.TrimSpace(event.EventID),
		SessionID: target,
		TurnID:    strings.TrimSpace(event.SyntheticTurnID),
		Type:      EventTypeGoalSnapshotChanged,
		AgentName: strings.TrimSpace(event.AgentName),
		Content:   string(event.Content),
		Timestamp: event.CreatedAt.UTC(),
	})
	if err != nil {
		return err
	}
	m.publishSessionEventByID(ctx, target, persisted)
	return nil
}
