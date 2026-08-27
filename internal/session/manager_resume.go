package session

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/store"
)

// Resume restarts a stopped session from its persisted metadata and event history.
func (m *Manager) Resume(ctx context.Context, id string) (resumed *Session, err error) {
	if ctx == nil {
		return nil, errors.New("session: resume context is required")
	}

	target, err := resumeTarget(id)
	if err != nil {
		return nil, err
	}
	operationCtx, unlockConversation, active, err := m.acquireResumeOperation(ctx, target)
	if err != nil {
		return nil, err
	}
	if active != nil {
		return active, nil
	}
	ctx = operationCtx
	defer unlockConversation()
	run, owner, err := m.beginSessionResume(target)
	if err != nil {
		return nil, err
	}
	if !owner {
		return waitForSessionResume(ctx, run)
	}

	defer func() {
		m.finishSessionResume(target, run, resumed, err)
	}()
	meta, err := m.readMetaWithContext(ctx, target)
	if err != nil {
		return nil, err
	}
	if err := m.requireSessionUnarchived(ctx, meta.WorkspaceID, target); err != nil {
		return nil, err
	}
	if err := m.rejectDeadSessionAttachment(ctx, target, meta); err != nil {
		return nil, err
	}
	return m.resumeSession(ctx, target)
}

func (m *Manager) acquireResumeOperation(
	ctx context.Context,
	target string,
) (context.Context, func(), *Session, error) {
	for {
		operationCtx, unlockConversation, lockErr := m.lockConversationOperation(ctx, target)
		if lockErr != nil {
			return nil, nil, nil, lockErr
		}
		current, exists := m.Get(target)
		if exists && current != nil {
			switch current.Info().State {
			case StateStopping, StateStopped:
				unlockConversation()
				if waitErr := m.waitForTerminalSessionBeforeResume(ctx, target); waitErr != nil {
					return nil, nil, nil, waitErr
				}
				continue
			case StateActive:
				active, activeErr := m.waitForResumedSession(operationCtx, target, current)
				if activeErr == nil {
					unlockConversation()
					return nil, nil, active, nil
				}
				currentState := current.Info().State
				becameTerminal := errors.Is(activeErr, ErrSessionNotActive) &&
					(currentState == StateStopping || currentState == StateStopped)
				unlockConversation()
				if !becameTerminal {
					return nil, nil, nil, activeErr
				}
				if currentState == StateStopping {
					if waitErr := m.waitForTerminalSessionBeforeResume(ctx, target); waitErr != nil {
						return nil, nil, nil, waitErr
					}
				}
				continue
			}
		}
		return operationCtx, unlockConversation, nil, nil
	}
}

func (m *Manager) waitForTerminalSessionBeforeResume(ctx context.Context, target string) error {
	m.mu.RLock()
	current := m.sessions[target]
	finalization := m.finalizing[target]
	m.mu.RUnlock()
	if current == nil {
		return nil
	}
	if current.Info().State == StateStopped {
		if finalization == nil {
			return fmt.Errorf("session: stopped session %q remains active without finalization", target)
		}
		if err := waitForSessionFinalization(ctx, finalization); err != nil {
			return fmt.Errorf("session: wait for stopped session %q finalization before resume: %w", target, err)
		}
		return nil
	}
	if current.Info().State != StateStopping {
		return nil
	}
	process := current.processHandle()
	if process != nil {
		select {
		case <-process.Done():
		case <-ctx.Done():
			return fmt.Errorf("session: wait for stopping process %q before resume: %w", target, ctx.Err())
		}
		if err := m.finalizeStopped(ctx, current, process.Wait()); err != nil {
			return fmt.Errorf("session: finalize stopping session %q before resume: %w", target, err)
		}
		return nil
	}
	if err := m.finalizeStopped(ctx, current, nil); err != nil {
		return fmt.Errorf("session: finalize runtime-free session %q before resume: %w", target, err)
	}
	return nil
}

func (m *Manager) resumeSession(ctx context.Context, target string) (*Session, error) {
	if session, ok := m.Get(target); ok && session.Info().State != StateStopped {
		return m.waitForResumedSession(ctx, target, session)
	}
	if err := m.checkNewWorkAdmission(ctx); err != nil {
		return nil, err
	}

	meta, err := m.readMetaWithContext(ctx, target)
	if err != nil {
		return nil, err
	}
	runtimeFree := isRuntimeFreeLogicalSession(meta)
	if !runtimeFree {
		if err := requirePersistedProvider(meta); err != nil {
			return nil, err
		}
	}
	if validationErrs := m.validateInfrastructureForRuntime(ctx, meta, runtimeFree); len(validationErrs) > 0 {
		m.logResumeValidationFailures(meta, validationErrs)
		return nil, fmt.Errorf(
			"session: validate resume infrastructure for %q: %w",
			target,
			errors.Join(validationErrs...),
		)
	}
	if _, err := m.RecoverPendingInteractions(ctx, target); err != nil {
		return nil, err
	}

	spec, err := m.prepareResumeStart(ctx, meta)
	if err != nil {
		return nil, err
	}
	spec.runtimeFree = runtimeFree
	if err := m.discardMaterializedSessionLedgerForResume(ctx, meta); err != nil {
		return nil, err
	}
	var resumed *Session
	if isUnboundLogicalResume(meta) {
		resumed, err = m.resumeAcceptedLogicalSession(ctx, &spec)
	} else {
		resumed, err = m.startResumedSession(ctx, target, meta, &spec)
	}
	if err != nil {
		return nil, errors.Join(err, m.rematerializeStoppedSessionLedger(ctx, target))
	}
	return resumed, nil
}

