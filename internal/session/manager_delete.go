package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// ErrDeleteStopAccepted marks a delete failure after its stop request was accepted.
// Callers may retain cleanup only when this marker accompanies a canceled wait.
var ErrDeleteStopAccepted = errors.New("session: delete stop accepted")

// Delete removes one session from active runtime state and persisted history.
func (m *Manager) Delete(ctx context.Context, id string) error {
	return m.deleteSession(ctx, id, nil)
}

// DeleteArchived removes one session only while it is archived and owned by the
// expected profile and workspace: it is refused before anything stops, and the
// catalog row goes only if the deleting transaction still finds it archived, so
// a session restored meanwhile is never deleted.
func (m *Manager) DeleteArchived(ctx context.Context, target store.ArchivedSessionDelete) error {
	return m.deleteSession(ctx, target.SessionID, &target)
}

func (m *Manager) deleteSession(ctx context.Context, id string, archived *store.ArchivedSessionDelete) (err error) {
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
			return
		}
		m.settledStatus.forget(target)
	}()
	ctx, unlock, err := m.lockSessionDelete(ctx, target)
	if err != nil {
		return err
	}
	finishSubagent := func(bool) {}
	defer func() {
		unlock()
		if finishSubagent != nil {
			finishSubagent(err == nil)
		}
	}()
	if err := m.guardInstanceMain(ctx, target); err != nil {
		return err
	}
	if archived != nil {
		if err := m.requireArchivedDelete(ctx, target, *archived); err != nil {
			return err
		}
	}
	if handled, finish, deleteErr := m.deleteMissingSession(ctx, target, archived); handled {
		finishSubagent = finish
		return deleteErr
	}
	if archived != nil {
		if err := m.requireArchivedDelete(ctx, target, *archived); err != nil {
			return err
		}
	}

	// Stop settlement and its subagent/Goal callbacks can acquire lifecycleMu.
	// Keep them outside the global staging lock. The conversation operation
	// prevents resume, another deletion, or workspace removal from racing us.
	info, err := m.prepareSessionDelete(ctx, target)
	if err != nil {
		return err
	}

	finishSubagent, err = m.prepareSubagentSessionDelete(ctx, target)
	if err != nil {
		return err
	}
	// Source admission must succeed before any side history is removed.
	// Side deletes acquire their own lifecycle locks before source staging does.
	if err := m.reconcileBudgetSessionBeforeDeletion(ctx, target); err != nil {
		return err
	}
	if err := m.deleteSideSessions(ctx, target); err != nil {
		return err
	}

	return m.deletePreparedSession(ctx, target, info, archived)
}

