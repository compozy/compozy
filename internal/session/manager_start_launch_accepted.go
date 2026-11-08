package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

func startupFailure(summary string, err error) error {
	if err == nil {
		return nil
	}
	return acp.WrapFailure(store.FailureStartup, summary, err)
}

func (m *Manager) runAcceptedSessionStart(accepted *acceptedSessionStart) (err error) {
	if accepted == nil || accepted.run == nil {
		return errors.New("session: accepted start is required")
	}
	defer func() {
		if !accepted.cleanupPending {
			m.finishSessionStartRun(accepted.spec.sessionID, accepted.run, err)
		}
	}()

	err = m.launchAcceptedSessionStart(accepted)
	if err == nil {
		return nil
	}
	return m.settleAcceptedSessionStartFailure(accepted, err)
}

func (m *Manager) runAcceptedSessionStartAndDispatch(accepted *acceptedSessionStart) error {
	if err := m.runAcceptedSessionStart(accepted); err != nil {
		return err
	}
	// Resume owns the dispatch kick after releasing its conversation lock and
	// publishing the completed transition, including logical-only resumes.
	if accepted.spec.startAction != sessionStartActionResume {
		m.startNextQueuedInputPrompt(accepted.session.ID)
	}
	return nil
}

func (m *Manager) launchAcceptedSessionStart(accepted *acceptedSessionStart) error {
	ctx := accepted.run.ctx
	spec := accepted.spec
	session := accepted.session
	if err := m.prepareAcceptedSessionRecorder(accepted); err != nil {
		return err
	}

	runtime := accepted.runtime
	if err := m.prepareAcceptedSessionRuntime(ctx, spec, &runtime, m.now()); err != nil {
		spec.startLogger(m).Warn(
			"session.start.runtime_prepare_failed",
			"phase", spec.startAction,
			"error", err,
		)
		return startupFailure("session runtime preparation failed", err)
	}
	accepted.runtime = runtime
	session.setAgentDefinition(runtime.agentDef, runtime.startupManifest)
	session.updateSoulSnapshot(spec.soulSnapshot, spec.parentSoulDigest, m.now())
	if err := prepareStartCreationIdentityIfEnabled(spec, runtime.agent); err != nil {
		return startupFailure(
			"session creation identity preparation failed",
			fmt.Errorf("session: prepare creation identity for %q: %w", spec.sessionID, err),
		)
	}
	if err := m.restoreAdvertisedCommands(ctx, session); err != nil {
		return startupFailure(
			"session advertised command restoration failed",
			fmt.Errorf("session: restore advertised commands for %q: %w", spec.sessionID, err),
		)
	}

	accepted.persistFailure = accepted.async || !spec.discardStartFailure
	startOpts, err := m.launchAcceptedSessionAttempts(accepted, &runtime)
	if err != nil {
		return err
	}
	accepted.runtime = runtime
	acceptedID := accepted.proc.SessionID
	if spec.resumeReplay || spec.resumeReplayBlock != "" {
		session.setPendingResumeReplay(spec.resumeReplayBlock)
	}
	route := acceptedRouteRecord(spec.fallbackAttempt, runtime.agent, "")
	session.commitAcceptedRoute(route, spec.command, spec.sessionEnv)
	if err := m.persistResumeReplayMarker(ctx, spec, session); err != nil {
		return acp.WrapAcceptedStart(acceptedID, startupFailure("session resume marker persistence failed", err))
	}

	if !accepted.async {
		accepted.persistFailure = false
	}
	if err := m.activateAndWatch(
		ctx,
		session,
		accepted.proc,
		strings.TrimSpace(startOpts.PreferredModel) == "" &&
			startOpts.RuntimeStrategy == acp.RuntimeApplicationSessionConfig,
		runtime.agent,
		spec.postEvent,
		spec.preserveStopReason,
	); err != nil {
		return acp.WrapAcceptedStart(acceptedID, startupFailure(
			"session activation failed",
			fmt.Errorf("session: activate %s session %q: %w", spec.startAction, spec.sessionID, err),
		))
	}
	if spec.resumeReplayBlock != "" {
		m.stageResumeReplay(spec.sessionID, spec.resumeReplayBlock)
	}
	return nil
}

