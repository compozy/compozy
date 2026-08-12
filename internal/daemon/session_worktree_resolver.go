package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/worktree"
)

type daemonSessionWorktreeResolver struct {
	state  *bootState
	lookup func() sessionWorktreeLookup
}

type sessionWorktreeLookup interface {
	Get(context.Context, string, string) (*worktree.Worktree, error)
}

func (r daemonSessionWorktreeResolver) service() sessionWorktreeLookup {
	if r.lookup != nil {
		return r.lookup()
	}
	if r.state == nil {
		return nil
	}
	return r.state.worktrees
}

func (r daemonSessionWorktreeResolver) ResolveSessionWorktree(
	ctx context.Context,
	workspaceID string,
	ref string,
) (string, string, error) {
	service := r.service()
	if service == nil {
		return "", "", errors.New("daemon: worktree service is unavailable")
	}
	item, err := service.Get(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(ref))
	if err != nil {
		return "", "", err
	}
	switch item.State {
	case worktree.StateReady:
		if _, statErr := os.Stat(item.Path); statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				return "", "", worktree.ErrMissing
			}
			return "", "", fmt.Errorf("daemon: inspect worktree root: %w", statErr)
		}
		return item.ID, item.Path, nil
	case worktree.StateMissing, worktree.StateRemoved, worktree.StateDismissed:
		return "", "", worktree.ErrMissing
	case worktree.StatePending, worktree.StateFailed, worktree.StateRemoving:
		return "", "", worktree.ErrNotReady
	default:
		return "", "", worktree.ErrNotReady
	}
}

func (r daemonSessionWorktreeResolver) ValidateTaskWorktreeRef(
	ctx context.Context,
	workspaceID string,
	ref string,
) error {
	if _, _, err := r.ResolveSessionWorktree(ctx, workspaceID, ref); err != nil {
		return fmt.Errorf("%w: %v", worktree.ErrRefInvalid, err)
	}
	return nil
}
