package extensionpkg

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/resources"
)

type pendingExtensionCleanup struct {
	key                InstanceKey
	process            processHandle
	redactionCleanups  []func()
	source             resources.ResourceSource
	sourceSessionNonce string
}

func processTerminal(proc processHandle) bool {
	if proc == nil {
		return true
	}
	select {
	case <-proc.Done():
		return true
	default:
		return false
	}
}

func (m *Manager) retainPendingCleanup(cleanup pendingExtensionCleanup) {
	if m == nil || (cleanup.process == nil && cleanup.sourceSessionNonce == "") {
		return
	}
	cleanup.key = cleanup.key.Normalize()
	cleanup.source = cleanup.source.Normalize()
	m.mu.Lock()
	m.pendingCleanups = append(m.pendingCleanups, cleanup)
	m.mu.Unlock()
}

func (m *Manager) retryPendingCleanups(ctx context.Context) error {
	if m == nil {
		return nil
	}

	m.mu.Lock()
	pending := m.pendingCleanups
	m.pendingCleanups = nil
	m.mu.Unlock()

	var (
		errs      []error
		remaining []pendingExtensionCleanup
	)
	for _, cleanup := range pending {
		if cleanup.process != nil {
			if err := shutdownProcessWithTimeout(ctx, cleanup.process, m.defaultShutdownTimeout); err != nil {
				errs = append(errs, fmt.Errorf(
					"extension %q pending process cleanup: %w",
					cleanup.key.runtimeID(),
					err,
				))
			}
			if processTerminal(cleanup.process) {
				runExtensionRedactionCleanups(cleanup.redactionCleanups)
				cleanup.process = nil
				cleanup.redactionCleanups = nil
			}
		}

		if cleanup.sourceSessionNonce != "" {
			if err := m.resetExtensionResourceSourceForCleanup(
				ctx,
				cleanup.source,
				cleanup.sourceSessionNonce,
			); err != nil {
				errs = append(errs, fmt.Errorf(
					"extension %q pending source cleanup: %w",
					cleanup.key.runtimeID(),
					err,
				))
			} else {
				cleanup.sourceSessionNonce = ""
			}
		}

		if cleanup.process != nil || cleanup.sourceSessionNonce != "" {
			remaining = append(remaining, cleanup)
		}
	}
	if len(remaining) > 0 {
		m.mu.Lock()
		m.pendingCleanups = append(m.pendingCleanups, remaining...)
		m.mu.Unlock()
	}
	return errors.Join(errs...)
}

func (m *Manager) resetExtensionResourceSourceForCleanup(
	ctx context.Context,
	source resources.ResourceSource,
	sessionNonce string,
) error {
	if m == nil {
		return ErrManagerRequired
	}
	if ctx == nil {
		return ErrContextRequired
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), m.defaultShutdownTimeout)
	defer cancel()
	return m.resetExtensionResourceSourceIfActiveSession(cleanupCtx, source, sessionNonce)
}

func (m *Manager) resetExtensionResourceSourceOrRetain(
	ctx context.Context,
	key InstanceKey,
	source resources.ResourceSource,
	sessionNonce string,
) error {
	if sessionNonce == "" {
		return nil
	}
	err := m.resetExtensionResourceSourceForCleanup(ctx, source, sessionNonce)
	if err == nil {
		return nil
	}
	m.retainPendingCleanup(pendingExtensionCleanup{
		key:                key,
		source:             source,
		sourceSessionNonce: sessionNonce,
	})
	return err
}
