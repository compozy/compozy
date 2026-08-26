package session

import (
	"context"
	"errors"
	"fmt"

	hookspkg "github.com/compozy/compozy/internal/hooks"
)

// CreateAccepted durably creates an active logical user session. The ACP
// process is intentionally left unbound until the first prompt supplies its
// runtime snapshot.
func (m *Manager) CreateAccepted(ctx context.Context, opts CreateAcceptedOpts) (*Info, error) {
	if ctx == nil {
		return nil, errors.New("session: create accepted context is required")
	}
	if err := m.checkNewWorkAdmission(ctx); err != nil {
		return nil, err
	}
	spec, err := m.prepareCreateStart(ctx, opts.Session)
	if err != nil {
		return nil, err
	}
	// A logical session has no ACP process yet. Catalog validation belongs to
	// the first runtime bind, where the concrete per-prompt selection is known.
	spec.deferRuntimeValidation = true
	if opts.RuntimeFree {
		if spec.creationIdentityPinned {
			return nil, fmt.Errorf("%w: runtime-free sessions cannot pin a runtime creation profile", ErrValidation)
		}
		spec.runtimeFree = true
		spec.creationIdentityEnabled = false
	}
	accepted, err := m.acceptSessionStart(ctx, m.lifecycleCtx, &spec)
	if err != nil {
		return nil, err
	}
	if err := m.activateAcceptedLogicalSession(accepted); err != nil {
		return nil, err
	}
	return accepted.session.Info(), nil
}

func (m *Manager) activateAcceptedLogicalSession(accepted *acceptedSessionStart) (err error) {
	if accepted == nil || accepted.run == nil || accepted.session == nil {
		return errors.New("session: accepted logical session is required")
	}
	defer func() {
		m.finishSessionStartRun(accepted.spec.sessionID, accepted.run, err)
	}()

	ctx := accepted.run.ctx
	storage, err := m.openSessionStartRecorder(ctx, accepted.spec, accepted.storage)
	if err != nil {
		return m.discardLogicalSessionStart(accepted, fmt.Errorf(
			"session: open logical session event store for %q: %w",
			accepted.spec.sessionID,
			err,
		))
	}
	accepted.storage = storage
	accepted.session.setRecorder(storage.recorder)
	accepted.run.signalRecorderReady()

	runtime := accepted.runtime
	if err := m.prepareAcceptedSessionDefinition(ctx, accepted.spec, &runtime, m.now()); err != nil {
		return m.discardLogicalSessionStart(accepted, err)
	}
	accepted.runtime = runtime
	accepted.session.setAgentDefinition(runtime.agentDef)
	accepted.session.updateSoulSnapshot(accepted.spec.soulSnapshot, accepted.spec.parentSoulDigest, m.now())
	accepted.session.setEffectivePermissions(string(m.startPermissions(
		accepted.spec.sessionType,
		startSpecPermissions(accepted.spec, runtime.agent.Permissions),
	)))
	if err := prepareStartCreationIdentityIfEnabled(accepted.spec, runtime.agent); err != nil {
		return m.discardLogicalSessionStart(accepted, err)
	}
	if err := finalizeStartCreationIdentityIfEnabled(accepted.spec, accepted.session); err != nil {
		return m.discardLogicalSessionStart(accepted, err)
	}
	if err := m.persistSessionLifecycleState(ctx, accepted.session, true); err != nil {
		return m.discardLogicalSessionStart(accepted, err)
	}
	if err := accepted.session.activateWithProcess(nil, m.now(), false, false); err != nil {
		return m.discardLogicalSessionStart(accepted, err)
	}
	if err := m.persistSessionLifecycleState(ctx, accepted.session, true); err != nil {
		return m.discardLogicalSessionStart(accepted, err)
	}
	if err := m.joinNetworkPeer(ctx, accepted.session, runtime.networkCapabilities); err != nil {
		leaveErr := m.leaveNetworkPeer(m.fallbackLifecycleContext(), accepted.session)
		return m.discardLogicalSessionStart(accepted, errors.Join(err, leaveErr))
	}

	switch accepted.spec.postEvent {
	case hookspkg.HookSessionPostCreate:
		m.dispatchSessionPostCreate(ctx, accepted.session)
	case hookspkg.HookSessionPostResume:
		m.dispatchSessionPostResume(ctx, accepted.session)
	}
	if m.notifier != nil {
		m.notifier.OnSessionCreated(ctx, accepted.session)
	}
	if _, err := m.persistSessionPresence(ctx, accepted.session, m.now()); err != nil {
		m.sessionLogger(accepted.session).Warn(
			"session: persist logical session presence failed",
			"error",
			err,
		)
	}
	m.observeCommittedParticipation(ctx, accepted.spec.participationObservation)
	return nil
}

func (m *Manager) discardLogicalSessionStart(accepted *acceptedSessionStart, cause error) error {
	cleanupCtx, cancel := m.lifecycleCleanupContext()
	defer cancel()
	return m.discardAcceptedSessionStart(cleanupCtx, accepted, cause)
}
