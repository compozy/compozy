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
	original, err := m.Archive(ctx, meta.WorkspaceID, meta.ID)
	if err != nil {
		return MoveToProfileResult{}, err
	}
	execCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), profileMoveTimeout)
	defer cancel()
	var progress profileMoveProgress
	relocation := &deriveRelocation{profileID: plan.opts.TargetProfileID}
	if plan.checkout != nil {
		err := plan.worktree.ReleaseProfileMoveWorktree(execCtx, meta.WorkspaceID, plan.checkout.ID)
		if err != nil {
			return MoveToProfileResult{}, m.rollbackProfileMove(execCtx, plan, progress, err)
		}
		progress.released = true
		id, root, err := plan.worktree.CreateProfileMoveWorktree(
			execCtx, meta.WorkspaceID, plan.opts.TargetProfileID, *plan.checkout,
		)
		if err != nil {
			return MoveToProfileResult{}, m.rollbackProfileMove(execCtx, plan, progress, err)
		}
		progress.targetWorktreeID = id
		relocation.worktreeID, relocation.cwd = id, root
	}
	key, err := m.createSessionID("")
	if err != nil {
		return MoveToProfileResult{}, m.rollbackProfileMove(execCtx, plan, progress, err)
	}
	derived, err := m.deriveSession(execCtx, deriveSpec{
		kind: store.LineageKindFork, workspaceID: meta.WorkspaceID, profileID: plan.opts.ProfileID,
		sourceID: meta.ID, key: "profile-move:" + key, relocation: relocation,
	})
	if err != nil && derived.ChildSessionID == "" {
		return MoveToProfileResult{}, m.rollbackProfileMove(execCtx, plan, progress, err)
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
		if result.Copy, err = m.Status(execCtx, derived.ChildSessionID); err != nil {
			return result, err
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

// rollbackProfileMove undoes a failed move in reverse order: the target checkout is
// released, the branch is checked out again for the source profile, and the
// original is unarchived and rebound to it. The cause is returned as is when the
// rollback completes; otherwise a recovery_incomplete refusal names what is left.
func (m *Manager) rollbackProfileMove(
	ctx context.Context,
	plan *profileMovePlan,
	progress profileMoveProgress,
	cause error,
) error {
	meta := &plan.meta
	var errs []error
	if progress.targetWorktreeID != "" {
		if err := plan.worktree.ReleaseProfileMoveWorktree(
			ctx,
			meta.WorkspaceID,
			progress.targetWorktreeID,
		); err != nil {
			errs = append(errs, fmt.Errorf("release the target worktree %s: %w", progress.targetWorktreeID, err))
		}
	}
	restoredID := ""
	if progress.released && len(errs) == 0 {
		id, _, err := plan.worktree.CreateProfileMoveWorktree(
			ctx,
			meta.WorkspaceID,
			plan.opts.ProfileID,
			*plan.checkout,
		)
		if err != nil {
			errs = append(errs, fmt.Errorf("check out branch %s again: %w", plan.checkout.Branch, err))
		}
		restoredID = id
	}
	if _, err := m.Unarchive(ctx, meta.WorkspaceID, meta.ID); err != nil {
		errs = append(errs, fmt.Errorf("unarchive session %s: %w", meta.ID, err))
	} else if restoredID != "" {
		expected := plan.checkout.ID
		if _, err := m.ChangeWorktreeBinding(ctx, meta.ID, restoredID, &expected); err != nil {
			errs = append(errs, fmt.Errorf("bind session %s to worktree %s: %w", meta.ID, restoredID, err))
		}
	}
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

func profileMoveBranchNote(plan *profileMovePlan) string {
	if plan.checkout == nil {
		return ""
	}
	return fmt.Sprintf(" and branch %s keeps every commit", plan.checkout.Branch)
}
