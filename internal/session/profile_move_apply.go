package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/store"
)

// profileMoveTimeout bounds the steps after the archive, which finish (or roll back)
// on their own lifetime so a disconnected caller cannot strand a move halfway.
const profileMoveTimeout = 5 * time.Minute

// profileMoveProgress records the steps a failed move has to undo.
type profileMoveProgress struct {
	released         bool
	targetWorktreeID string
}

// applyProfileMove archives the source, moves its checkout, and forks the copy into
// the target profile. Archive comes first: an archived session cannot resume, so
// nothing reopens the source while its worktree is released.
func (m *Manager) applyProfileMove(ctx context.Context, plan *profileMovePlan) (MoveToProfileResult, error) {
	meta := &plan.meta
	if err := ctx.Err(); err != nil {
		return MoveToProfileResult{}, err
	}
	// From the archive on, the move finishes or rolls back on its own lifetime.
	execCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), profileMoveTimeout)
	defer cancel()
	original, err := m.Archive(execCtx, meta.WorkspaceID, meta.ID)
	if err != nil {
		return MoveToProfileResult{}, m.rollbackFailedArchive(execCtx, plan, err)
	}
	var progress profileMoveProgress
	if err := m.requireProfileMoveBinding(execCtx, plan); err != nil {
		return MoveToProfileResult{}, m.rollbackProfileMove(ctx, plan, progress, err)
	}
	relocation := &deriveRelocation{profileID: plan.opts.TargetProfileID}
	if plan.checkout != nil {
		err := plan.worktree.ReleaseProfileMoveWorktree(execCtx, meta.WorkspaceID, plan.checkout.ID)
		if err != nil {
			progress.released = m.profileMoveCheckoutGone(execCtx, plan)
			return MoveToProfileResult{}, m.rollbackProfileMove(ctx, plan, progress, err)
		}
		progress.released = true
		id, root, err := plan.worktree.CreateProfileMoveWorktree(
			execCtx, meta.WorkspaceID, plan.opts.TargetProfileID, *plan.checkout,
		)
		if err != nil {
			return MoveToProfileResult{}, m.rollbackProfileMove(ctx, plan, progress, err)
		}
		progress.targetWorktreeID = id
		relocation.worktreeID, relocation.cwd = id, root
	}
	key, err := m.createSessionID("")
	if err != nil {
		return MoveToProfileResult{}, m.rollbackProfileMove(ctx, plan, progress, err)
	}
	derived, err := m.deriveSession(execCtx, deriveSpec{
		kind: store.LineageKindFork, workspaceID: meta.WorkspaceID, profileID: plan.opts.ProfileID,
		sourceID: meta.ID, key: "profile-move:" + key, relocation: relocation,
	})
	if err != nil && derived.ChildSessionID == "" {
		return MoveToProfileResult{}, m.rollbackProfileMove(ctx, plan, progress, err)
	}
	if err != nil {
		// The copy committed before a later step failed; it exists, so the move stands.
		m.logger.Warn(
			"session.profile_move.copy_committed_with_error",
			"session_id", meta.ID, "copy_session_id", derived.ChildSessionID, "error", err,
		)
	}
	result := MoveToProfileResult{Copy: derived.Child, Original: original, WorktreeID: relocation.worktreeID}
	if plan.checkout != nil {
		result.Branch = plan.checkout.Branch
	}
	if result.Copy == nil {
		result.Copy = m.readProfileMoveCopy(execCtx, plan, derived.ChildSessionID, relocation.worktreeID)
	}
	m.logger.Info(
		"session.profile_move.completed",
		"session_id", meta.ID, "copy_session_id", result.Copy.ID,
		"from_profile_id", plan.opts.ProfileID, "to_profile_id", plan.opts.TargetProfileID,
		"worktree_id", result.WorktreeID,
	)
	return result, nil
}

// rollbackFailedArchive keeps a failed archive harmless: when it committed before a
// later step of the archive failed, the source is unarchived again. The archive state
// is read on a fresh lifetime, and a read that fails is reported, never taken as proof
// that nothing was archived.
func (m *Manager) rollbackFailedArchive(ctx context.Context, plan *profileMovePlan, cause error) error {
	readCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), profileMoveTimeout)
	defer cancel()
	err := m.requireSessionUnarchived(readCtx, plan.meta.WorkspaceID, plan.meta.ID)
	switch {
	case err == nil:
		return cause
	case errors.Is(err, ErrSessionArchived):
		return m.rollbackProfileMove(ctx, plan, profileMoveProgress{}, cause)
	default:
		return &ProfileMoveError{
			Reason: ProfileMoveRefusalRecoveryIncomplete,
			Message: fmt.Sprintf(
				"archiving session %s failed (%v) and its archive state could not be read (%v); "+
					"unarchive it if it stays archived",
				plan.meta.ID, cause, err,
			),
			cause: errors.Join(cause, err),
		}
	}
}

