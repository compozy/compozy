package daemon

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/worktree"

	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

func terminalWorktreeRootResolver(state *bootState) terminalpkg.WorktreeRootResolver {
	return func(ctx context.Context, workspaceID, profileID, ref string) (string, error) {
		if state == nil {
			return "", terminalpkg.ErrServiceUnavailable
		}
		return resolveTerminalWorktreeRoot(
			ctx,
			daemonSessionWorktreeResolver{state: state},
			workspaceID,
			profileID,
			ref,
		)
	}
}

func resolveTerminalWorktreeRoot(
	ctx context.Context,
	worktrees terminalExecutionWorktrees,
	workspaceID, profileID, ref string,
) (string, error) {
	_, root, err := worktrees.ResolveSessionWorktreeForProfile(ctx, workspaceID, profileID, ref)
	if err == nil && strings.TrimSpace(root) == "" {
		err = worktree.ErrNotFound
	}
	if err != nil {
		return "", terminalWorktreeError(ref, err)
	}
	return root, nil
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
		return terminalpkg.ErrServiceUnavailable
	}
}
