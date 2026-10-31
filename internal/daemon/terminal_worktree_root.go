package daemon

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/worktree"

	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

type terminalWorktreeUsage interface {
	AcquireUsage(context.Context, string, string) (*worktree.Worktree, func(), error)
}

func terminalWorktreeRootResolver(state *bootState) terminalpkg.WorktreeRootResolver {
	return func(ctx context.Context, workspaceID, profileID, ref string) (string, func(), error) {
		if state == nil || state.worktrees == nil {
			return "", nil, terminalpkg.ErrServiceUnavailable
		}
		root, release, err := resolveTerminalWorktreeRoot(
			ctx, daemonSessionWorktreeResolver{state: state}, state.worktrees, workspaceID, profileID, ref,
		)
		if errors.Is(err, terminalpkg.ErrServiceUnavailable) {
			state.logger.Warn(
				"terminal: resolve worktree", "workspace_id", workspaceID, "worktree", ref, "error", err,
			)
			return "", nil, terminalpkg.ErrServiceUnavailable
		}
		return root, release, err
	}
}

// resolveTerminalWorktreeRoot holds a usage lease on the ready worktree so removal
// refuses while the terminal runs in its checkout.
func resolveTerminalWorktreeRoot(
	ctx context.Context,
	worktrees terminalExecutionWorktrees,
	usage terminalWorktreeUsage,
	workspaceID, profileID, ref string,
) (string, func(), error) {
	id, root, err := worktrees.ResolveSessionWorktreeForProfile(ctx, workspaceID, profileID, ref)
	if err == nil && strings.TrimSpace(root) == "" {
		err = worktree.ErrNotFound
	}
	if err != nil {
		return "", nil, terminalWorktreeError(ref, err)
	}
	item, release, err := usage.AcquireUsage(ctx, workspaceID, id)
	if err != nil {
		return "", nil, terminalWorktreeError(ref, err)
	}
	if item == nil || item.ID != id || item.ProfileID != profileID || item.Path != root {
		release()
		return "", nil, terminalWorktreeError(ref, worktree.ErrNotFound)
	}
	return root, release, nil
}

func terminalWorktreeError(ref string, err error) error {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case errors.Is(err, worktree.ErrNotFound):
		return &terminalpkg.Error{
			Code:    terminalpkg.ErrorCodeWorktreeNotFound,
			Message: "worktree " + ref + " was not found in this project for the acting profile",
			Err:     errors.Join(terminalpkg.ErrWorktreeNotFound, err),
		}
	case errors.Is(err, worktree.ErrNotReady):
		return &terminalpkg.Error{
			Code:    terminalpkg.ErrorCodeWorktreeNotReady,
			Message: "worktree " + ref + " is not ready; wait for it to finish setting up",
			Err:     errors.Join(terminalpkg.ErrWorktreeNotReady, err),
		}
	case errors.Is(err, worktree.ErrMissing):
		return &terminalpkg.Error{
			Code:    terminalpkg.ErrorCodeWorktreeMissing,
			Message: "worktree " + ref + " was removed or its folder no longer exists",
			Err:     errors.Join(terminalpkg.ErrWorktreeMissing, err),
		}
	default:
		return errors.Join(terminalpkg.ErrServiceUnavailable, err)
	}
}