// profileMoveCheckoutGone reports whether a release that failed still removed the
// checkout, so the rollback checks the branch out again instead of rebinding a
// session to a removed worktree. An unreadable checkout counts as gone: recreating
// it either succeeds or reports what is left.
func (m *Manager) profileMoveCheckoutGone(ctx context.Context, plan *profileMovePlan) bool {
	checkout, err := plan.worktree.InspectProfileMoveWorktree(ctx, plan.meta.WorkspaceID, plan.checkout.ID)
	return err != nil || !checkout.Ready
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

// readProfileMoveCopy reads the committed copy back. The move is complete either way,
// so a failed read is logged and the copy is reported by what the move knows of it.
func (m *Manager) readProfileMoveCopy(ctx context.Context, plan *profileMovePlan, copyID, worktreeID string) *Info {
	copied, err := m.Status(ctx, copyID)
	if err == nil {
		return copied
	}
	m.logger.Warn(
		"session.profile_move.copy_read_failed", "session_id", plan.meta.ID, "copy_session_id", copyID, "error", err,
	)
	return &Info{
		ID: copyID, Name: plan.meta.Name, AgentName: plan.meta.AgentName, WorkspaceID: plan.meta.WorkspaceID,
		ProfileID: plan.opts.TargetProfileID, WorktreeID: worktreeID, State: StateStopped,
	}
}

// rollbackProfileMove undoes a failed move in reverse order: the target checkout is
// released, the branch is checked out again for the source profile, and the
// original is unarchived and rebound to it. It runs on its own detached, bounded
// lifetime, so a move that spent its deadline still recovers. The cause is returned
// as is when the rollback completes; otherwise a recovery_incomplete refusal names
// what is left.
func (m *Manager) rollbackProfileMove(
	ctx context.Context,
	plan *profileMovePlan,
	progress profileMoveProgress,
	cause error,
) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), profileMoveTimeout)
	defer cancel()
	meta := &plan.meta
	restoredID, errs := m.undoProfileMoveCheckouts(ctx, plan, progress)
	errs = append(errs, m.restoreProfileMoveSource(ctx, plan, restoredID)...)
	if len(errs) == 0 {
		return cause
	}
	rollbackErr := errors.Join(errs...)
	m.logger.Error(
		"session.profile_move.rollback_incomplete",
		"session_id", meta.ID, "cause", cause, "error", rollbackErr,
	)
	return &ProfileMoveError{
		Reason: ProfileMoveRefusalRecoveryIncomplete,
		Message: fmt.Sprintf(
			"moving session %s failed (%v) and its rollback did not finish (%v); the session is kept%s",
			meta.ID, cause, rollbackErr, profileMoveBranchNote(plan),
		),
		cause: errors.Join(cause, rollbackErr),
	}
}

// undoProfileMoveCheckouts releases the target checkout and checks the branch out again
// for the source profile, returning that checkout's id. A source checkout that is
// still ready (a release that failed before removing it) is kept as it is.
func (m *Manager) undoProfileMoveCheckouts(
	ctx context.Context,
	plan *profileMovePlan,
	progress profileMoveProgress,
) (string, []error) {
	meta := &plan.meta
	var errs []error
	if progress.targetWorktreeID != "" {
		err := plan.worktree.ReleaseProfileMoveWorktree(ctx, meta.WorkspaceID, progress.targetWorktreeID)
		if err != nil {
			errs = append(errs, fmt.Errorf("release the target worktree %s: %w", progress.targetWorktreeID, err))
		}
	}
	if !progress.released || len(errs) != 0 {
		return "", errs
	}
	checkout, err := plan.worktree.InspectProfileMoveWorktree(ctx, meta.WorkspaceID, plan.checkout.ID)
	if err == nil && checkout.Ready {
		return "", nil
	}
	id, _, err := plan.worktree.CreateProfileMoveWorktree(ctx, meta.WorkspaceID, plan.opts.ProfileID, *plan.checkout)
	if err != nil {
		return "", []error{fmt.Errorf("check out branch %s again: %w", plan.checkout.Branch, err)}
	}
	return id, nil
}

// restoreProfileMoveSource unarchives the source and rebinds it to restoredID under the
// session's conversation lock, so a resume cannot start it on a removed checkout in
// between. The move already holds the profile-move lock the public Unarchive takes.
func (m *Manager) restoreProfileMoveSource(ctx context.Context, plan *profileMovePlan, restoredID string) []error {
	meta := &plan.meta
	operationCtx, unlock, err := m.lockConversationOperation(ctx, meta.ID)
	if err != nil {
		return []error{fmt.Errorf("lock session %s: %w", meta.ID, err)}
	}
	if _, err := m.setArchived(operationCtx, meta.WorkspaceID, meta.ID, false); err != nil {
		unlock()
		return []error{fmt.Errorf("unarchive session %s: %w", meta.ID, err)}
	}
	if restoredID == "" {
		unlock()
		return nil
	}
	expected := plan.checkout.ID
	info, previous, changed, err := m.changeStoppedWorktreeBinding(operationCtx, meta.ID, restoredID, &expected)
	unlock()
	if err != nil {
		return []error{fmt.Errorf("bind session %s to worktree %s: %w", meta.ID, restoredID, err)}
	}
	if changed {
		m.notifyWorktreeBindingChanged(ctx, info, previous)
	}
	return nil
}

func profileMoveBranchNote(plan *profileMovePlan) string {
	if plan.checkout == nil {
		return ""
	}
	return fmt.Sprintf(" and branch %s keeps every commit", plan.checkout.Branch)
}
