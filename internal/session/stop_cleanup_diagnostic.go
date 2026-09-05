package session

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/store"
)

// recordStoppedCleanupFailure preserves auxiliary diagnostics after the normal
// recorder closes without changing the verified terminal classification.
func (m *Manager) recordStoppedCleanupFailure(
	ctx context.Context, session *Session, step string, cleanupErr error,
) error {
	diagnosticCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultLifecycleTimeout)
	defer cancel()
	id, err := m.newPromptTurnID()
	if err != nil {
		return err
	}
	info := session.Info()
	event := m.normalizeEvent(session, id, acp.AgentEvent{
		Type: acp.EventTypeRuntimeWarning,
		Text: diagnostics.RedactAndBound(
			fmt.Sprintf("Session stopped; %s cleanup failed: %v", step, cleanupErr),
			maxSessionFailureSummaryBytes,
		),
		Timestamp: m.now(),
	})
	payload, err := marshalAgentEvent(event)
	if err != nil {
		return err
	}
	persisted, err := m.appendStoredSessionEvent(diagnosticCtx, session.ID, store.SessionEvent{
		ID: id, SessionID: session.ID, TurnID: event.TurnID, Type: event.Type,
		AgentName: info.AgentName, Content: payload, Timestamp: event.Timestamp,
	})
	if err != nil {
		return fmt.Errorf("session: persist %s cleanup diagnostic: %w", step, err)
	}
	m.publishSessionEventByID(diagnosticCtx, session.ID, persisted)
	return nil
}