func (m *Manager) prepareAcceptedSessionRecorder(accepted *acceptedSessionStart) error {
	ctx, spec, session := accepted.run.ctx, accepted.spec, accepted.session
	storage := accepted.storage
	if storage.recorder == nil {
		var err error
		storage, err = m.openSessionStartRecorder(ctx, spec, storage)
		if err != nil {
			return startupFailure(
				"session event store startup failed",
				fmt.Errorf("session: open %s event store for %q: %w", spec.startAction, spec.sessionID, err),
			)
		}
	}
	accepted.storage = storage
	session.setRecorder(storage.recorder)
	accepted.run.signalRecorderReady()
	if err := m.projectInputClearTraces(ctx, session); err != nil {
		return startupFailure("session queue clear projection failed", err)
	}

	return nil
}

func (m *Manager) settleAcceptedSessionStartFailure(
	accepted *acceptedSessionStart,
	startErr error,
) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(m.lifecycleCtx), defaultLifecycleTimeout)
	defer cancel()
	session := accepted.session

	if session.stopWasRequested() {
		return m.settleCanceledSessionStart(cleanupCtx, accepted)
	}
	if !accepted.persistFailure {
		return m.discardAcceptedSessionStart(cleanupCtx, accepted, startErr)
	}

	accepted.spec.cleanupSessionDir = false
	verified, processErr := m.stopFailedStartProcess(accepted)
	if !verified {
		return m.deferFailedStartCleanup(accepted, errors.Join(startErr, processErr))
	}
	startErr = errors.Join(startErr, processErr)
	persistErr := m.persistFailedStart(cleanupCtx, session, startErr, true)
	cleanupErr := m.cleanupFailedStart("", accepted.storage.recorder, accepted.proc)
	m.removeActive(session.ID)
	if m.hostedMCP != nil {
		m.hostedMCP.CancelLaunch(session.ID)
	}
	session.clearProviderSecretRedactions()
	return errors.Join(startErr, persistErr, cleanupErr)
}

// discardAcceptedSessionStart closes the captured recorder before acquiring a deletion capability.
func (m *Manager) discardAcceptedSessionStart(
	ctx context.Context,
	accepted *acceptedSessionStart,
	startErr error,
) error {
	if accepted.spec.deriveReceipt != nil && !accepted.session.isDeriveCommitted() {
		return m.discardUncommittedSessionStart(accepted, startErr)
	}
	// Staging retains the family lease. Final close must finish before staging,
	// and a committed derived child must retain its receipt and history.
	retain := accepted.spec.deriveReceipt != nil
	closeErr := m.cleanupFailedStart("", accepted.storage.recorder, nil)
	accepted.session.setRecorder(nil)
	failure := errors.Join(startErr, closeErr)
	if closeErr != nil {
		joinErr := m.cleanupFailedStart("", accepted.storage.recorder, nil)
		failure = errors.Join(failure, joinErr)
		complete := joinErr == nil
		if cleanup, ok := accepted.storage.recorder.(interface{ CleanupComplete() bool }); ok {
			complete = cleanup.CleanupComplete()
		}
		if !complete {
			accepted.recorderClosing = true
			return m.deferFailedStartCleanup(accepted, failure)
		}
		retain = true
	}
	accepted.storage.recorder = nil
	pending, result := m.settleClosedSessionStart(ctx, accepted, failure, retain)
	if pending {
		return m.deferFailedStartCleanup(accepted, errors.Join(failure, result))
	}
	return result
}

// settleClosedSessionStart arbitrates failure retention, deletion and requested stop
// only after cleanup releases the captured recorder. No recorder is reopened before the gate.
func (m *Manager) settleClosedSessionStart(
	ctx context.Context, accepted *acceptedSessionStart, failure error, retain bool,
) (bool, error) {
	releaseCommit, err := acquireSessionStartLaunchCommit(ctx, accepted.run)
	if err != nil {
		return true, err
	}
	defer releaseCommit()
	settleCtx, cancel := m.lifecycleCleanupContext()
	defer cancel()
	if retain || accepted.session.stopWasRequested() {
		if err := m.reopenAcceptedSessionRecorder(settleCtx, accepted); err != nil {
			if errors.Is(err, ErrSessionNotFound) {
				return false, errors.Join(failure, m.releaseMissingSessionStart(accepted, err))
			}
			return true, err
		}
	}
	// Once a failure batch reserves its terminal identity, retries own that outcome.
	// A later Stop joins the start run instead of appending another terminal event.
	if accepted.session.stopWasRequested() && accepted.retainedFailure == nil {
		releaseCommit()
		return false, m.settleCanceledSessionStart(settleCtx, accepted)
	}
	if retain {
		pending, retainErr := m.retainAcceptedSessionAfterDiscardFailure(settleCtx, accepted, failure)
		return pending, errors.Join(failure, retainErr)
	}
	return m.discardClosedSessionStart(settleCtx, accepted, failure)
}

