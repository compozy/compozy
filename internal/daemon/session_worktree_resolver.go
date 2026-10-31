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
	return r.resolveSessionWorktree(ctx, workspaceID, ref, "")
}

func (r daemonSessionWorktreeResolver) ResolveSessionWorktreeForProfile(
	ctx context.Context, workspaceID, profileID, ref string,
) (string, string, error) {
	if strings.TrimSpace(profileID) == "" {
		return "", "", worktree.ErrNotFound
	}
	return r.resolveSessionWorktree(ctx, workspaceID, ref, profileID)
}

func (r daemonSessionWorktreeResolver) resolveSessionWorktree(
	ctx context.Context, workspaceID, ref, profileID string,
) (string, string, error) {
	service := r.service()
	if service == nil {
		return "", "", errors.New("daemon: worktree service is unavailable")
	}
	item, err := service.Get(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(ref))
	if err != nil {
		return "", "", err
	}
	if item == nil || (profileID != "" && item.ProfileID != profileID) {
		return "", "", worktree.ErrNotFound
	}
	switch item.State {
	case worktree.StateReady:
		info, statErr := os.Stat(item.Path)
		if statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				return "", "", worktree.ErrMissing
			}
			return "", "", fmt.Errorf("daemon: inspect worktree root: %w", statErr)
		}
		if !info.IsDir() {
			return "", "", worktree.ErrMissing
		}
		return item.ID, item.Path, nil
	case worktree.StateMissing, worktree.StateRemoved, worktree.StateDismissed:
		return "", "", worktree.ErrMissing
	case worktree.StateFailed:
		return "", "", failedWorktreeRefusal(*item)
	case worktree.StatePending, worktree.StateRemoving:
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

func (r daemonSessionWorktreeResolver) AcquireSessionWorktreeBinding(
	ctx context.Context,
	workspaceID, profileID, ref string,
) (string, string, func(), error) {
	service, ok := r.service().(interface {
		AcquireBindingUsage(context.Context, string, string) (*worktree.Worktree, func(), error)
	})
	if !ok {
		return "", "", nil, errors.New("daemon: worktree usage service is unavailable")
	}
	id, root, err := r.ResolveSessionWorktreeForProfile(ctx, workspaceID, profileID, ref)
	if err != nil {
		return "", "", nil, err
	}
	item, release, err := service.AcquireBindingUsage(ctx, workspaceID, id)
	if err != nil {
		return "", "", nil, err
	}
	if item.ProfileID != profileID || item.Path != root {
		release()
		return "", "", nil, worktree.ErrNotFound
	}
	if item.State != worktree.StateReady {
		release()
		return "", "", nil, worktree.ErrNotReady
	}
	return id, root, release, nil
}

func (r daemonSessionWorktreeResolver) AcquireSessionWorktreeUnbinding(
	ctx context.Context,
	workspaceID, profileID, id string,
) (func(), error) {
	service, ok := r.service().(interface {
		AcquireBindingUsage(context.Context, string, string) (*worktree.Worktree, func(), error)
	})
	if !ok {
		return nil, errors.New("daemon: worktree usage service is unavailable")
	}
	item, release, err := service.AcquireBindingUsage(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if item.ProfileID != profileID {
		release()
		return nil, worktree.ErrNotFound
	}
	return release, nil
}

// failedWorktreeRefusal keeps the worktree_not_ready code for a failed
// creation and names its recorded cause, so callers can dismiss and retry.
func failedWorktreeRefusal(item worktree.Worktree) error {
	cause := strings.TrimSpace(item.CreateError)
	if cause == "" {
		cause = "cause unavailable"
	}
	return fmt.Errorf("%w: worktree %s creation failed (%s); dismiss it and create a new worktree",
		worktree.ErrNotReady, item.ID, cause)
}
