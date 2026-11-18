package session

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/compozy/compozy/internal/store"
)

// ErrSessionProfileMoveOpen refuses deleting a session whose move to another profile is
// still journaled: the journal is what lets recovery release the move's checkouts.
var ErrSessionProfileMoveOpen = errors.New("session: a move to another profile is not settled yet")

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
		if err := ctx.Err(); err != nil {
			// The rest stay journaled and settle at the next boot.
			return errors.Join(append(errs, err)...)
		}
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
	lockedCtx, unlock, err := m.lockProfileMove(ctx, move.SessionID)
	if err != nil {
		return false, err
	}
	defer unlock()
	return m.settleProfileMove(lockedCtx, move)
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
				"an earlier move of session %s is not settled yet (%v); the session is kept%s",
				sessionID, err, profileMoveBranchNote(&move),
			),
			cause: err,
		}
	}
	return nil
}

// settleProfileMove finishes an open move from durable state alone. A copy whose
// derive receipt committed completes the move, while that copy still exists: a copy
// deleted before the journal closed leaves nothing to keep, so the move rolls back
// and the original is available again. Otherwise it rolls back in reverse
// order: the target checkout is released, the branch is checked out again for the
// source profile when its checkout is gone, and the original is unarchived and
// rebound to it. A binding the original's owner changed before the archive is left
// as they set it: its checkout was never released. Every step reads current state
// first, so a settle that a stop interrupted is repeated safely. The journal is
// closed once nothing is left; a close that fails is reported, so the move is not
// taken as settled while its row remains.
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
	receipt, found, err := derivations.SessionDerivationReceipt(
		ctx, move.WorkspaceID, move.CopyKey, move.TargetProfileID,
	)
	if err != nil {
		return false, fmt.Errorf("read the copy's derive receipt: %w", err)
	}
	if found && receipt.ChildDeletedAt == nil {
		if err := m.recordCompletedProfileMove(ctx, &move); err != nil {
			return false, err
		}
		closed, err := journal.CloseCompletedSessionProfileMove(ctx, move.SessionID)
		if err != nil {
			return false, fmt.Errorf("close the move journal: %w", err)
		}
		if closed {
			return true, nil
		}
		// The copy was deleted after the receipt read; nothing is left to keep.
	}
	if err := m.rollbackProfileMoveState(ctx, journal, &move); err != nil {
		return false, err
	}
	if err := journal.DeleteSessionProfileMove(ctx, move.SessionID); err != nil {
		return false, fmt.Errorf("close the move journal: %w", err)
	}
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
	if err := m.restoreProfileMoveSource(ctx, &meta, move.SourceWorktreeID, restoredID, restoredID != ""); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// restoreProfileMoveSource restores the family before releasing its conversation locks.
func (m *Manager) restoreProfileMoveSource(
	ctx context.Context, meta *store.SessionMeta, previousID, restoredID string, rebind bool,
) error {
	operationCtx, unlockFamily, children, err := m.lockProfileMoveArchiveFamily(ctx, meta)
	if err != nil {
		return err
	}
	var info *Info
	var previous string
	var changed bool
	var rebound []*Info
	defer func() {
		unlockFamily()
		if changed {
			m.notifyWorktreeBindingChanged(ctx, info, previous)
		}
		for _, child := range rebound {
			m.notifyWorktreeBindingChanged(ctx, child, previousID)
		}
	}()
	current, err := m.readMetaWithContext(operationCtx, meta.ID)
	if err != nil {
		return err
	}
	if current.WorktreeTransition != nil && current.WorktreeTransition.Pending {
		if err := m.persistSessionCatalogFromMeta(operationCtx, &current); err != nil {
			return err
		}
	}
	if current.WorktreeIDValue() != meta.WorktreeIDValue() {
		return store.ErrSessionWorktreeConflict
	}
	meta = &current
	info, previous, changed, err = m.unarchiveAndRebindLocked(operationCtx, meta, "", false)
	if err != nil {
		return err
	}
	if !rebind {
		restoredID = meta.WorktreeIDValue()
	}
	// Restore children first so an interrupted unarchive retains the original binding for retry.
	rebound, err = m.rebindProfileMoveSubagents(operationCtx, children, previousID, restoredID)
	if err != nil {
		return err
	}
	if rebind {
		info, previous, changed, err = m.unarchiveAndRebindLocked(operationCtx, meta, restoredID, true)
	}
	return err
}

