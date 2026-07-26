package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
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
		m.finishSessionStartRun(accepted.spec.sessionID, accepted.run, err)
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
	m.startNextQueuedInputPrompt(accepted.session.ID)
	return nil
}

func (m *Manager) launchAcceptedSessionStart(accepted *acceptedSessionStart) error {
	ctx := accepted.run.ctx
	spec := accepted.spec
	session := accepted.session
	storage, err := m.openSessionStartRecorder(ctx, spec, accepted.storage)
	if err != nil {
		return startupFailure(
			"session event store startup failed",
			fmt.Errorf("session: open %s event store for %q: %w", spec.startAction, spec.sessionID, err),
		)
	}
	accepted.storage = storage
	session.setRecorder(storage.recorder)
	accepted.run.signalRecorderReady()

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
	session.setAgentDefinition(runtime.agentDef)
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

	accepted.persistFailure = true
	startOpts, err := m.prepareSessionLaunch(ctx, spec, session, &runtime)
	if err != nil {
		return fmt.Errorf("session: prepare %s launch for %q: %w", spec.startAction, spec.sessionID, err)
	}
	accepted.proc, err = m.startAgentProcess(ctx, spec, session, startOpts)
	if err != nil {
		return fmt.Errorf("session: start %s agent process for %q: %w", spec.startAction, spec.sessionID, err)
	}
	if err := m.persistResumeReplayMarker(ctx, spec, session); err != nil {
		return startupFailure("session resume marker persistence failed", err)
	}

	if !accepted.async {
		accepted.persistFailure = false
	}
	if err := m.activateAndWatch(
		ctx,
		session,
		accepted.proc,
		strings.TrimSpace(startOpts.PreferredModel) == "",
		runtime.agent,
		runtime.networkCapabilities,
		spec.postEvent,
		spec.preserveStopReason,
	); err != nil {
		return startupFailure(
			"session activation failed",
			fmt.Errorf("session: activate %s session %q: %w", spec.startAction, spec.sessionID, err),
		)
	}
	m.observeCommittedParticipation(ctx, spec.participationObservation)
	if spec.resumeReplay {
		m.stageResumeReplay(spec.sessionID, spec.resumeReplayBlock)
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
		var stopErr error
		if accepted.proc != nil && !isProcessDone(accepted.proc) {
			stopErr = m.driver.Stop(cleanupCtx, accepted.proc)
		}
		finalizeErr := m.finalizeStopped(cleanupCtx, session, context.Cause(accepted.run.ctx))
		return errors.Join(stopErr, finalizeErr)
	}
	if !accepted.persistFailure {
		return m.discardAcceptedSessionStart(cleanupCtx, accepted, startErr)
	}

	accepted.spec.cleanupSessionDir = false
	persistErr := m.persistFailedStart(cleanupCtx, session, startErr, true)
	sandboxErr := m.finalizeSandbox(cleanupCtx, session, sandboxSyncReasonForStop(session))
	cleanupErr := m.cleanupFailedStart("", accepted.storage.recorder, accepted.proc)
	m.removeActive(session.ID)
	if m.hostedMCP != nil {
		m.hostedMCP.CancelLaunch(session.ID)
	}
	session.clearProviderSecretRedactions()
	return errors.Join(startErr, persistErr, sandboxErr, cleanupErr)
}

func (m *Manager) discardAcceptedSessionStart(
	ctx context.Context,
	accepted *acceptedSessionStart,
	startErr error,
) error {
	session := accepted.session
	sandboxErr := m.finalizeSandbox(ctx, session, sandboxSyncReasonForStop(session))
	var staged *stagedSessionDelete
	if accepted.spec.cleanupSessionDir {
		entry, stageErr := m.stageSessionDirectoryDelete(ctx, session.Info())
		if stageErr != nil {
			retainErr := m.retainAcceptedSessionAfterDiscardFailure(
				ctx,
				accepted,
				errors.Join(startErr, stageErr),
			)
			return errors.Join(startErr, sandboxErr, stageErr, retainErr)
		}
		staged = &entry
	}
	if m.sessionCatalog != nil {
		if catalogErr := m.sessionCatalog.DeleteSession(ctx, session.ID); catalogErr != nil {
			var rollbackErr error
			if staged != nil {
				rollbackErr = m.rollbackStagedSessionDeletes([]stagedSessionDelete{*staged})
			}
			retainErr := m.retainAcceptedSessionAfterDiscardFailure(
				ctx,
				accepted,
				errors.Join(startErr, catalogErr, rollbackErr),
			)
			return errors.Join(startErr, sandboxErr, catalogErr, rollbackErr, retainErr)
		}
	}
	cleanupErr := m.cleanupFailedStart("", accepted.storage.recorder, accepted.proc)
	var directoryErr error
	if staged != nil {
		directoryErr = m.commitStagedSessionDeletes([]stagedSessionDelete{*staged})
	} else {
		m.remove(session.ID)
	}
	if m.hostedMCP != nil {
		m.hostedMCP.CancelLaunch(session.ID)
	}
	session.clearProviderSecretRedactions()
	return errors.Join(startErr, sandboxErr, cleanupErr, directoryErr)
}

func (m *Manager) retainAcceptedSessionAfterDiscardFailure(
	ctx context.Context,
	accepted *acceptedSessionStart,
	failure error,
) error {
	session := accepted.session
	persistErr := m.persistFailedStart(ctx, session, failure, false)
	cleanupErr := m.cleanupFailedStart("", accepted.storage.recorder, accepted.proc)
	m.removeActive(session.ID)
	if m.hostedMCP != nil {
		m.hostedMCP.CancelLaunch(session.ID)
	}
	session.clearProviderSecretRedactions()
	return errors.Join(persistErr, cleanupErr)
}
