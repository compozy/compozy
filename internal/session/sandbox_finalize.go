package session

import (
	"context"

	"errors"
	"fmt"

	"log/slog"

	"strings"
	"time"

	envpkg "github.com/compozy/agh/internal/sandbox"
	"github.com/compozy/agh/internal/store"
)

func (m *Manager) finalizeSandbox(
	ctx context.Context,
	session *Session,
	reason envpkg.SyncReason,
) error {
	if session == nil {
		return nil
	}
	meta := cloneSessionSandboxMeta(session.Info().Sandbox)
	if meta == nil {
		return nil
	}
	if m.sandbox == nil {
		return errors.New("session: sandbox registry is required")
	}

	provider, err := m.sandbox.Provider(envpkg.Backend(strings.TrimSpace(meta.Backend)))
	if err != nil {
		return fmt.Errorf("session: resolve sandbox provider %q: %w", meta.Backend, err)
	}

	state := sessionSandboxStateFromMeta(meta)
	var errs []error
	if syncErr := m.syncSandboxFromRuntime(ctx, provider, session, state, meta, reason); syncErr != nil {
		if reason == envpkg.SyncReasonCrash {
			m.sessionLogger(session).Warn("session: sandbox crash sync failed", "error", syncErr)
		} else {
			errs = append(errs, syncErr)
		}
		meta = cloneSessionSandboxMeta(session.Info().Sandbox)
		state = sessionSandboxStateFromMeta(meta)
	}

	shouldDestroy := session.sandboxShouldDestroy()
	stopPayload, stopErr := m.dispatchSandboxStop(ctx, session, state, meta, reason, shouldDestroy)
	if stopErr != nil {
		errs = append(errs, stopErr)
		shouldDestroy = false
	}
	if stopPayload.Denied {
		shouldDestroy = false
	}

	if shouldDestroy {
		if destroyErr := m.destroySandbox(ctx, provider, session, state); destroyErr != nil {
			errs = append(errs, destroyErr)
		}
	} else {
		now := m.now()
		meta = cloneSessionSandboxMeta(session.Info().Sandbox)
		if meta != nil {
			meta.State = sandboxStateStopped
			session.setSandbox(meta, now)
			if err := m.persistSessionMetadataOnly(session); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errors.Join(errs...)
}

func (m *Manager) syncSandboxFromRuntime(
	ctx context.Context,
	provider envpkg.Provider,
	session *Session,
	state envpkg.SessionState,
	meta *store.SessionSandboxMeta,
	reason envpkg.SyncReason,
) error {
	return m.syncSandboxRuntime(
		ctx,
		session,
		state,
		meta,
		envpkg.SyncDirectionFromRuntime,
		reason,
		provider.SyncFromRuntime,
	)
}

func (m *Manager) destroySandbox(
	ctx context.Context,
	provider envpkg.Provider,
	session *Session,
	state envpkg.SessionState,
) error {
	meta := cloneSessionSandboxMeta(session.Info().Sandbox)
	started := time.Now()
	startEvent := sandboxEventFromMeta(meta, session.ID, session.WorkspaceID, sandboxEventDestroyStart, "")
	m.logSandboxLifecycle(startEvent)

	err := provider.Destroy(ctx, state)
	duration := time.Since(started)
	if err != nil {
		errorEvent := sandboxEventFromMeta(meta, session.ID, session.WorkspaceID, sandboxEventDestroyError, "")
		errorEvent.Duration = duration
		attachSandboxError(&errorEvent, err)
		m.logSandboxLifecycle(errorEvent)
		return fmt.Errorf("session: destroy sandbox %q for %q: %w", state.SandboxID, session.ID, err)
	}

	now := m.now()
	meta = cloneSessionSandboxMeta(meta)
	if meta != nil {
		meta.State = sandboxStateDestroyed
		session.setSandbox(meta, now)
		if err := m.persistSessionMetadataOnly(session); err != nil {
			return err
		}
	}
	completeEvent := sandboxEventFromMeta(
		meta,
		session.ID,
		session.WorkspaceID,
		sandboxEventDestroyComplete,
		"",
	)
	completeEvent.Duration = duration
	m.logSandboxLifecycle(completeEvent)
	return nil
}

func (m *Manager) logSandboxTransport(session *Session, eventName string, err error, duration time.Duration) {
	if session == nil {
		return
	}
	meta := cloneSessionSandboxMeta(session.Info().Sandbox)
	event := sandboxEventFromMeta(meta, session.ID, session.WorkspaceID, eventName, "")
	event.Duration = duration
	if err != nil {
		attachSandboxError(&event, err)
	}
	m.logSandboxLifecycle(event)
}

func (m *Manager) logSandboxLifecycle(event SandboxLifecycleEvent) {
	event.Name = strings.TrimSpace(event.Name)
	if event.Name == "" {
		return
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = m.now()
	}
	if event.Span == "" {
		event.Span = sandboxSpanForEvent(event.Name, event.Reason)
	}
	logger := m.logger
	if logger == nil {
		logger = slog.Default()
	}

	args := []any{
		"backend", strings.TrimSpace(event.Backend),
		"profile", strings.TrimSpace(event.Profile),
		"sandbox_id", strings.TrimSpace(event.SandboxID),
		"instance_id", strings.TrimSpace(event.InstanceID),
		workspaceIDFieldKey, strings.TrimSpace(event.WorkspaceID),
		sessionIDFieldKey, strings.TrimSpace(event.SessionID),
		"duration_ms", event.Duration.Milliseconds(),
	}
	if strings.TrimSpace(event.Reason) != "" {
		args = append(args, "reason", strings.TrimSpace(event.Reason))
	}
	if strings.TrimSpace(event.ErrorKind) != "" {
		args = append(args, "error_kind", strings.TrimSpace(event.ErrorKind))
	}
	if strings.TrimSpace(event.Error) != "" {
		args = append(args, "error", strings.TrimSpace(event.Error))
	}
	if strings.Contains(event.Name, ".error") {
		logger.Warn(event.Name, args...)
	} else {
		logger.Info(event.Name, args...)
	}

	if notifier, ok := m.notifier.(SandboxLifecycleNotifier); ok {
		notifier.OnSandboxLifecycleEvent(m.lifecycleCtx, event)
	}
}
