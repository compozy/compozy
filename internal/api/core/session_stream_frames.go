package core

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

func (h *BaseHandlers) writeGoalSnapshotChangedEvents(
	ctx context.Context,
	writer FlushWriter,
	sessionID string,
	afterSequence int64,
	events []store.SessionEvent,
) error {
	if events == nil {
		latest, err := h.Sessions.LatestSessionEventByType(ctx, sessionID, session.EventTypeGoalSnapshotChanged)
		if err != nil {
			return fmt.Errorf("query latest Goal snapshot event: %w", err)
		}
		if latest != nil {
			events = []store.SessionEvent{*latest}
		}
	}
	for _, event := range events {
		if event.Sequence <= afterSequence || event.Type != session.EventTypeGoalSnapshotChanged {
			continue
		}
		content := []byte(event.Content)
		if !json.Valid(content) {
			return fmt.Errorf("decode Goal snapshot event %d: invalid JSON", event.Sequence)
		}
		// This side signal precedes transcript catch-up and cannot advance its resume cursor.
		if err := WriteSSE(writer, SSEMessage{
			Name: contract.SessionStreamEventGoalSnapshotChanged,
			Data: json.RawMessage(append([]byte(nil), content...)),
		}); err != nil {
			return fmt.Errorf("write Goal snapshot event %d: %w", event.Sequence, err)
		}
	}
	return nil
}

func (h *BaseHandlers) writeSessionCommandsChanged(
	ctx context.Context,
	writer FlushWriter,
	sessionID string,
	previousRevision string,
) (string, error) {
	manager, ok := h.Sessions.(SessionCommandCatalogManager)
	if !ok {
		return previousRevision, nil
	}
	catalog, err := manager.CommandCatalog(ctx, sessionID)
	if err != nil {
		h.sessionStreamLogger().WarnContext(
			ctx,
			"api: session command catalog unavailable; continuing transcript stream",
			"session_id", sessionID,
			"error", err,
		)
		return previousRevision, nil
	}
	if previousRevision == "" || catalog.Revision == previousRevision {
		return catalog.Revision, nil
	}
	if err := WriteSSE(writer, SSEMessage{
		Name: contract.SessionStreamEventCommandsChanged,
		Data: contract.SessionCommandsChangedPayload{SessionID: sessionID, Revision: catalog.Revision},
	}); err != nil {
		return previousRevision, fmt.Errorf("write session command catalog change: %w", err)
	}
	return catalog.Revision, nil
}

func (h *BaseHandlers) writeConsumerDegraded(
	writer FlushWriter,
	sessionID string,
	cursor int64,
	event store.SessionEvent,
) error {
	payload := contract.SessionConsumerDegradedPayload{SessionID: sessionID, AfterSequence: cursor, Refresh: true}
	var marker contract.SessionConsumerDegradedPayload
	if err := json.Unmarshal([]byte(event.Content), &marker); err != nil {
		return fmt.Errorf("decode consumer degradation: %w", err)
	}
	payload.ThroughSequence = marker.ThroughSequence
	// No SSE id: a delivery diagnostic cannot advance the durable replay cursor.
	return WriteSSE(writer, SSEMessage{Name: contract.SessionStreamEventConsumerDegraded, Data: payload})
}
