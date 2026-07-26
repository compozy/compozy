package session

import (
	"context"
	"errors"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/transcript"
)

func (m *Manager) handlePromptPumpChunkBatch(
	ctx context.Context,
	deliveryCtx context.Context,
	session *Session,
	turnState *promptTurnDispatchState,
	out chan<- acp.AgentEvent,
	loop *promptPumpLoopState,
	events []acp.AgentEvent,
) (*store.SessionFailure, string, bool) {
	normalized := make([]acp.AgentEvent, 0, len(events))
	for _, event := range events {
		next, skip := m.preparePromptPumpEventForDelivery(ctx, session, turnState, loop, event, false)
		if skip {
			continue
		}
		normalized = append(normalized, next)
	}
	if len(normalized) == 0 {
		return nil, "", false
	}

	if loop.activity != nil {
		for _, event := range normalized {
			loop.activity.observeEvent(event)
		}
	}
	if err := m.recordPromptEventBatch(ctx, session, normalized); err != nil {
		failure, errorText := m.promptPersistenceFailure(session, turnState.turnID, err)
		return failure, errorText, true
	}

	for _, event := range normalized {
		loop.fileMutations.Observe(event)
		m.emitFileMutationMarkerBeforeTerminalNotification(ctx, session, turnState, loop, event)
		m.notifyManagedPromptEvent(ctx, session, turnState, event)
		m.scheduleCompactionFromUsage(session, event)
		if kind, summary, evidence, ok := promptTranscriptMarker(event); ok {
			m.emitTranscriptMarker(ctx, session, turnState.turnID, kind, summary, evidence)
		}
		fatalPromptFailure := promptFatalFailure(event)
		if stop := m.sendPromptPumpEvent(ctx, deliveryCtx, out, loop, event, false); stop {
			return fatalPromptFailure, event.Error, true
		}
		if stop := m.finishPromptTurnIfNeeded(ctx, turnState, loop, event); stop {
			return fatalPromptFailure, event.Error, true
		}
	}
	return nil, "", false
}

func (m *Manager) recordPromptEventBatch(
	ctx context.Context,
	session *Session,
	events []acp.AgentEvent,
) error {
	recorder := session.recorderHandle()
	if recorder == nil {
		return errors.New("session: event recorder is not available")
	}

	storeEvents := make([]store.SessionEvent, 0, len(events))
	for _, event := range events {
		enriched := m.enrichRecordedAgentEvent(session, event)
		payload, err := marshalAgentEvent(enriched)
		if err != nil {
			return err
		}
		m.dispatchEventPreRecord(ctx, session, enriched, payload)
		storeEvents = append(storeEvents, store.SessionEvent{
			TurnID:    enriched.TurnID,
			Type:      enriched.Type,
			AgentName: session.Info().AgentName,
			Content:   payload,
			Timestamp: enriched.Timestamp,
		})
	}

	persisted, err := recordPersistedSessionEventBatch(ctx, recorder, storeEvents)
	if err != nil {
		return err
	}
	for _, event := range events {
		m.recordPromptTokenUsageProjection(ctx, session, recorder, event)
	}
	for _, event := range events {
		if err := m.persistAdvertisedCommandsFromEvent(ctx, session, event); err != nil {
			return err
		}
	}

	for _, event := range persisted {
		m.dispatchPersistedRecordedEvent(ctx, session, event)
	}
	return nil
}

func (m *Manager) dispatchPersistedRecordedEvent(
	ctx context.Context,
	session *Session,
	persisted store.SessionEvent,
) {
	event, err := transcript.UnmarshalAgentEvent(persisted.Content)
	if err != nil {
		m.sessionLogger(session).Warn(
			"session: unmarshal persisted prompt event failed",
			"sequence",
			persisted.Sequence,
			"error",
			err,
		)
		event = acp.AgentEvent{}
	}
	if event.Type == "" {
		event.Type = persisted.Type
	}
	if event.TurnID == "" {
		event.TurnID = persisted.TurnID
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = persisted.Timestamp
	}

	m.dispatchEventPostRecord(ctx, session, event, persisted.Content, persisted.Sequence)
	m.dispatchSessionMessagePersisted(ctx, session, event, persisted, persisted.Content)
	m.publishSessionEvent(ctx, session, persisted)
}

type persistedSessionEventBatchRecorder interface {
	RecordPersistedBatch(context.Context, []store.SessionEvent) ([]store.SessionEvent, error)
}

func recordPersistedSessionEventBatch(
	ctx context.Context,
	recorder EventRecorder,
	events []store.SessionEvent,
) ([]store.SessionEvent, error) {
	if batchRecorder, ok := recorder.(persistedSessionEventBatchRecorder); ok {
		return batchRecorder.RecordPersistedBatch(ctx, events)
	}
	persisted := make([]store.SessionEvent, 0, len(events))
	for _, event := range events {
		row, err := recordPersistedSessionEvent(ctx, recorder, event)
		if err != nil {
			return nil, err
		}
		persisted = append(persisted, row)
	}
	return persisted, nil
}
