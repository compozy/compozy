package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

// failedStartRetention keeps stable append identities and progress until all failure
// events and the stopped projection commit. Its accepted start run owns every field.
type failedStartRetention struct {
	failure       *store.SessionFailure
	events        []acp.AgentEvent
	next          int
	committed     bool
	diagnosticErr error
}

// prepareFailedStartEvents is shared by ordinary and retried retained-start settlement.
func (m *Manager) prepareFailedStartEvents(
	session *Session, failure *store.SessionFailure, startErr error, stopReason store.StopReason,
) (acp.AgentEvent, acp.AgentEvent, error) {
	turnID, err := m.newPromptTurnID()
	if err != nil {
		return acp.AgentEvent{}, acp.AgentEvent{}, err
	}
	stopTurnID, err := m.newPromptTurnID()
	if err != nil {
		return acp.AgentEvent{}, acp.AgentEvent{}, err
	}
	summary := failureSummary(failure, startErr.Error())
	now := m.now()
	errorEvent := m.normalizeEvent(session, turnID, acp.AgentEvent{
		Type:      acp.EventTypeError,
		TurnID:    turnID,
		Timestamp: now,
		Error:     summary,
		Failure:   store.CloneSessionFailure(failure),
	})
	stopEvent := m.normalizeEvent(session, stopTurnID, acp.AgentEvent{
		Type:       EventTypeSessionStopped,
		TurnID:     stopTurnID,
		Timestamp:  now,
		StopReason: string(stopReason),
		Error:      summary,
		Failure:    store.CloneSessionFailure(failure),
	})
	return errorEvent, stopEvent, nil
}

// prepareFailedStartRetention reserves diagnostics once, including any crash bundle,
// so an error reported after SQLite commit cannot create a second terminal row.
func (m *Manager) prepareFailedStartRetention(
	ctx context.Context,
	accepted *acceptedSessionStart,
	startErr error,
) error {
	failure := sessionFailureFromError(startErr, store.FailureStartup)
	failure, diagnosticErr := m.attachCrashBundleToFailure(ctx, accepted.session, failure, startErr, "")
	errorEvent, stopEvent, err := m.prepareFailedStartEvents(
		accepted.session,
		failure,
		startErr,
		failureStopReason(failure),
	)
	if err != nil {
		return err
	}
	errorEvent = errorEvent.WithEventID("session-start-error:" + errorEvent.TurnID)
	stopEvent = stopEvent.WithEventID("session-start-stop:" + stopEvent.TurnID)
	batch := &failedStartRetention{
		failure:       failure,
		events:        []acp.AgentEvent{errorEvent, stopEvent},
		diagnosticErr: diagnosticErr,
	}
	if !acp.IsLoadSessionResourceMissing(startErr) && !errors.Is(startErr, acp.ErrAgentDoesNotSupportSession) {
		if kind, summary, evidence, ok := sessionStoppedTranscriptMarker(stopEvent); ok {
			marker, err := m.prepareTranscriptMarkerEvent(accepted.session, stopEvent.TurnID, transcriptMarkerInput{
				EventID: "session-start-marker:" + stopEvent.TurnID, Kind: kind, Summary: summary, Evidence: evidence,
			})
			if err != nil {
				return err
			}
			batch.events = append(batch.events, marker)
		}
	}
	accepted.retainedFailure = batch
	return nil
}

