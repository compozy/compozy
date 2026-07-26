package session

import (
	"context"

	"errors"
	"fmt"

	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	envpkg "github.com/compozy/agh/internal/sandbox"
	"github.com/compozy/agh/internal/store"
)

func (m *Manager) syncSandboxToRuntime(
	ctx context.Context,
	provider envpkg.Provider,
	session *Session,
	state envpkg.SessionState,
	meta *store.SessionSandboxMeta,
) error {
	return m.syncSandboxRuntime(
		ctx,
		session,
		state,
		meta,
		envpkg.SyncDirectionToRuntime,
		envpkg.SyncReasonStart,
		provider.SyncToRuntime,
	)
}

type sandboxSyncRunner func(context.Context, envpkg.SessionState, envpkg.SyncOptions) (envpkg.SyncResult, error)

func (m *Manager) syncSandboxRuntime(
	ctx context.Context,
	session *Session,
	state envpkg.SessionState,
	meta *store.SessionSandboxMeta,
	direction envpkg.SyncDirection,
	reason envpkg.SyncReason,
	run sandboxSyncRunner,
) error {
	started := time.Now()
	m.logSandboxLifecycle(sandboxEventFromMeta(
		meta,
		session.ID,
		session.WorkspaceID,
		sandboxEventSyncStart,
		string(reason),
	))

	before, err := m.dispatchSandboxSyncBefore(ctx, session, state, meta, direction, reason)
	if err != nil {
		return err
	}
	if before.Denied {
		return nil
	}

	result, err := run(ctx, state, envpkg.SyncOptions{
		Reason:          reason,
		ExcludePatterns: append([]string(nil), before.ExcludePatterns...),
	})
	duration := time.Since(started)
	errorsList := syncResultErrors(result, err)
	now := m.now()
	meta = cloneSessionSandboxMeta(meta)
	meta.LastSyncAt = &now
	if err != nil {
		return m.finishSandboxSyncError(ctx, session, state, meta, direction, reason, sandboxSyncOutcome{
			result:     result,
			duration:   duration,
			errorsList: errorsList,
			syncTime:   now,
			err:        err,
		})
	}
	return m.finishSandboxSyncSuccess(ctx, session, state, meta, direction, reason, sandboxSyncOutcome{
		result:     result,
		duration:   duration,
		errorsList: errorsList,
		syncTime:   now,
	})
}

type sandboxSyncOutcome struct {
	result     envpkg.SyncResult
	duration   time.Duration
	errorsList []string
	syncTime   time.Time
	err        error
}

func (m *Manager) finishSandboxSyncSuccess(
	ctx context.Context,
	session *Session,
	state envpkg.SessionState,
	meta *store.SessionSandboxMeta,
	direction envpkg.SyncDirection,
	reason envpkg.SyncReason,
	outcome sandboxSyncOutcome,
) error {
	meta.LastSyncError = ""
	session.setSandbox(meta, outcome.syncTime)
	if err := m.persistSessionMetadataOnly(session); err != nil {
		return err
	}
	if err := m.dispatchSandboxSyncAfter(
		ctx,
		session,
		state,
		meta,
		direction,
		reason,
		outcome.duration,
		outcome.result,
		outcome.errorsList,
	); err != nil {
		return err
	}
	completeEvent := sandboxEventFromMeta(
		meta,
		session.ID,
		session.WorkspaceID,
		sandboxEventSyncComplete,
		string(reason),
	)
	completeEvent.Duration = outcome.duration
	m.logSandboxLifecycle(completeEvent)
	return nil
}

func (m *Manager) finishSandboxSyncError(
	ctx context.Context,
	session *Session,
	state envpkg.SessionState,
	meta *store.SessionSandboxMeta,
	direction envpkg.SyncDirection,
	reason envpkg.SyncReason,
	outcome sandboxSyncOutcome,
) error {
	err := syncSandboxWriteError(m, session, meta, outcome)
	errorsList := syncResultErrors(outcome.result, err)
	if afterErr := m.dispatchSandboxSyncAfter(
		ctx,
		session,
		state,
		meta,
		direction,
		reason,
		outcome.duration,
		outcome.result,
		errorsList,
	); afterErr != nil {
		m.warnHookDispatch(ctx, session, hookspkg.HookSandboxSyncAfter, afterErr)
	}

	errorEvent := sandboxEventFromMeta(
		meta,
		session.ID,
		session.WorkspaceID,
		sandboxEventSyncError,
		string(reason),
	)
	errorEvent.Duration = outcome.duration
	attachSandboxError(&errorEvent, err)
	m.logSandboxLifecycle(errorEvent)
	return fmt.Errorf(
		"session: sync sandbox %q %s runtime for %q: %w",
		state.SandboxID,
		syncDirectionPreposition(direction),
		session.ID,
		err,
	)
}

func syncSandboxWriteError(
	m *Manager,
	session *Session,
	meta *store.SessionSandboxMeta,
	outcome sandboxSyncOutcome,
) error {
	meta.LastSyncError = outcome.err.Error()
	session.setSandbox(meta, outcome.syncTime)
	if writeErr := m.persistSessionMetadataOnly(session); writeErr != nil {
		return errors.Join(outcome.err, writeErr)
	}
	return outcome.err
}

func syncDirectionPreposition(direction envpkg.SyncDirection) string {
	if direction == envpkg.SyncDirectionFromRuntime {
		return "from"
	}
	return "to"
}
