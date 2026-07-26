package session

import (
	"context"

	"errors"
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/transcript"
)

func (m *Manager) sendPromptPumpEvent(
	ctx context.Context,
	deliveryCtx context.Context,
	out chan<- acp.AgentEvent,
	loop *promptPumpLoopState,
	normalized acp.AgentEvent,
	runtimeEvent bool,
) bool {
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
	normalized.Goal = acp.CloneGoalPromptMeta(event.Goal)
	if strings.TrimSpace(normalized.TurnID) == "" {
		normalized.TurnID = turnID
	}
	if normalized.Timestamp.IsZero() {
		normalized.Timestamp = m.now()
	}
	if session != nil {
		if normalized.Goal == nil {
			normalized.Goal = goalPromptMetaFromPromptMeta(session.CurrentPromptMeta())
		}
		info := session.Info()
		if strings.TrimSpace(normalized.SessionID) == "" {
			normalized.SessionID = info.ACPSessionID
		}
	}
	return normalized
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
	recorder := session.recorderHandle()
	if recorder == nil {
		return errors.New("session: event recorder is not available")
	}
	event = m.enrichRecordedAgentEvent(session, event)

	payload, err := marshalAgentEventPreservingAuthoredText(event, authoredText)
	if err != nil {
		return err
	}

	m.dispatchEventPreRecord(ctx, session, event, payload)

	persisted, err := recordPersistedSessionEvent(ctx, recorder, store.SessionEvent{
		TurnID:    event.TurnID,
		Type:      event.Type,
		AgentName: session.Info().AgentName,
		Content:   payload,
		Timestamp: event.Timestamp,
	})
	if err != nil {
		return err
	}

	m.recordPromptTokenUsageProjection(ctx, session, recorder, event)
	if err := m.persistAdvertisedCommandsFromEvent(ctx, session, event); err != nil {
		return err
	}

	m.dispatchEventPostRecord(ctx, session, event, payload, persisted.Sequence)
	m.dispatchSessionMessagePersisted(ctx, session, event, persisted, payload)
	m.publishSessionEvent(ctx, session, persisted)

	return nil
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

	enriched := event
	if enriched.Goal == nil {
		enriched.Goal = goalPromptMetaFromPromptMeta(session.CurrentPromptMeta())
	} else {
		enriched.Goal = acp.CloneGoalPromptMeta(enriched.Goal)
	}
	correlation := enriched.Normalize()
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
