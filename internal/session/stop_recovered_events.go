package session

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) recordRecoveredStopEvent(
	ctx context.Context, id string, settlement *recoveredStopSettlement, eventType string, outcome StopOutcome,
) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("%w: persist recovered stop event for %s: %w", ErrRecoveryPersistence, id, err)
		}
	}()
	if eventType == EventTypeSessionStopped && settlement.receipt.TerminalEvent != nil {
		return m.replayActiveStopEvent(ctx, id, settlement.receipt.TerminalEvent)
	}
	turnID := settlement.turnID
	meta, err := m.readMetaWithContext(ctx, id)
	if err != nil {
		return err
	}
	payload := stopEventPayload{
		WorkspaceID: meta.WorkspaceID,
		SessionID:   id,
		TurnID:      turnID,
		Scope:       stopEventScopeSession,
		Phase:       outcome.Phase,
		ElapsedMS:   outcome.Elapsed.Milliseconds(),
		Cause:       outcome.Cause,
	}
	if eventType == events.SessionStopVerificationFailed {
		payload.ReasonCode = StopVerificationFailedCode
	}
	if actor := settlement.actorID; actor != "" {
		payload.ActorID, payload.ActorKind = actor, actingSessionActorKind
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	event := acp.AgentEvent{
		Type:      eventType,
		TurnID:    turnID,
		SessionID: stringValue(meta.ACPSessionID),
		Raw:       raw,
		Timestamp: settlement.startedAt.Add(outcome.Elapsed),
	}
	if eventType == EventTypeSessionStopped {
		event.StopReason = string(sessionMetaStopReason(&meta))
	}
	content, err := marshalAgentEvent(event)
	if err != nil {
		return err
	}
	persisted, err := m.appendDurableSessionEvent(ctx, id, store.SessionEvent{
		ID:        fmt.Sprintf("recovered-stop:%s:%s:%s", turnID, eventType, outcome.Phase),
		SessionID: id, TurnID: turnID, Type: eventType, AgentName: meta.AgentName,
		Content: content, Timestamp: event.Timestamp,
	})
	if err != nil {
		return err
	}
	m.publishSessionEventByID(ctx, id, persisted)
	m.notifyAgentEventFromInfo(ctx, sessionInfoFromMeta(meta), event)
	return nil
}
