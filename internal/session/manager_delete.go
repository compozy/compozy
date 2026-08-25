package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

// Delete removes one session from active runtime state and persisted history.
func (m *Manager) Delete(ctx context.Context, id string) error {
	if m == nil {
		return errors.New("session: manager is required")
	}
	if ctx == nil {
		return errors.New("session: delete context is required")
	}

	target, err := normalizeStoredSessionID(id)
	if err != nil {
		return fmt.Errorf("session: normalize delete id %q: %w", id, err)
	}
	ctx, unlockConversation, err := m.lockConversationOperation(ctx, target)
	if err != nil {
		return err
	}
	defer unlockConversation()

	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()

	staged, err := m.stageSessionDelete(ctx, target, true)
	if err != nil {
		return err
	}

	if m.sessionCatalog != nil {
		if err := m.sessionCatalog.DeleteSession(ctx, target); err != nil {
			deleteErr := fmt.Errorf("session: delete catalog state for %q: %w", target, err)
			if rollbackErr := m.rollbackStagedSessionDeletes(ctx, []stagedSessionDelete{staged}); rollbackErr != nil {
				return errors.Join(deleteErr, rollbackErr)
			}
			return deleteErr
		}
	}
	if m.sessionWindowReconciler != nil && staged.info != nil {
		if reconcileErr := m.reconcileDeletedSessionWindows(
			context.WithoutCancel(ctx),
			staged.info.ProfileID,
			staged.info.WorkspaceID,
			target,
		); reconcileErr != nil {
			staged.windowReconciliationPending = true
			m.logger.Warn(
				"session: retained deletion tombstone for window reconciliation retry",
				"session_id", target,
				"workspace_id", staged.info.WorkspaceID,
				"error", reconcileErr,
			)
		}
	}
	if cleanupErr := m.commitStagedSessionDeletes(ctx, []stagedSessionDelete{staged}); cleanupErr != nil {
		// The catalog deletion is already committed. The tombstone is deliberately
		// retained and retried during the next manager construction, while the
		// logical deletion remains successful and cannot lose partially restored
		// audit history.
		m.logger.Warn(
			"session: deferred deletion tombstone cleanup",
			"session_id", target,
			"error", cleanupErr,
		)
	}
	if staged.windowReconciliationPending {
		m.scheduleDeletedSessionWindowRetry(deletedSessionWindowRetryPath(staged))
	}
	return nil
}

func (m *Manager) quiesceSessionQueries(
	ctx context.Context,
	owner store.SessionDBOwner,
	sessionDir string,
) (func(), error) {
	if m.queryStoreRuntime == nil {
		return func() {}, nil
	}
	return m.queryStoreRuntime.Quiesce(ctx, owner, store.SessionDBFile(sessionDir))
}

func stopSessionBeforeDelete(
	ctx context.Context,
	target string,
	stop func(context.Context, string, StopCause, string) error,
) error {
	err := stop(ctx, target, CauseUserRequested, "session deleted")
	if err == nil || errors.Is(err, ErrSessionNotFound) {
		return nil
	}
	return err
}
