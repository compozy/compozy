package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/worktree"
)

// profileMoveTimeout bounds the steps after the archive, which finish (or roll back)
// on their own lifetime so a disconnected caller cannot strand a move halfway.
const profileMoveTimeout = 5 * time.Minute

// applyProfileMove journals the move, archives the source, moves its checkout, and
// forks the copy into the target profile. Archive comes first: an archived session
// cannot resume, so nothing reopens the source while its worktree is released. The
// journal row is written before the archive, so a daemon stop at any later step is
// settled at the next boot.
func (m *Manager) applyProfileMove(ctx context.Context, plan *profileMovePlan) (MoveToProfileResult, error) {
	meta := &plan.meta
	if err := ctx.Err(); err != nil {
		return MoveToProfileResult{}, err
	}
	journal, err := m.profileMoveJournal()
	if err != nil {
		return MoveToProfileResult{}, err
	}
	move, err := m.newProfileMoveRecord(plan)
	if err != nil {
		return MoveToProfileResult{}, err
	}
	// From the journal on, the move finishes or rolls back on its own lifetime.
	execCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), profileMoveTimeout)
	defer cancel()
	if err := journal.InsertSessionProfileMove(execCtx, move); err != nil {
		return MoveToProfileResult{}, fmt.Errorf("session: journal move of %s: %w", meta.ID, err)
	}
	original, err := m.Archive(execCtx, meta.WorkspaceID, meta.ID)
	if err != nil {
		return MoveToProfileResult{}, m.rollbackFailedArchive(execCtx, plan, &move, err)
	}
	if err := m.requireProfileMoveBinding(execCtx, plan); err != nil {
		return MoveToProfileResult{}, m.rollbackProfileMove(execCtx, plan, &move, err)
	}
	relocation := &deriveRelocation{profileID: plan.opts.TargetProfileID}
	if plan.checkout != nil {
		err := plan.worktree.ReleaseProfileMoveWorktree(execCtx, meta.WorkspaceID, plan.checkout.ID)
		if err != nil {
			return MoveToProfileResult{}, m.rollbackProfileMove(execCtx, plan, &move, err)
		}
		id, root, err := plan.worktree.CreateProfileMoveWorktree(
			execCtx, meta.WorkspaceID, plan.opts.TargetProfileID, move.TargetWorktreeID, *plan.checkout,
		)
		if err != nil {
			return MoveToProfileResult{}, m.rollbackProfileMove(execCtx, plan, &move, err)
		}
		relocation.worktreeID, relocation.cwd = id, root
	}
	derived, err := m.deriveSession(execCtx, deriveSpec{
		kind: store.LineageKindFork, workspaceID: meta.WorkspaceID, profileID: plan.opts.ProfileID,
		sourceID: meta.ID, key: move.CopyKey, relocation: relocation,
	})
	if err != nil && derived.ChildSessionID == "" {
		return MoveToProfileResult{}, m.rollbackProfileMove(execCtx, plan, &move, err)
	}
	if err != nil {
		// The copy committed before a later step failed; it exists, so the move stands.
		m.logger.Warn(
			"session.profile_move.copy_committed_with_error",
			"session_id", meta.ID, "copy_session_id", derived.ChildSessionID, "error", err,
		)
	}
	m.closeProfileMove(execCtx, journal, meta.ID)
	result := MoveToProfileResult{Copy: derived.Child, Original: original, WorktreeID: relocation.worktreeID}
	if plan.checkout != nil {
		result.Branch = plan.checkout.Branch
	}
	if result.Copy == nil {
		if result.Copy, err = m.Status(execCtx, derived.ChildSessionID); err != nil {
			return result, fmt.Errorf(
				"session: moved %s to copy %s, but reading the copy failed: %w", meta.ID, derived.ChildSessionID, err,
			)
		}
	}
	m.logger.Info(
		"session.profile_move.completed",
		"session_id", meta.ID, "copy_session_id", result.Copy.ID,
		"from_profile_id", plan.opts.ProfileID, "to_profile_id", plan.opts.TargetProfileID,
		"worktree_id", result.WorktreeID,
	)
	return result, nil
}

