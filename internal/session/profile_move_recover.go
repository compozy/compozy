package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

// ErrProfileMoveWorktreeNotFound reports a worktree id no row was ever recorded for.
// The move's journal chooses ids before creating them, so a missing row means the
// step that creates it never committed.
var ErrProfileMoveWorktreeNotFound = errors.New("session: profile move worktree not found")

// RecoverProfileMoves settles every move a daemon stop left open: a move whose copy
// committed is complete, and any other is rolled back, so the original ends up
// unarchived and bound to a checkout of its branch. It runs at boot, after worktree
// recovery has finished or reverted the removals and creations the stop
// interrupted. A move that cannot settle yet stays journaled for the next attempt.
func (m *Manager) RecoverProfileMoves(ctx context.Context) error {
	if m == nil {
		return errors.New("session: manager is required")
	}
	journal, err := m.profileMoveJournal()
	if err != nil {
		return err
	}
	moves, err := journal.ListSessionProfileMoves(ctx)
	if err != nil {
		return fmt.Errorf("session: list open profile moves: %w", err)
	}
	var errs []error
	for i := range moves {
		move := moves[i]
		completed, err := m.settleProfileMoveLocked(ctx, move)
		if err != nil {
			m.logger.Error(
				"session.profile_move.recovery_incomplete",
				"session_id", move.SessionID, "branch", move.Branch, "error", err,
			)
			errs = append(errs, fmt.Errorf("session: recover move of %s: %w", move.SessionID, err))
			continue
		}
		m.logger.Info(
			"session.profile_move.recovered",
			"session_id", move.SessionID, "completed", completed, "to_profile_id", move.TargetProfileID,
		)
	}
	return errors.Join(errs...)
}

func (m *Manager) settleProfileMoveLocked(ctx context.Context, move store.SessionProfileMove) (bool, error) {
	unlock := m.lockProfileMove(move.WorkspaceID, move.SessionID)
	defer unlock()
	return m.settleProfileMove(ctx, move)
}

// settleOpenProfileMove settles a move of the session that is still journaled
// before a new move plans, so a retry first finishes the interrupted one.
func (m *Manager) settleOpenProfileMove(ctx context.Context, sessionID string) error {
	journal, err := m.profileMoveJournal()
	if err != nil {
		return err
	}
	move, found, err := journal.SessionProfileMove(ctx, sessionID)
	if err != nil || !found {
		return err
	}
	if _, err := m.settleProfileMove(ctx, move); err != nil {
		return &ProfileMoveError{
			Reason: ProfileMoveRefusalRecoveryIncomplete,
			Message: fmt.Sprintf(
				"an earlier move of session %s did not finish its rollback (%v); the session is kept%s",
				sessionID, err, profileMoveBranchNote(&move),
			),
			cause: err,
		}
	}
	return nil
}

// settleProfileMove finishes an open move from durable state alone. A copy whose
// derive receipt committed completes the move. Otherwise it rolls back in reverse
// order: the target checkout is released, the branch is checked out again for the
// source profile when its checkout is gone, and the original is unarchived and
// rebound to it. Every step reads current state first, so a settle that a stop
// interrupted is repeated safely. The journal is closed once nothing is left.
func (m *Manager) settleProfileMove(ctx context.Context, move store.SessionProfileMove) (bool, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), profileMoveTimeout)
	defer cancel()
	journal, err := m.profileMoveJournal()
	if err != nil {
		return false, err
	}
	derivations, err := m.derivationStore()
	if err != nil {
		return false, err
	}
	_, copied, err := derivations.SessionDerivationReceipt(ctx, move.WorkspaceID, move.CopyKey, move.TargetProfileID)
	if err != nil {
		return false, fmt.Errorf("read the copy's derive receipt: %w", err)
	}
	if copied {
		m.closeProfileMove(ctx, journal, move.SessionID)
		return true, nil
	}
	if err := m.rollbackProfileMoveState(ctx, journal, &move); err != nil {
		return false, err
	}
	m.closeProfileMove(ctx, journal, move.SessionID)
	return false, nil
}

func (m *Manager) rollbackProfileMoveState(
	ctx context.Context,
	journal store.SessionProfileMoveStore,
	move *store.SessionProfileMove,
) error {
	var port ProfileMoveWorktrees
	if move.SourceWorktreeID != "" {
		var ok bool
		if port, ok = m.worktreeResolver.(ProfileMoveWorktrees); !ok || port == nil {
			return errors.New("worktree release is unavailable")
		}
		// The branch can be checked out once, so the copy's checkout goes first.
		if err := releaseProfileMoveTarget(ctx, port, move); err != nil {
			return err
		}
	}
	meta, err := m.readSessionMetaReadOnly(ctx, move.SessionID)
	if errors.Is(err, ErrSessionNotFound) {
		// The original was deleted meanwhile; nothing is left to restore.
		return nil
	}
	if err != nil {
		return err
	}
	var errs []error
	restoredID := ""
	if port != nil {
		if restoredID, err = m.restoreProfileMoveCheckout(ctx, port, journal, move, &meta); err != nil {
			errs = append(errs, err)
		}
	}
	// The caller holds the lock the public Unarchive takes.
	if err := m.requireSessionUnarchived(ctx, meta.WorkspaceID, meta.ID); errors.Is(err, ErrSessionArchived) {
		if _, err := m.setArchived(ctx, meta.WorkspaceID, meta.ID, false); err != nil {
			return errors.Join(append(errs, fmt.Errorf("unarchive session %s: %w", meta.ID, err))...)
		}
	} else if err != nil {
		return errors.Join(append(errs, fmt.Errorf("read the archive state of %s: %w", meta.ID, err))...)
	}
	if restoredID != "" {
		expected := meta.WorktreeIDValue()
		if _, err := m.ChangeWorktreeBinding(ctx, meta.ID, restoredID, &expected); err != nil {
			errs = append(errs, fmt.Errorf("bind session %s to worktree %s: %w", meta.ID, restoredID, err))
		}
	}
	return errors.Join(errs...)
}

