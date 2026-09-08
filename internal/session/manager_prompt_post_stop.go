package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

var errPromptEventDiscardedAfterStop = errors.New("session: prompt event discarded after stop")

func (m *Manager) discardPromptEventAfterStop(
	ctx context.Context, session *Session, turnID, eventType string, err error,
) bool {
	// Finalization closes the recorder before marking the runtime stopped. An
	// event already inside a pre-record hook can resume after that boundary.
	return errors.Is(err, errPromptEventDiscardedAfterStop) ||
		(errors.Is(err, store.ErrClosed) && m.discardStoppedPromptEvent(ctx, session, turnID, eventType))
}

func (m *Manager) discardStoppedPromptEvent(ctx context.Context, session *Session, turnID, eventType string) bool {
	if session.Info().State != StateStopped {
		return false
	}
	markerCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultLifecycleTimeout)
	defer cancel()
	if err := m.recordPostStopMarker(markerCtx, session, turnID, eventType); err != nil {
		m.sessionLogger(session).Warn("session: record discarded post-stop event failed", "error", err)
	}
	return true
}

func (m *Manager) recordPostStopMarker(ctx context.Context, session *Session, turnID, eventType string) error {
	marker, err := transcript.NewMarker(
		transcript.MarkerPostStop,
		"Late agent output discarded after the session stopped.",
		m.now(),
		map[string]any{transcriptMarkerEvidenceEventTypeKey: eventType},
	)
	if err != nil {
		return err
	}
	event, err := marker.AgentEvent(session.Info().ACPSessionID, turnID)
	if err != nil {
		return err
	}
	payload, err := marshalAgentEvent(event)
	if err != nil {
		return err
	}
	// The ordinary recorder has already closed. Reuse the durable append owner
	// and one stable marker ID per turn without applying agent state projections.
	persisted, err := m.appendDurableSessionEvent(ctx, session.ID, store.SessionEvent{
		ID:        fmt.Sprintf("post-stop:%s:%s", session.ID, turnID),
		SessionID: session.ID,
		TurnID:    turnID,
		Type:      event.Type,
		AgentName: session.Info().AgentName,
		Content:   payload,
		Timestamp: event.Timestamp,
	})
	if err != nil {
		return err
	}
	m.publishSessionEventByID(ctx, session.ID, persisted)
	// The transcript stream has ended; mounted readers reconcile through the catalog.
	m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventUpserted, session.Info()))
	return nil
}