func (m *Manager) deletePreparedSession(
	ctx context.Context, target string, info *Info, archived *store.ArchivedSessionDelete,
) error {
	if err := m.lifecycleMu.LockContext(ctx); err != nil {
		return fmt.Errorf("session: await delete lifecycle lock: %w", err)
	}
	defer m.lifecycleMu.Unlock()

	staged, err := m.stageSessionDirectoryDelete(ctx, target, info)
	if err != nil {
		return err
	}

	if m.sessionCatalog != nil {
		if err := m.deleteSessionCatalogRow(ctx, target, archived); err != nil {
			deleteErr := fmt.Errorf("session: delete catalog state for %q: %w", target, err)
			if rollbackErr := m.rollbackStagedSessionDeletes(ctx, []stagedSessionDelete{staged}); rollbackErr != nil {
				return errors.Join(deleteErr, rollbackErr)
			}
			return deleteErr
		}
	}
	m.replyWatchSessionDeleted(ctx, target)
	m.reconcileStagedSessionDeleteWindows(ctx, &staged)
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

func (m *Manager) reconcileBudgetSessionBeforeDeletion(ctx context.Context, target string) error {
	if ready, ok := m.notifier.(interface {
		ReconcileBudgetSessionBeforeDeletion(context.Context, string) error
	}); ok {
		if err := ready.ReconcileBudgetSessionBeforeDeletion(ctx, target); err != nil {
			return fmt.Errorf("session: budget ledger is not safe to delete: %w", err)
		}
	}
	return nil
}

func (m *Manager) deleteMissingSession(
	ctx context.Context, target string, archived *store.ArchivedSessionDelete,
) (bool, func(bool), error) {
	finishSubagent := func(bool) {}
	if m.sessionCatalog == nil {
		return false, finishSubagent, nil
	}
	if _, active := m.Get(target); active || m.isPending(target) || m.hasPendingStopSettlement(target) {
		return false, finishSubagent, nil
	}
	metaPath := store.SessionMetaFile(filepath.Join(m.homePaths.SessionsDir, target))
	if _, err := os.Lstat(metaPath); !errors.Is(err, os.ErrNotExist) {
		return false, finishSubagent, nil
	}
	info, err := m.durableSessionInfoForLifecycleTransition(ctx, target)
	if err != nil {
		return true, finishSubagent, err
	}
	if info.State != StateStopped {
		return true, finishSubagent, fmt.Errorf(
			"%w: cannot delete missing session %s before process exit",
			ErrStopVerificationFailed,
			target,
		)
	}
	if archived != nil {
		if err := validateArchivedDelete(info, *archived); err != nil {
			return true, finishSubagent, err
		}
	}
	// The metadata and ledger may both have disappeared. This catalog-only
	// delete must still refuse to erase a budgeted, unreconciled session;
	// otherwise the next restart could silently clear the recovery pause.
	if err := m.reconcileBudgetSessionBeforeDeletion(ctx, target); err != nil {
		return true, finishSubagent, err
	}
	stopInfo := *info
	stopInfo.StopCause = CauseUserRequested
	if err := m.removeSessionGoals(ctx, &stopInfo); err != nil {
		return true, finishSubagent, fmt.Errorf("session: cancel Goals before delete: %w", err)
	}
	finishSubagent, err = m.prepareSubagentSessionDelete(ctx, target)
	if err != nil {
		return true, finishSubagent, err
	}
	// Missing history still requires a stopped, eligible source before the cascade.
	// Keep the cascade outside lifecycleMu, as in the live-history path.
	if err := m.deleteSideSessions(ctx, target); err != nil {
		return true, finishSubagent, err
	}
	if _, err := os.Lstat(store.SessionDBFile(filepath.Join(m.homePaths.SessionsDir, target))); err == nil {
		return true, finishSubagent, m.deletePreparedSession(ctx, target, info, archived)
	} else if !errors.Is(err, os.ErrNotExist) {
		return true, finishSubagent, err
	}
	if err := m.lifecycleMu.LockContext(ctx); err != nil {
		return true, finishSubagent, fmt.Errorf("session: await delete lifecycle lock: %w", err)
	}
	defer m.lifecycleMu.Unlock()
	if err := m.deleteSessionCatalogRow(ctx, target, archived); err != nil {
		return true, finishSubagent, err
	}
	m.replyWatchSessionDeleted(ctx, target)
	m.publishCommittedSessionDelete(&stagedSessionDelete{info: info})
	if err := m.reconcileDeletedSessionWindows(
		context.WithoutCancel(ctx),
		info.ProfileID,
		info.WorkspaceID,
		target,
	); err != nil {
		m.logger.Warn("session: reconcile deleted missing session windows", "session_id", target, "error", err)
	}
	return true, finishSubagent, nil
}

// requireArchivedDelete refuses an archived-only delete of a session that was
// restored, moved, or never matched the expected owner.
func (m *Manager) requireArchivedDelete(
	ctx context.Context,
	target string,
	archived store.ArchivedSessionDelete,
) error {
	var info *Info
	var err error
	if m.sessionCatalog != nil {
		info, err = m.durableSessionInfoForLifecycleTransition(ctx, target)
	} else {
		info, err = m.Status(ctx, target)
	}
	if err != nil {
		return err
	}
	return validateArchivedDelete(info, archived)
}

func validateArchivedDelete(info *Info, archived store.ArchivedSessionDelete) error {
	if info.ArchivedAt == nil ||
		strings.TrimSpace(info.ProfileID) != strings.TrimSpace(archived.ProfileID) ||
		strings.TrimSpace(info.WorkspaceID) != strings.TrimSpace(archived.WorkspaceID) {
		return fmt.Errorf("session: delete %q: %w", info.ID, store.ErrSessionNotArchived)
	}
	return nil
}

func (m *Manager) deleteSessionCatalogRow(
	ctx context.Context,
	target string,
	archived *store.ArchivedSessionDelete,
) error {
	if archived == nil {
		return m.sessionCatalog.DeleteSession(ctx, target)
	}
	cleanupStore, ok := m.sessionCatalog.(store.SessionHistoryCleanupStore)
	if !ok {
		return ErrSessionArchiveUnavailable
	}
	guarded := *archived
	guarded.SessionID = target
	return cleanupStore.DeleteArchivedSession(ctx, guarded)
}

// prepareSessionDelete settles execution and owned Goals before reversible history removal.
// The caller must own the conversation operation, without holding lifecycleMu.
// lockSessionDelete takes the session's profile-move lock, refuses a session whose
// move is still journaled, then takes its conversation lock. A move takes the same
// two locks in that order.
func (m *Manager) lockSessionDelete(ctx context.Context, target string) (context.Context, func(), error) {
	ctx, unlockMove, err := m.lockProfileMove(ctx, target)
	if err != nil {
		return nil, nil, err
	}
	if err := m.refuseOpenProfileMove(ctx, target); err != nil {
		unlockMove()
		return nil, nil, err
	}
	ctx, unlockConversation, err := m.lockConversationOperation(ctx, target)
	if err != nil {
		unlockMove()
		return nil, nil, err
	}
	return ctx, func() {
		unlockConversation()
		unlockMove()
	}, nil
}

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
	timeout := m.queryQuiesceTimeout
	if timeout <= 0 {
		timeout = defaultLifecycleTimeout
	}
	// Keep staging and rollback on the caller's original lifetime.
	quiesceCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return m.queryStoreRuntime.Quiesce(quiesceCtx, owner, store.SessionDBFile(sessionDir))
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