// persistRetainedFailedStart retries only unfinished appends and metadata. The recorder
// and start run stay owned on every persistence error; completed appends remain idempotent.
func (m *Manager) persistRetainedFailedStart(
	ctx context.Context,
	accepted *acceptedSessionStart,
	startErr error,
) error {
	if accepted.retainedFailure == nil {
		if err := m.prepareFailedStartRetention(ctx, accepted, startErr); err != nil {
			return err
		}
	}
	batch, session := accepted.retainedFailure, accepted.session
	if batch.committed {
		return nil
	}
	for batch.next < len(batch.events) {
		event := batch.events[batch.next]
		if err := m.recordEventWithWriter(ctx, session, event, "", recordRetainedSessionEvent); err != nil {
			return err
		}
		if batch.next >= 2 {
			m.notifyAgentEvent(ctx, session, event)
		}
		batch.next++
	}
	session.setStopClassification(failureStopReason(batch.failure), failureSummary(batch.failure, startErr.Error()))
	session.setFailure(batch.failure)
	session.markExited(m.now())
	session.markStartFailed(m.now())
	if err := m.persistSessionLifecycleState(ctx, session, true); err != nil {
		if _, ok := errors.AsType[*committedSessionProjectionError](err); !ok {
			return err
		}
		batch.diagnosticErr = errors.Join(batch.diagnosticErr, err)
	}
	batch.committed = true
	return nil
}

// recordRetainedSessionEvent resumes this batch's unpublished append after an ambiguous
// acknowledgment, including recorders exposing only the mandatory Query/Record methods.
func recordRetainedSessionEvent(
	ctx context.Context,
	recorder EventRecorder,
	event store.SessionEvent,
) (store.SessionEvent, error) {
	rows, err := recorder.Query(ctx, store.EventQuery{ID: event.ID, Limit: 1})
	if err != nil {
		return store.SessionEvent{}, err
	}
	if len(rows) == 0 {
		if appender, ok := recorder.(idempotentSessionEventAppender); ok {
			row, _, err := appender.AppendEventIfAbsent(ctx, event)
			return row, err
		}
		if err := recorder.Record(ctx, event); err != nil {
			return store.SessionEvent{}, err
		}
		rows, err = recorder.Query(ctx, store.EventQuery{ID: event.ID, Limit: 1})
		if err != nil {
			return store.SessionEvent{}, err
		}
	}
	if len(rows) != 1 || rows[0].Type != event.Type || rows[0].TurnID != event.TurnID ||
		rows[0].Content != event.Content {
		return store.SessionEvent{}, fmt.Errorf(
			"session: retained event identity %q does not match persisted content",
			event.ID,
		)
	}
	return rows[0], nil
}

// stopFailedStartProcess releases its captured process only after exit verification.
func (m *Manager) stopFailedStartProcess(accepted *acceptedSessionStart) (bool, error) {
	if accepted.proc == nil {
		return true, nil
	}
	ctx, cancel := m.lifecycleCleanupContext()
	defer cancel()
	stopErr := m.driver.Stop(ctx, accepted.proc)
	verified, verifyErr := m.verifyStopPhase(ctx, accepted.proc)
	if !verified {
		return false, errors.Join(ErrStopVerificationFailed, stopErr, verifyErr)
	}
	accepted.proc = nil
	return true, errors.Join(stopErr, verifyErr)
}

// joinFailedStartProcess keeps the tracked start owner until termination is verified.
func (m *Manager) joinFailedStartProcess(ctx context.Context, accepted *acceptedSessionStart) error {
	var previous error
	for delay := 100 * time.Millisecond; ; delay = min(2*delay, time.Second) {
		verified, err := m.stopFailedStartProcess(accepted)
		if verified {
			return errors.Join(previous, err)
		}
		previous = err
		m.sessionLogger(accepted.session).Error("session: failed start process cleanup pending", "error", err)
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return errors.Join(err, ctx.Err())
		}
	}
}

// joinFailedStartProcessForSettlement yields cancellation to its full stop ladder without releasing ownership.
func (m *Manager) joinFailedStartProcessForSettlement(ctx context.Context, accepted *acceptedSessionStart) error {
	if accepted.session.hasVerifiedStopProcess(accepted.proc) {
		accepted.proc = nil
		return nil
	}
	joinCtx := ctx
	if accepted.retainedFailure == nil {
		joinCtx = accepted.run.ctx
	}
	err := m.joinFailedStartProcess(joinCtx, accepted)
	if accepted.session.stopWasRequested() && accepted.retainedFailure == nil {
		return nil
	}
	return err
}