func (m *Manager) waitForResumedSession(
	ctx context.Context,
	target string,
	session *Session,
) (*Session, error) {
	if run := m.sessionStartRun(target); run != nil {
		if err := waitForSessionStartRun(ctx, run); err != nil {
			return nil, err
		}
		active, ok := m.Get(target)
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrSessionNotFound, target)
		}
		session = active
	}
	if state := session.Info().State; state != StateActive {
		return nil, fmt.Errorf("%w: %s (%s)", ErrSessionNotActive, target, state)
	}
	return session, nil
}

func resumeTarget(id string) (string, error) {
	target := strings.TrimSpace(id)
	if target == "" {
		return "", errors.New("session: session id is required")
	}
	return target, nil
}

// rejectDeadSessionAttachment keeps terminal process failures out of runtime attachment paths.
func (m *Manager) rejectDeadSessionAttachment(ctx context.Context, target string, meta store.SessionMeta) error {
	if isUnboundLogicalResume(meta) {
		return nil
	}
	if meta.Failure == nil || meta.Failure.Kind != store.FailureProcess {
		return nil
	}
	health, err := m.GetSessionHealth(ctx, target)
	if err != nil {
		return fmt.Errorf("session: read health before attachment %q: %w", target, err)
	}
	if health.Health != heartbeat.SessionHealthDead || health.Attachable || health.EligibleForWake {
		return nil
	}
	return fmt.Errorf("%w: session %q has a dead runtime", store.ErrSessionNotAttachable, target)
}

func isUnboundLogicalResume(meta store.SessionMeta) bool {
	return meta.RuntimeStatus == RuntimeStatusUnbound &&
		meta.RuntimeTransition == RuntimeTransitionNone &&
		strings.TrimSpace(derefString(meta.ACPSessionID)) == ""
}

func isRuntimeFreeLogicalSession(meta store.SessionMeta) bool {
	return isUnboundLogicalResume(meta) &&
		strings.TrimSpace(meta.Provider) == "" &&
		meta.CreationProfile == nil &&
		meta.CreationOptions == nil &&
		strings.TrimSpace(meta.CreationProfileRef) == "" &&
		strings.TrimSpace(meta.PolicySpecDigest) == "" &&
		strings.TrimSpace(meta.CreationDigest) == ""
}

func (m *Manager) resumeAcceptedLogicalSession(
	ctx context.Context,
	spec *sessionStartSpec,
) (*Session, error) {
	accepted, err := m.acceptSessionStart(ctx, m.lifecycleCtx, spec)
	if err != nil {
		return nil, err
	}
	if err := m.activateAcceptedLogicalSession(accepted); err != nil {
		return nil, err
	}
	return accepted.session, nil
}

func (m *Manager) startResumedSession(
	ctx context.Context,
	target string,
	meta store.SessionMeta,
	spec *sessionStartSpec,
) (*Session, error) {
	if strings.TrimSpace(spec.acpSessionID) == "" {
		spec.resumeReplay = true
	}

	session, err := m.startSession(ctx, spec)
	if err == nil {
		return session, nil
	}
	return m.recoverFailedResumeStart(ctx, target, meta, err)
}

func (m *Manager) recoverFailedResumeStart(
	ctx context.Context,
	target string,
	meta store.SessionMeta,
	startErr error,
) (*Session, error) {
	clearACP := acp.IsLoadSessionResourceMissing(startErr) || errors.Is(startErr, acp.ErrAgentDoesNotSupportSession)
	metaPath := store.SessionMetaFile(filepath.Join(m.homePaths.SessionsDir, target))
	restoredMeta, err := m.restoreFailedResumeStart(metaPath, meta, clearACP)
	if err != nil {
		return nil, errors.Join(startErr, err)
	}
	if err := m.persistSessionCatalogFromMeta(ctx, restoredMeta); err != nil {
		return nil, errors.Join(startErr, err)
	}
	if !clearACP {
		return nil, startErr
	}
	return m.resumeWithContextReplay(ctx, meta, restoredMeta, startErr)
}

func (m *Manager) resumeWithContextReplay(
	ctx context.Context,
	meta store.SessionMeta,
	restoredMeta store.SessionMeta,
	startErr error,
) (*Session, error) {
	m.resumeLogger(meta).Info(
		"session.resume.context_replay_fallback",
		"phase", "resume",
		"fallback_reason", resumeFallbackReason(startErr),
		"error", startErr,
	)

	fallbackSpec, err := m.prepareResumeStart(ctx, restoredMeta)
	if err != nil {
		return nil, errors.Join(startErr, err)
	}
	fallbackSpec.resumeReplay = true
	fallbackSpec.resumeReplayReason = resumeFallbackReason(startErr)

	session, err := m.startSession(ctx, &fallbackSpec)
	if err != nil {
		return nil, errors.Join(startErr, err)
	}
	return session, nil
}

func resumeFallbackReason(err error) string {
	if errors.Is(err, acp.ErrAgentDoesNotSupportSession) {
		return "load_session_unsupported"
	}
	return "load_session_resource_missing"
}