// unarchiveAndRebindLocked restores catalog visibility before the binding store permits an update.
func (m *Manager) unarchiveAndRebindLocked(
	ctx context.Context,
	meta *store.SessionMeta,
	restoredID string,
	rebind bool,
) (*Info, string, bool, error) {
	err := m.requireSessionUnarchived(ctx, meta.WorkspaceID, meta.ID)
	switch {
	case errors.Is(err, ErrSessionArchived):
		if _, err := m.setArchived(ctx, meta.WorkspaceID, meta.ID, false); err != nil {
			return nil, "", false, fmt.Errorf("unarchive session %s: %w", meta.ID, err)
		}
	case err != nil:
		return nil, "", false, fmt.Errorf("read the archive state of %s: %w", meta.ID, err)
	}
	if !rebind {
		return nil, "", false, nil
	}
	expected := meta.WorktreeIDValue()
	info, previous, changed, err := m.changeStoppedWorktreeBinding(ctx, meta.ID, restoredID, &expected)
	if err != nil {
		return nil, "", false, fmt.Errorf("bind session %s to worktree %s: %w", meta.ID, restoredID, err)
	}
	return info, previous, changed, nil
}

// releaseProfileMoveTarget removes the copy's checkout when its creation committed;
// the branch stays. A target that is neither ready nor gone, such as a dismissed
// creation whose cleanup did not finish, cannot be released here: the restore that
// follows either finds the branch free or reports that it is still held.
func releaseProfileMoveTarget(ctx context.Context, port ProfileMoveWorktrees, move *store.SessionProfileMove) error {
	target, err := port.InspectProfileMoveWorktree(ctx, move.WorkspaceID, move.TargetWorktreeID)
	switch {
	case errors.Is(err, ErrProfileMoveWorktreeNotFound) || (err == nil && target.Gone):
		return nil
	case err != nil:
		return fmt.Errorf("read the target worktree %s: %w", move.TargetWorktreeID, err)
	case !target.Ready:
		return nil
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
	if current == "" {
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
	case err == nil && restore.Ready && restore.Branch != move.Branch:
		return "", fmt.Errorf(
			"the restore worktree %s is on branch %s, not %s", move.RestoreWorktreeID, restore.Branch, move.Branch,
		)
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

// refuseOpenProfileMove keeps a journaled move's session until the move settles. The
// caller holds the profile-move lock, so no move can open between this read and the
// delete.
func (m *Manager) refuseOpenProfileMove(ctx context.Context, sessionID string) error {
	journal, ok := m.creationStore.(store.SessionProfileMoveStore)
	if !ok || journal == nil {
		return nil
	}
	_, open, err := journal.SessionProfileMove(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("session: read the open move of %s: %w", sessionID, err)
	}
	if open {
		return fmt.Errorf(
			"%w: session %s; it settles when the daemon restarts or the session is moved again",
			ErrSessionProfileMoveOpen, sessionID,
		)
	}
	return nil
}

func (m *Manager) profileMoveJournal() (store.SessionProfileMoveStore, error) {
	journal, ok := m.creationStore.(store.SessionProfileMoveStore)
	if !ok || journal == nil {
		return nil, errors.New("session: profile move journal is unavailable")
	}
	return journal, nil
}

// recordCompletedProfileMove retains checkout provenance before the open journal is removed.
func (m *Manager) recordCompletedProfileMove(ctx context.Context, move *store.SessionProfileMove) error {
	if move.SourceWorktreeID == "" {
		return nil
	}
	lockedCtx, unlock, err := m.lockConversationOperation(ctx, move.SessionID)
	if err != nil {
		return err
	}
	defer unlock()
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	meta, err := m.readSessionMetaReadOnly(lockedCtx, move.SessionID)
	if errors.Is(err, ErrSessionNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	meta.CompletedProfileMove = new(*move)
	return store.WriteSessionMeta(store.SessionMetaFile(filepath.Join(m.homePaths.SessionsDir, meta.ID)), &meta)
}