// discardClosedSessionStart holds launchCommit while staging and deleting a failed start.
func (m *Manager) discardClosedSessionStart(
	ctx context.Context, accepted *acceptedSessionStart, startErr error,
) (bool, error) {
	verified, processErr := m.stopFailedStartProcess(accepted)
	if !verified {
		return true, errors.Join(startErr, processErr)
	}
	startErr = errors.Join(startErr, processErr)
	session := accepted.session
	var staged *stagedSessionDelete
	if accepted.spec.cleanupSessionDir {
		entry, stageErr := m.stageSessionDirectoryDelete(ctx, session.ID, session.Info())
		if stageErr != nil {
			pending, retainErr := m.retainAcceptedSessionAfterDiscardFailure(
				ctx,
				accepted,
				errors.Join(startErr, stageErr),
			)
			return pending,
				errors.Join(startErr, stageErr, retainErr)
		}
		staged = &entry
	}
	if m.sessionCatalog != nil {
		if catalogErr := m.sessionCatalog.DeleteSession(ctx, session.ID); catalogErr != nil {
			var rollbackErr error
			if staged != nil {
				rollbackErr = m.rollbackStagedSessionDeletes(ctx, []stagedSessionDelete{*staged})
			}
			pending, retainErr := m.retainAcceptedSessionAfterDiscardFailure(
				ctx,
				accepted,
				errors.Join(startErr, catalogErr, rollbackErr),
			)
			return pending,
				errors.Join(startErr, catalogErr, rollbackErr, retainErr)
		}
	}
	cleanupErr := m.cleanupFailedStart("", accepted.storage.recorder, accepted.proc)
	var directoryErr error
	if staged != nil {
		directoryErr = m.commitStagedSessionDeletes(ctx, []stagedSessionDelete{*staged})
	} else {
		m.remove(session.ID)
	}
	if m.hostedMCP != nil {
		m.hostedMCP.CancelLaunch(session.ID)
	}
	session.clearProviderSecretRedactions()
	return false, errors.Join(startErr, cleanupErr, directoryErr)
}

// deferFailedStartCleanup keeps the existing start run pending until its recorder releases ownership.
func (m *Manager) deferFailedStartCleanup(accepted *acceptedSessionStart, failure error) error {
	if accepted.run == nil {
		return errors.Join(failure, ErrRecoveryPersistence, m.cleanupFailedStart("", nil, accepted.proc))
	}
	accepted.cleanupPending = true
	go func() {
		// Caller expiry and manager cancellation cannot release this resource owner.
		// Shutdown still bounds its wait through the existing tracked start run.
		ctx := context.WithoutCancel(m.lifecycleCtx)
		processErr := m.joinFailedStartProcess(ctx, accepted)
		var result error
		for delay := 100 * time.Millisecond; ; delay = min(2*delay, time.Second) {
			if accepted.recorderClosing {
				m.joinFailedStartRecorder(ctx, accepted.storage.recorder)
				accepted.storage.recorder = nil
				accepted.recorderClosing = false
			}
			if accepted.retainedFailure != nil && accepted.retainedFailure.committed {
				releaseCommit, err := acquireSessionStartLaunchCommit(ctx, accepted.run)
				if err != nil {
					result = err
					continue
				}
				m.finishRetainedSessionStart(ctx, accepted)
				releaseCommit()
				result = errors.Join(failure, result, accepted.retainedFailure.diagnosticErr)
				break
			}
			var pending bool
			pending, result = m.settleClosedSessionStart(ctx, accepted, failure, true)
			if !pending {
				break
			}
			m.sessionLogger(accepted.session).Error("session: failed start settlement pending", "error", result)
			// Reopening has not succeeded: preserve Starting/Stopping and its owner
			// until storage becomes available, without duplicating lifecycle events.
			timer := time.NewTimer(delay)
			<-timer.C
		}
		m.finishSessionStartRun(accepted.session.ID, accepted.run, errors.Join(processErr, result))
	}()
	return failure
}

// joinFailedStartRecorder also joins EventRecorder-only implementations through Close.
// An error without a completion signal keeps ownership and retries the bounded close.
func (m *Manager) joinFailedStartRecorder(ctx context.Context, recorder EventRecorder) {
	if recorder == nil {
		return
	}
	if waiter, ok := recorder.(interface{ WaitForCleanup(context.Context) error }); ok {
		if waiter.WaitForCleanup(ctx) == nil {
			return
		}
	}
	for delay := 100 * time.Millisecond; ; delay = min(2*delay, time.Second) {
		closeErr := m.cleanupFailedStart("", recorder, nil)
		if closeErr == nil {
			return
		}
		if cleanup, ok := recorder.(interface{ CleanupComplete() bool }); ok && cleanup.CleanupComplete() {
			return
		}
		timer := time.NewTimer(delay)
		<-timer.C
	}
}

