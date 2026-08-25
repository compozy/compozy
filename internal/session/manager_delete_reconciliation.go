package session

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	deletedSessionWindowRetryInitialDelay = 250 * time.Millisecond
	deletedSessionWindowRetryMaxDelay     = 5 * time.Second
)

var errDeletedSessionWindowReconciliationPending = errors.New(
	"session: deleted-session window reconciliation is pending",
)

func (m *Manager) reconcileDeletedSessionWindows(
	ctx context.Context,
	profileID string,
	workspaceID string,
	sessionID string,
) error {
	if m == nil || m.sessionWindowReconciler == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("session: deleted-session window reconciliation context is required")
	}
	attemptCtx, cancel := context.WithTimeout(ctx, defaultLifecycleTimeout)
	defer cancel()
	if err := m.sessionWindowReconciler.ReconcileDeletedSession(
		attemptCtx,
		profileID,
		workspaceID,
		sessionID,
	); err != nil {
		return fmt.Errorf("session: reconcile deleted-session windows for %q: %w", sessionID, err)
	}
	return nil
}

func (m *Manager) scheduleDeletedSessionWindowRetry(committedPath string) {
	if m == nil || m.sessionWindowReconciler == nil || strings.TrimSpace(committedPath) == "" {
		return
	}
	base := m.windowReconciliationCtx
	if base == nil {
		base = m.lifecycleCtx
	}
	if base == nil {
		base = context.Background()
	}
	m.windowReconciliationMu.Lock()
	if m.windowReconciliationClosed {
		m.windowReconciliationMu.Unlock()
		return
	}
	m.windowReconciliationWG.Add(1)
	m.windowReconciliationMu.Unlock()
	go func() {
		defer m.windowReconciliationWG.Done()
		delay := deletedSessionWindowRetryInitialDelay
		for {
			if !waitForDeletedSessionWindowRetry(base, delay) {
				return
			}
			err := m.retryCommittedSessionDeleteTombstone(base, committedPath)
			if err == nil {
				return
			}
			if (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) && base.Err() != nil {
				return
			}
			logger := m.logger
			if logger == nil {
				logger = slog.Default()
			}
			logger.Warn(
				"session: deleted-session window reconciliation retry failed",
				"path", committedPath,
				"error", err,
			)
			if delay < deletedSessionWindowRetryMaxDelay {
				delay *= 2
				if delay > deletedSessionWindowRetryMaxDelay {
					delay = deletedSessionWindowRetryMaxDelay
				}
			}
		}
	}()
}

func deletedSessionWindowRetryPath(entry stagedSessionDelete) string {
	if _, err := os.Stat(entry.committedPath); err == nil {
		return entry.committedPath
	} else if !errors.Is(err, os.ErrNotExist) {
		return entry.committedPath
	}
	if _, err := os.Stat(entry.stagedPath); err == nil {
		return entry.stagedPath
	} else if !errors.Is(err, os.ErrNotExist) {
		return entry.stagedPath
	}
	return entry.committedPath
}

func (m *Manager) stopDeletedSessionWindowReconciliation() {
	if m == nil {
		return
	}
	m.windowReconciliationMu.Lock()
	m.windowReconciliationClosed = true
	if m.windowReconciliationCancel != nil {
		m.windowReconciliationCancel()
	}
	m.windowReconciliationMu.Unlock()
}

func (m *Manager) waitForDeletedSessionWindowReconciliation() {
	if m == nil {
		return
	}
	m.windowReconciliationWG.Wait()
}

func (m *Manager) retryCommittedSessionDeleteTombstone(ctx context.Context, path string) error {
	if ctx == nil {
		return errors.New("session: deleted-session window retry context is required")
	}
	name := filepath.Base(path)
	attemptCtx, cancel := context.WithTimeout(ctx, defaultLifecycleTimeout)
	defer cancel()
	err := m.cleanupSessionDeleteTombstoneWithContext(attemptCtx, path, name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func waitForDeletedSessionWindowRetry(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
