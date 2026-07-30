package session

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

// Resume restarts a stopped session from its persisted metadata and event history.
func (m *Manager) Resume(ctx context.Context, id string) (_ *Session, err error) {
	if ctx == nil {
		return nil, errors.New("session: resume context is required")
	}

	target, err := resumeTarget(id)
	if err != nil {
		return nil, err
	}
	if session, ok := m.Get(target); ok {
		return session, nil
	}
	if err := m.checkNewWorkAdmission(ctx); err != nil {
		return nil, err
	}

	meta, err := m.readMetaWithContext(ctx, target)
	if err != nil {
		return nil, err
	}
	if validationErrs := m.validateInfrastructure(ctx, meta); len(validationErrs) > 0 {
		m.logResumeValidationFailures(meta, validationErrs)
		return nil, fmt.Errorf(
			"session: validate resume infrastructure for %q: %w",
			target,
			errors.Join(validationErrs...),
		)
	}

	spec, err := m.prepareResumeStart(ctx, meta)
	if err != nil {
		return nil, err
	}
	if isUnboundLogicalResume(meta) {
		return m.resumeAcceptedLogicalSession(ctx, &spec)
	}

	return m.startResumedSession(ctx, target, meta, &spec)
}

func resumeTarget(id string) (string, error) {
	target := strings.TrimSpace(id)
	if target == "" {
		return "", errors.New("session: session id is required")
	}
	return target, nil
}

func isUnboundLogicalResume(meta store.SessionMeta) bool {
	return meta.RuntimeStatus == RuntimeStatusUnbound &&
		meta.RuntimeTransition == RuntimeTransitionNone &&
		strings.TrimSpace(derefString(meta.ACPSessionID)) == ""
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