// discardUncommittedSessionStart sweeps a start whose catalog registration never
// committed (a derived child whose registration transaction failed or lost a race):
// only its reserved directory exists, so it is removed like a failed accept.
func (m *Manager) discardUncommittedSessionStart(accepted *acceptedSessionStart, startErr error) error {
	session := accepted.session
	cleanupErr := m.cleanupFailedStart(accepted.storage.sessionDir, accepted.storage.recorder, accepted.proc)
	m.remove(session.ID)
	if m.hostedMCP != nil {
		m.hostedMCP.CancelLaunch(session.ID)
	}
	session.clearProviderSecretRedactions()
	return errors.Join(startErr, cleanupErr)
}

// retainAcceptedSessionAfterDiscardFailure preserves history when deletion rolls back.
func (m *Manager) retainAcceptedSessionAfterDiscardFailure(
	ctx context.Context, accepted *acceptedSessionStart, failure error,
) (bool, error) {
	verified, processErr := m.stopFailedStartProcess(accepted)
	if !verified {
		return true, processErr
	}
	failure = errors.Join(failure, processErr)
	if err := m.reopenAcceptedSessionRecorder(ctx, accepted); err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return false, m.releaseMissingSessionStart(accepted, err)
		}
		return true, err
	}
	if err := m.persistRetainedFailedStart(ctx, accepted, failure); err != nil {
		return true, err
	}
	cleanupErr := m.cleanupFailedStart("", accepted.storage.recorder, nil)
	complete := cleanupErr == nil
	if cleanup, ok := accepted.storage.recorder.(interface{ CleanupComplete() bool }); ok {
		complete = cleanup.CleanupComplete()
	}
	if !complete {
		accepted.recorderClosing = true
		return true, errors.Join(processErr, cleanupErr, accepted.retainedFailure.diagnosticErr)
	}
	accepted.storage.recorder = nil
	m.finishRetainedSessionStart(ctx, accepted)
	return false, errors.Join(processErr, cleanupErr, accepted.retainedFailure.diagnosticErr)
}

// finishRetainedSessionStart releases the active projection after writes and cleanup finish.
func (m *Manager) finishRetainedSessionStart(ctx context.Context, accepted *acceptedSessionStart) {
	m.removeActive(accepted.session.ID)
	m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventUpserted, accepted.session.Info()))
	if accepted.persistFailure {
		m.notifyFailedStart(ctx, accepted.session)
	}
	if accepted.session.stopWasRequested() || (accepted.run != nil && accepted.run.stopJoined.Load()) {
		m.dispatchSessionPostStop(ctx, accepted.session)
		if !accepted.persistFailure {
			m.notifyStoppedSession(ctx, accepted.session)
		}
	}
	if m.hostedMCP != nil {
		m.hostedMCP.CancelLaunch(accepted.session.ID)
	}
	accepted.session.clearProviderSecretRedactions()
}

// releaseMissingSessionStart leaves a failed rollback's tombstone to existing boot
// recovery. Reopening a missing original path cannot converge and must not recreate it.
func (m *Manager) releaseMissingSessionStart(accepted *acceptedSessionStart, cause error) error {
	cleanupErr := m.cleanupFailedStart("", nil, accepted.proc)
	m.removeActive(accepted.session.ID)
	if m.hostedMCP != nil {
		m.hostedMCP.CancelLaunch(accepted.session.ID)
	}
	accepted.session.clearProviderSecretRedactions()
	return errors.Join(ErrRecoveryPersistence, cause, cleanupErr)
}

// reopenAcceptedSessionRecorder restores the retained owner without clearing prior history.
func (m *Manager) reopenAcceptedSessionRecorder(ctx context.Context, accepted *acceptedSessionStart) error {
	if accepted.storage.recorder != nil {
		return nil
	}
	// A failed rollback must not recreate storage at a missing original directory.
	if _, err := m.readMetaWithContext(ctx, accepted.session.ID); err != nil {
		return err
	}
	spec := *accepted.spec
	spec.clearEventStoreOnOpen = false
	storage, err := m.openSessionStartRecorder(ctx, &spec, accepted.storage)
	if err != nil {
		return err
	}
	accepted.storage = storage
	accepted.session.setRecorder(storage.recorder)
	return nil
}
