package session

import (
	"context"
	"errors"
	"fmt"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
)

type acceptedSessionStart struct {
	spec           *sessionStartSpec
	runtime        sessionStartRuntime
	session        *Session
	storage        sessionStartStorage
	run            *sessionStartRun
	proc           *AgentProcess
	async          bool
	persistFailure bool
}

func (m *Manager) acceptSessionStart(
	acceptCtx context.Context,
	runBaseCtx context.Context,
	spec *sessionStartSpec,
) (_ *acceptedSessionStart, err error) {
	if acceptCtx == nil || runBaseCtx == nil {
		return nil, errors.New("session: accept start contexts are required")
	}
	if spec == nil {
		return nil, errors.New("session: start spec is required")
	}

	runtime, err := m.resolveValidatedAcceptedStartRuntime(acceptCtx, spec)
	if err != nil {
		return nil, err
	}
	releaseLifecycle, err := m.reserveStartLifecycle(
		acceptCtx,
		spec.sessionID,
		spec.workspace.ID,
		spec.scope == store.SessionScopeGlobal,
	)
	if err != nil {
		return nil, fmt.Errorf("session: reserve %s session %q: %w", spec.startAction, spec.sessionID, err)
	}
	defer releaseLifecycle()
	defer func() {
		if err != nil {
			m.releaseReservation(spec.sessionID)
		}
	}()

	storage, err := m.prepareSessionStartStorage(spec)
	if err != nil {
		return nil, fmt.Errorf("session: prepare %s storage for %q: %w", spec.startAction, spec.sessionID, err)
	}
	run, err := m.newSessionStartRun(runBaseCtx, spec.sessionID)
	if err != nil {
		cleanupErr := m.cleanupFailedStart(storage.sessionDir, storage.recorder, nil)
		return nil, errors.Join(err, cleanupErr)
	}
	defer func() {
		if err == nil {
			return
		}
		m.finishSessionStartRun(spec.sessionID, run, err)
	}()
	if err := m.revalidateSpawnedLineageForStart(acceptCtx, spec); err != nil {
		cleanupErr := m.cleanupFailedStart(storage.sessionDir, storage.recorder, nil)
		return nil, errors.Join(err, cleanupErr)
	}

	session := spec.newStartingSession(runtime.agent, runtime.agentDef, storage, m.now())
	if err := m.registerStarting(session); err != nil {
		cleanupErr := m.cleanupFailedStart(storage.sessionDir, storage.recorder, nil)
		return nil, errors.Join(err, cleanupErr)
	}
	if err := m.persistSessionLifecycleState(acceptCtx, session, true); err != nil {
		m.remove(session.ID)
		cleanupErr := m.cleanupFailedStart(storage.sessionDir, storage.recorder, nil)
		return nil, errors.Join(
			fmt.Errorf("session: persist accepted start for %q: %w", spec.sessionID, err),
			cleanupErr,
		)
	}

	return &acceptedSessionStart{
		spec: spec, runtime: runtime, session: session, storage: storage, run: run,
	}, nil
}

func (m *Manager) resolveValidatedAcceptedStartRuntime(
	ctx context.Context,
	spec *sessionStartSpec,
) (sessionStartRuntime, error) {
	runtime, err := m.resolveAcceptedSessionStartRuntime(spec)
	if err != nil {
		spec.startLogger(m).Warn(
			"session.start.runtime_prepare_failed",
			"phase", spec.startAction,
			"error", err,
		)
		return sessionStartRuntime{}, fmt.Errorf(
			"session: resolve %s runtime for %q: %w",
			spec.startAction,
			spec.sessionID,
			err,
		)
	}
	if !spec.deferRuntimeValidation {
		if err := m.validateExplicitStartModel(ctx, &runtime, spec); err != nil {
			return sessionStartRuntime{}, err
		}
	}
	if err := preparePinnedStartCreationIdentity(spec, runtime.agent); err != nil {
		return sessionStartRuntime{}, err
	}
	return runtime, nil
}

func preparePinnedStartCreationIdentity(spec *sessionStartSpec, agent compozyconfig.ResolvedAgent) error {
	if !spec.creationIdentityPinned {
		return nil
	}
	if err := prepareStartCreationIdentityIfEnabled(spec, agent); err != nil {
		return fmt.Errorf("session: prepare creation identity for %q: %w", spec.sessionID, err)
	}
	return nil
}
