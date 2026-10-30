package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// ErrDeleteStopAccepted marks a delete failure after its stop request was accepted.
// Callers may retain cleanup only when this marker accompanies a canceled wait.
var ErrDeleteStopAccepted = errors.New("session: delete stop accepted")

// Delete removes one session from active runtime state and persisted history.
func (m *Manager) Delete(ctx context.Context, id string) (err error) {
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
	defer func() {
		if err != nil {
			m.logger.Error("session: deletion failed", "session_id", target, "error", err)
		}
	}()
	ctx, unlockConversation, err := m.lockConversationOperation(ctx, target)
	if err != nil {
		return err
	}
	defer unlockConversation()

	// Stop settlement and its subagent/Goal callbacks can acquire lifecycleMu.
	// Keep them outside the global staging lock. The conversation operation
	// prevents resume, another deletion, or workspace removal from racing us.
	info, err := m.prepareSessionDelete(ctx, target)
	if err != nil {
		return err
	}

	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()

	staged, err := m.stageSessionDirectoryDelete(ctx, target, info)
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
	m.replyWatchSessionDeleted(ctx, target)
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

// prepareSessionDelete settles execution and owned Goals before reversible history removal.
// The caller must own the conversation operation, without holding lifecycleMu.
func (m *Manager) prepareSessionDelete(ctx context.Context, target string) (*Info, error) {
	// Restore durable stop settlement before deciding whether deletion can proceed.
	if _, active := m.Get(target); !active {
		if _, err := m.reconcileInactiveSession(ctx, target); err != nil {
			return nil, err
		}
	}
	info, err := m.Status(ctx, target)
	if err != nil {
		return nil, err
	}
	if m.isPending(target) {
		return nil, fmt.Errorf("session: stage %q: %w", target, workspacepkg.ErrWorkspaceHasActiveSessions)
	}
	_, active := m.Get(target)
	if active || m.hasPendingStopSettlement(target) {
		if err := stopSessionBeforeDelete(ctx, target, m.stopWithDeleteReceipt); err != nil {
			return nil, fmt.Errorf("session: stop %q before delete: %w", target, err)
		}
		info, err = m.Status(ctx, target)
		if err != nil {
			return nil, fmt.Errorf("session: read %q after stop for delete: %w", target, err)
		}
	}
	if m.hasPendingStopSettlement(target) {
		return nil, fmt.Errorf("%w: recovered stop for %s has pending persistence", ErrRecoveryPersistence, target)
	}
	if info.State == StateStopping {
		return nil, fmt.Errorf("%w: cannot delete %s before process exit", ErrStopVerificationFailed, target)
	}
	// A staging rollback restores stopped history, never the process or its
	// canceled Goal. Also settle Goals left live by older versions for a stopped session.
	stopInfo := *info
	stopInfo.StopCause = CauseUserRequested
	if err := m.removeSessionGoals(ctx, &stopInfo); err != nil {
		return nil, fmt.Errorf("session: cancel Goals before delete: %w", err)
	}
	return info, nil
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

func (m *Manager) stopWithDeleteReceipt(ctx context.Context, id string, cause StopCause, detail string) error {
	accepted, err := m.stopWithCauseReceipt(ctx, id, cause, detail)
	if accepted && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
		return errors.Join(ErrDeleteStopAccepted, err)
	}
	return err
}
