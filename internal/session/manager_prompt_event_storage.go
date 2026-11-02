package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

func (m *Manager) sendPromptPumpEvent(
	ctx context.Context,
	deliveryCtx context.Context,
	out chan<- acp.AgentEvent,
	loop *promptPumpLoopState,
	normalized acp.AgentEvent,
	runtimeEvent bool,
) bool {
	if out == nil {
		ackPromptPumpRuntimeEvent(loop, normalized, runtimeEvent)
		return false
	}
	if ctx == nil {
		ctx = m.fallbackLifecycleContext()
	}
	if deliveryCtx == nil {
		deliveryCtx = ctx
	}
	select {
	case <-deliveryCtx.Done():
		ackPromptPumpRuntimeEvent(loop, normalized, runtimeEvent)
		return false
	default:
	}
	select {
	case out <- normalized:
	case <-deliveryCtx.Done():
		ackPromptPumpRuntimeEvent(loop, normalized, runtimeEvent)
		return false
	case <-ctx.Done():
		return true
	}
	ackPromptPumpRuntimeEvent(loop, normalized, runtimeEvent)
	return false
}

func ackPromptPumpRuntimeEvent(loop *promptPumpLoopState, normalized acp.AgentEvent, runtimeEvent bool) {
	if runtimeEvent && loop.activity != nil {
		loop.activity.ackPromptDeadlineWarning(normalized)
	}
}

func (m *Manager) normalizeEvent(session *Session, turnID string, event acp.AgentEvent) acp.AgentEvent {
	normalized := event
	normalized = normalized.WithGoalPromptMeta(event.GoalPromptMeta())
	if strings.TrimSpace(normalized.TurnID) == "" {
		normalized.TurnID = turnID
	}
	normalized = promptRuntimeFallbackForEvent(session, normalized)
	if normalized.Timestamp.IsZero() {
		normalized.Timestamp = m.now()
	}
	if session != nil {
		if normalized.GoalPromptMeta() == nil {
			normalized = normalized.WithGoalPromptMeta(goalPromptMetaFromPromptMeta(session.CurrentPromptMeta()))
		}
		info := session.Info()
		if strings.TrimSpace(normalized.SessionID) == "" {
			normalized.SessionID = info.ACPSessionID
		}
	}
	return normalized
}

func promptRuntimeFallbackForEvent(session *Session, event acp.AgentEvent) acp.AgentEvent {
	event = event.WithPromptRuntime(event.PromptRuntimeSnapshot())
	if session == nil || event.PromptRuntimeSnapshot() != nil {
		return event
	}
	return event.WithPromptRuntime(promptRuntimeFromSelection(session.promptRuntimeForEventTurn(event.TurnID)))
}

func (m *Manager) recordEvent(ctx context.Context, session *Session, event acp.AgentEvent) error {
	return m.recordEventWithAuthoredText(ctx, session, event, "")
}

func (m *Manager) recordEventWithAuthoredText(
	ctx context.Context,
	session *Session,
	event acp.AgentEvent,
	authoredText string,
) error {
	return m.recordEventWithWriter(ctx, session, event, authoredText, recordPersistedSessionEvent)
}

func (m *Manager) recordEventWithWriter(
	ctx context.Context,
	session *Session,
	event acp.AgentEvent,
	authoredText string,
	write func(context.Context, EventRecorder, store.SessionEvent) (store.SessionEvent, error),
) error {
	event = m.enrichRecordedAgentEvent(session, event)
	if event.Type == acp.EventTypeCompaction || event.Type == acp.EventTypeSystem {
		event = transcript.RedactAgentEvent(event)
	}
	compaction, skip, err := m.prepareCompactionSnapshot(ctx, session, &event)
	if err != nil {
		return err
	}
	if skip {
		return m.dispatchObservedCompaction(ctx, session, event, compaction)
	}
	attentionCommitted, err := m.applyAttentionAgentEvent(ctx, session, event)
	if err != nil {
		return fmt.Errorf("session: persist canonical attention event: %w", err)
	}
	recorder := session.recorderHandle()
	if recorder == nil {
		return m.handleAttentionTranscriptFailure(
			ctx,
			session,
			event,
			attentionCommitted,
			errors.New("session: event recorder is not available"),
		)
	}

	payload, err := marshalAgentEventPreservingAuthoredText(event, authoredText)
	if err != nil {
		return m.handleAttentionTranscriptFailure(ctx, session, event, attentionCommitted, err)
	}

	m.dispatchEventPreRecord(ctx, session, event, payload)

	persisted, err := write(ctx, recorder, store.SessionEvent{
		ID:        strings.TrimSpace(event.EventIDValue()),
		TurnID:    event.TurnID,
		Type:      event.Type,
		AgentName: session.Info().AgentName,
		Content:   payload,
		Timestamp: event.Timestamp,
	})
	if err != nil {
		return m.handleAttentionTranscriptFailure(ctx, session, event, attentionCommitted, err)
	}

	if err := m.reportExhaustedContext(ctx, session.ID, event); err != nil {
		return fmt.Errorf("session: report exhausted context: %w", err)
	}
	m.recordPromptTokenUsageProjection(ctx, session, recorder, event)
	if err := m.persistAdvertisedCommandsFromEvent(ctx, session, event); err != nil {
		return m.handleAttentionTranscriptFailure(ctx, session, event, attentionCommitted, err)
	}

	m.publishNativeSubagentEvent(ctx, session, persisted, event)
	m.dispatchEventPostRecord(ctx, session, event, payload, persisted.Sequence)
	m.dispatchSessionMessagePersisted(ctx, session, event, persisted, payload)
	m.publishSessionEvent(ctx, session, persisted)
	if !attentionCommitted {
		m.publishSessionCatalogWakeForEvent(session, event)
	}

	return m.dispatchObservedCompaction(ctx, session, event, compaction)
}