// releaseProfileMoveTarget removes the copy's checkout when its creation committed;
// the branch stays.
func releaseProfileMoveTarget(ctx context.Context, port ProfileMoveWorktrees, move *store.SessionProfileMove) error {
	target, err := port.InspectProfileMoveWorktree(ctx, move.WorkspaceID, move.TargetWorktreeID)
	switch {
	case errors.Is(err, ErrProfileMoveWorktreeNotFound) || (err == nil && target.Gone):
		return nil
	case err != nil:
		return fmt.Errorf("read the target worktree %s: %w", move.TargetWorktreeID, err)
	case !target.Ready:
		return fmt.Errorf("the target worktree %s is still changing", move.TargetWorktreeID)
	}
	if err := port.ReleaseProfileMoveWorktree(ctx, move.WorkspaceID, move.TargetWorktreeID); err != nil {
		return fmt.Errorf("release the target worktree %s: %w", move.TargetWorktreeID, err)
	}
	return nil
}

// restoreProfileMoveCheckout returns the checkout the original must be bound to, or
// "" when its current checkout still stands. A restore checkout whose creation
// failed is replaced by a fresh id, recorded before the next attempt.
func (m *Manager) restoreProfileMoveCheckout(
	ctx context.Context,
	port ProfileMoveWorktrees,
	journal store.SessionProfileMoveStore,
	move *store.SessionProfileMove,
	meta *store.SessionMeta,
) (string, error) {
	current := meta.WorktreeIDValue()
	if current == "" || current == move.RestoreWorktreeID {
		return "", nil
	}
	bound, err := port.InspectProfileMoveWorktree(ctx, move.WorkspaceID, current)
	switch {
	case err == nil && bound.Ready:
		return "", nil
	case err != nil && !errors.Is(err, ErrProfileMoveWorktreeNotFound):
		return "", fmt.Errorf("read worktree %s: %w", current, err)
	case err == nil && !bound.Gone:
		return "", fmt.Errorf("worktree %s is still changing", current)
	}
	restore, err := port.InspectProfileMoveWorktree(ctx, move.WorkspaceID, move.RestoreWorktreeID)
	switch {
	case err == nil && restore.Ready:
		return move.RestoreWorktreeID, nil
	case err == nil && restore.Gone:
		if err := m.renewProfileMoveRestoreID(ctx, journal, move); err != nil {
			return "", err
		}
	case err == nil:
		return "", fmt.Errorf("the restore worktree %s is still changing", move.RestoreWorktreeID)
	case !errors.Is(err, ErrProfileMoveWorktreeNotFound):
		return "", fmt.Errorf("read the restore worktree %s: %w", move.RestoreWorktreeID, err)
	}
	checkout := ProfileMoveCheckout{ID: move.SourceWorktreeID, Branch: move.Branch, HeadSHA: move.HeadSHA}
	id, _, err := port.CreateProfileMoveWorktree(
		ctx,
		move.WorkspaceID,
		move.ProfileID,
		move.RestoreWorktreeID,
		checkout,
	)
	if err != nil {
		return "", fmt.Errorf("check out branch %s again: %w", move.Branch, err)
	}
	return id, nil
}

func (m *Manager) renewProfileMoveRestoreID(
	ctx context.Context,
	journal store.SessionProfileMoveStore,
	move *store.SessionProfileMove,
) error {
	next, err := newProfileMoveWorktreeID()
	if err != nil {
		return err
	}
	if err := journal.SetSessionProfileMoveRestoreWorktree(ctx, move.SessionID, next); err != nil {
		return err
	}
	move.RestoreWorktreeID = next
	return nil
}

// closeProfileMove deletes a settled move's journal row. A delete that fails only
// leaves the row for the next boot, whose settle finds nothing left to do.
func (m *Manager) closeProfileMove(ctx context.Context, journal store.SessionProfileMoveStore, sessionID string) {
	if err := journal.DeleteSessionProfileMove(ctx, sessionID); err != nil {
		m.logger.Warn("session.profile_move.journal_close_failed", "session_id", sessionID, "error", err)
	}
}

func (m *Manager) profileMoveJournal() (store.SessionProfileMoveStore, error) {
	journal, ok := m.creationStore.(store.SessionProfileMoveStore)
	if !ok || journal == nil {
		return nil, errors.New("session: profile move journal is unavailable")
	}
	return journal, nil
}