// newProfileMoveRecord chooses every id the move may create before any of them is
// created, so recovery finds each one from the journal alone.
func (m *Manager) newProfileMoveRecord(plan *profileMovePlan) (store.SessionProfileMove, error) {
	key, err := m.createSessionID("")
	if err != nil {
		return store.SessionProfileMove{}, err
	}
	move := store.SessionProfileMove{
		SessionID: plan.meta.ID, WorkspaceID: plan.meta.WorkspaceID, ProfileID: plan.opts.ProfileID,
		TargetProfileID: plan.opts.TargetProfileID, CopyKey: "profile-move:" + key,
	}
	if plan.checkout == nil {
		return move, nil
	}
	move.SourceWorktreeID, move.Branch, move.HeadSHA = plan.checkout.ID, plan.checkout.Branch, plan.checkout.HeadSHA
	if move.TargetWorktreeID, err = newProfileMoveWorktreeID(); err != nil {
		return store.SessionProfileMove{}, err
	}
	if move.RestoreWorktreeID, err = newProfileMoveWorktreeID(); err != nil {
		return store.SessionProfileMove{}, err
	}
	return move, nil
}

func newProfileMoveWorktreeID() (string, error) {
	id, err := worktree.NewID()
	if err != nil {
		return "", fmt.Errorf("session: choose a move worktree id: %w", err)
	}
	return id, nil
}

// rollbackFailedArchive keeps a failed archive harmless: when it committed before a
// later step of the archive failed, the source is unarchived again. The archive state
// is read on a fresh lifetime, and a read that fails is reported, never taken as proof
// that nothing was archived; the journal then stays open for the next boot.
func (m *Manager) rollbackFailedArchive(
	ctx context.Context,
	plan *profileMovePlan,
	move *store.SessionProfileMove,
	cause error,
) error {
	readCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), profileMoveTimeout)
	defer cancel()
	err := m.requireSessionUnarchived(readCtx, plan.meta.WorkspaceID, plan.meta.ID)
	switch {
	case err == nil:
		if journal, journalErr := m.profileMoveJournal(); journalErr == nil {
			m.closeProfileMove(readCtx, journal, plan.meta.ID)
		}
		return cause
	case errors.Is(err, ErrSessionArchived):
		return m.rollbackProfileMove(ctx, plan, move, cause)
	default:
		return &ProfileMoveError{
			Reason: ProfileMoveRefusalRecoveryIncomplete,
			Message: fmt.Sprintf(
				"archiving session %s failed (%v) and its archive state could not be read (%v); "+
					"it is unarchived when the move is retried or the daemon restarts",
				plan.meta.ID, cause, err,
			),
			cause: errors.Join(cause, err),
		}
	}
}

// requireProfileMoveBinding rereads the archived source: a worktree binding that changed
// after planning would move the wrong checkout. Once archived, no binding change can
// commit (it requires an unarchived session), so this read is final.
func (m *Manager) requireProfileMoveBinding(ctx context.Context, plan *profileMovePlan) error {
	current, err := m.readSessionMetaReadOnly(ctx, plan.meta.ID)
	if err != nil {
		return err
	}
	if current.WorktreeIDValue() != plan.meta.WorktreeIDValue() {
		return profileMoveRefused(
			ProfileMoveRefusalWorktreeChanged,
			"session %s changed worktrees while it was being moved; try again", plan.meta.ID,
		)
	}
	return nil
}

// rollbackProfileMove undoes a failed move from the state it left. The cause is
// returned as is when the rollback completes; otherwise a recovery_incomplete
// refusal names what is left, and the open journal retries it on the next move of
// the session or at the next boot.
func (m *Manager) rollbackProfileMove(
	ctx context.Context,
	plan *profileMovePlan,
	move *store.SessionProfileMove,
	cause error,
) error {
	completed, err := m.settleProfileMove(ctx, *move)
	switch {
	case err == nil && completed:
		// The copy's receipt committed although the fork reported no child; the move stands.
		return fmt.Errorf("session: moved %s, but the copy reported an error: %w", plan.meta.ID, cause)
	case err == nil:
		return cause
	}
	m.logger.Error(
		"session.profile_move.rollback_incomplete",
		"session_id", plan.meta.ID, "cause", cause, "error", err,
	)
	return &ProfileMoveError{
		Reason: ProfileMoveRefusalRecoveryIncomplete,
		Message: fmt.Sprintf(
			"moving session %s failed (%v) and its rollback did not finish (%v); the session is kept%s, "+
				"and the rollback is retried when the move is retried or the daemon restarts",
			plan.meta.ID, cause, err, profileMoveBranchNote(move),
		),
		cause: errors.Join(cause, err),
	}
}

func profileMoveBranchNote(move *store.SessionProfileMove) string {
	if move.Branch == "" {
		return ""
	}
	return fmt.Sprintf(" and branch %s keeps every commit", move.Branch)
}