func (m *Manager) handleAttentionTranscriptFailure(
	ctx context.Context,
	target *Session,
	event acp.AgentEvent,
	attentionCommitted bool,
	err error,
) error {
	if !attentionCommitted {
		return err
	}
	m.sessionLogger(target).WarnContext(
		ctx,
		"session: append attention transcript event failed after canonical commit",
		"event_type", event.Type,
		"request_id", event.RequestIDValue(),
		"error", err,
	)
	return nil
}

func (m *Manager) recordPromptInputWithAuthoredText(
	ctx context.Context,
	session *Session,
	event acp.AgentEvent,
	authoredText string,
	eventID string,
	idempotent bool,
) (bool, error) {
	recorder := session.recorderHandle()
	if recorder == nil {
		return false, errors.New("session: event recorder is not available")
	}
	if !idempotent {
		event = m.enrichRecordedAgentEvent(session, event)
	}
	payload, err := marshalAgentEventPreservingAuthoredText(event, authoredText)
	if err != nil {
		return false, err
	}
	persistedEventID, err := promptInputEventID(eventID)
	if err != nil {
		return false, err
	}
	m.dispatchEventPreRecord(ctx, session, event, payload)
	candidate := store.SessionEvent{
		ID: persistedEventID, TurnID: event.TurnID, Type: event.Type,
		AgentName: session.Info().AgentName, Content: payload, Timestamp: event.Timestamp,
	}
	var persisted store.SessionEvent
	inserted := true
	if idempotent {
		appender, ok := recorder.(idempotentSessionEventAppender)
		if !ok {
			return false, errors.New("session: event recorder does not support idempotent append")
		}
		persisted, inserted, err = appender.AppendEventIfAbsent(ctx, candidate)
	} else {
		persisted, err = recordPersistedSessionEvent(ctx, recorder, candidate)
	}
	if err != nil {
		return false, err
	}
	if !inserted {
		return false, nil
	}
	if err := m.reportExhaustedContext(ctx, session.ID, event); err != nil {
		return false, fmt.Errorf("session: report exhausted context: %w", err)
	}
	m.recordPromptTokenUsageProjection(ctx, session, recorder, event)
	if err := m.persistAdvertisedCommandsFromEvent(ctx, session, event); err != nil {
		return false, err
	}
	m.dispatchEventPostRecord(ctx, session, event, payload, persisted.Sequence)
	m.dispatchSessionMessagePersisted(ctx, session, event, persisted, payload)
	m.publishSessionEvent(ctx, session, persisted)
	return true, nil
}

type persistedSessionEventRecorder interface {
	RecordPersisted(context.Context, store.SessionEvent) (store.SessionEvent, error)
}

func recordPersistedSessionEvent(
	ctx context.Context,
	recorder EventRecorder,
	event store.SessionEvent,
) (store.SessionEvent, error) {
	if persistedRecorder, ok := recorder.(persistedSessionEventRecorder); ok {
		return persistedRecorder.RecordPersisted(ctx, event)
	}
	if err := recorder.Record(ctx, event); err != nil {
		return store.SessionEvent{}, err
	}
	return event, nil
}

func marshalAgentEvent(event acp.AgentEvent) (string, error) {
	data, err := transcript.MarshalAgentEvent(event)
	if err != nil {
		return "", fmt.Errorf("session: marshal agent event: %w", err)
	}
	return data, nil
}

func marshalPromptInputEvent(event acp.AgentEvent, authoredText string) (string, error) {
	data, err := transcript.MarshalPromptInputEvent(event, authoredText)
	if err != nil {
		return "", fmt.Errorf("session: marshal prompt input event: %w", err)
	}
	return data, nil
}

func marshalAgentEventPreservingAuthoredText(event acp.AgentEvent, authoredText string) (string, error) {
	if strings.TrimSpace(authoredText) == "" || authoredText == event.Text {
		return marshalAgentEvent(event)
	}
	return marshalPromptInputEvent(event, authoredText)
}

func (m *Manager) enrichRecordedAgentEvent(session *Session, event acp.AgentEvent) acp.AgentEvent {
	if session == nil {
		return event
	}

	enriched := promptRuntimeFallbackForEvent(session, event)
	if enriched.GoalPromptMeta() == nil {
		enriched = enriched.WithGoalPromptMeta(goalPromptMetaFromPromptMeta(session.CurrentPromptMeta()))
	} else {
		enriched = enriched.WithGoalPromptMeta(enriched.GoalPromptMeta())
	}
	correlation := enriched.Normalize()
	if identity, ok := m.activePromptRunSnapshot(session.ID); ok {
		if correlation.RunID == "" {
			correlation.RunID = identity.RunID
		}
		if correlation.Generation == 0 {
			correlation.Generation = identity.Generation
		}
	}
	meta := session.CurrentPromptMeta().Normalize()
	if meta.Synthetic != nil {
		synthetic := meta.Synthetic.Normalize()
		if correlation.TaskID == "" {
			correlation.TaskID = synthetic.TaskID
		}
		if correlation.RunID == "" {
			correlation.RunID = synthetic.TaskRunID
		}
		if correlation.WorkflowID == "" {
			correlation.WorkflowID = synthetic.WorkflowID
		}
		if correlation.ClaimTokenHash == "" {
			correlation.ClaimTokenHash = synthetic.ClaimTokenHash
		}
		if correlation.CoordinatorSessionID == "" {
			correlation.CoordinatorSessionID = synthetic.CoordinatorSessionID
		}
		if correlation.SchedulerReason == "" {
			correlation.SchedulerReason = synthetic.Reason
		}
	}

	enriched.EventCorrelation = correlation.Normalize()
	return enriched
}
