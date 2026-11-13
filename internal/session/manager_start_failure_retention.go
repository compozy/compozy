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
	if !acp.LoadSessionRequiresFreshThread(startErr) {
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
// and start run stay owned on terminal persistence errors; ordinary marker errors remain diagnostic.
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
			if batch.next < 2 || accepted.spec.discardStartFailure || accepted.spec.deriveReceipt != nil {
				return err
			}
			// Ordinary failures report marker errors and close; rolled-back discards retain complete history.
			batch.diagnosticErr = errors.Join(batch.diagnosticErr, err)
			batch.next++
			continue
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
		if len(rows) == 0 {
			// Unsequenced recorders retain the mandatory Record acknowledgment contract.
			return event, nil
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
func (m *Manager) stopFailedStartProcess(accepted *acceptedSessionStart, yieldToStop bool) (bool, error) {
	if accepted.proc == nil {
		return true, nil
	}
	ctx, cancel := m.lifecycleCleanupContext()
	defer cancel()
	if accepted.processStopPending == nil {
		accepted.processStopPending = m.startStopAction(ctx, accepted.proc, m.driver.Stop)
	}
	var stopRequested <-chan struct{}
	if yieldToStop && accepted.run != nil && accepted.retainedFailure == nil {
		stopRequested = accepted.run.stopRequested
	}
	var stopErr error
	select {
	case stopErr = <-accepted.processStopPending:
		accepted.processStopPending = nil
	case <-stopRequested:
		return false, errors.Join(ErrStopVerificationFailed, errStartupCanceledByStop)
	case <-ctx.Done():
		stopErr = ctx.Err()
	}
	verified, verifyErr := m.verifyStopPhase(ctx, accepted.proc)
	if !verified {
		return false, errors.Join(ErrStopVerificationFailed, stopErr, verifyErr)
	}
	accepted.proc = nil
	return true, errors.Join(stopErr, verifyErr)
}

// joinFailedStartProcess keeps the tracked start owner until termination is verified.
func (m *Manager) joinFailedStartProcess(ctx context.Context, accepted *acceptedSessionStart, yieldToStop bool) error {
	var previous error
	for delay := 100 * time.Millisecond; ; delay = min(2*delay, time.Second) {
		if yieldToStop && accepted.retainedFailure == nil && accepted.session.stopWasRequested() {
			return nil
		}
		verified, err := m.stopFailedStartProcess(accepted, yieldToStop)
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
	return m.joinFailedStartProcess(ctx, accepted, true)
}

// retainFailedSessionStart retries the existing failure batch with an independent persistence budget.
func (m *Manager) retainFailedSessionStart(accepted *acceptedSessionStart, startErr error) error {
	ctx, cancel := m.lifecycleCleanupContext()
	defer cancel()
	pending, err := m.retainAcceptedSessionAfterDiscardFailure(ctx, accepted, startErr)
	if pending {
		return m.deferFailedStartCleanup(accepted, errors.Join(startErr, err))
	}
	return errors.Join(startErr, err)
}

// releaseMissingSessionStart verifies every captured process before leaving a rollback tombstone to boot recovery.
func (m *Manager) releaseMissingSessionStart(
	ctx context.Context, accepted *acceptedSessionStart, cause error,
) (bool, error) {
	for _, proc := range []*AgentProcess{accepted.proc, accepted.session.processHandle()} {
		if proc == nil || accepted.session.hasVerifiedStopProcess(proc) {
			continue
		}
		result, err := m.runTerminationLadder(ctx, proc, terminationTarget{})
		if !result.Verified {
			return true, errors.Join(ErrRecoveryPersistence, cause, err)
		}
		accepted.session.retainVerifiedStopOutcome(proc, result.StopOutcome)
		if accepted.proc == proc {
			accepted.proc = nil
		}
	}
	m.removeActive(accepted.session.ID)
	if m.hostedMCP != nil {
		m.hostedMCP.CancelLaunch(accepted.session.ID)
	}
	accepted.session.clearProviderSecretRedactions()
	return false, errors.Join(ErrRecoveryPersistence, cause)
}
