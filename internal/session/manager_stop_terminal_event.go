package session

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

func (m *Manager) recordSessionStoppedEvent(
	ctx context.Context,
	session *Session,
	waitErr error,
	promptOwnsTerminalFailure bool,
) error {
	if session.stopTerminalRecorded {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultLifecycleTimeout)
	defer cancel()
	normalizedStop := session.stopTerminalEvent
	if normalizedStop == nil {
		event, err := m.prepareSessionStoppedEvent(session, waitErr)
		if err != nil {
			return err
		}
		session.stopTerminalEvent = &event
		normalizedStop = &event
	}
	var recorderErr error
	write := func(ctx context.Context, recorder EventRecorder, event store.SessionEvent) (store.SessionEvent, error) {
		if err := m.persistActiveStopReceipt(ctx, session, &event); err != nil {
			return store.SessionEvent{}, err
		}
		persisted, err := recordIdempotentSessionEvent(ctx, recorder, event)
		if err == nil {
			return persisted, nil
		}
		recorderErr = err
		return m.appendStoredSessionEvent(ctx, session.ID, event)
	}
	if err := m.recordEventWithWriter(ctx, session, *normalizedStop, "", write); err != nil {
		return errors.Join(err, recorderErr)
	}
	session.stopTerminalRecorded = true
	m.notifyAgentEvent(ctx, session, *normalizedStop)
	if kind, summary, evidence, ok := sessionStoppedTranscriptMarker(*normalizedStop); ok &&
		(!promptOwnsTerminalFailure || kind != transcript.MarkerProviderFailure) {
		m.emitTranscriptMarker(ctx, session, normalizedStop.TurnID, kind, summary, evidence)
	}
	return recorderErr
}

func (m *Manager) prepareSessionStoppedEvent(session *Session, waitErr error) (acp.AgentEvent, error) {
	stopReason := store.StopReason("")
	if info := session.Info(); info != nil {
		stopReason = info.StopReason
	}
	turnID, err := m.sessionStopTurnID(session)
	if err != nil {
		return acp.AgentEvent{}, err
	}
	stopEvent := acp.AgentEvent{
		Type:       EventTypeSessionStopped,
		TurnID:     turnID,
		Timestamp:  m.now(),
		StopReason: string(stopReason),
	}
	if waitErr != nil {
		if failure := store.CloneSessionFailure(session.Info().Failure); failure != nil {
			stopEvent.Failure = failure
			stopEvent.Error = failureSummary(failure, waitErr.Error())
		} else {
			stopEvent.Error = diagnostics.RedactAndBound(waitErr.Error(), maxSessionFailureSummaryBytes)
		}
		if proc := session.processHandle(); proc != nil {
			stopEvent.Text = diagnostics.RedactAndBound(proc.Stderr(), maxCrashEvidenceBytes)
		}
	}

	normalized := m.normalizeEvent(session, stopEvent.TurnID, stopEvent).WithEventID("session-stop:" + turnID)
	return m.enrichRecordedAgentEvent(session, normalized), nil
}

func recordIdempotentSessionEvent(
	ctx context.Context,
	recorder EventRecorder,
	event store.SessionEvent,
) (store.SessionEvent, error) {
	if appender, ok := recorder.(idempotentSessionEventAppender); ok {
		return appender.AppendEventIfAbsent(ctx, event)
	}
	return recordPersistedSessionEvent(ctx, recorder, event)
}
