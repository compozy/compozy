package daemon

import (
	"context"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/worktree"
)

type loopActionWorktreeReader interface {
	Get(context.Context, string, string) (*worktree.Worktree, error)
}

type loopActionWorktreeUsage interface {
	AcquireUsage(context.Context, string, string) (*worktree.Worktree, func(), error)
}

type loopActionToolWorkspaceRootResolver struct {
	worktrees loopActionWorktreeUsage
}

var _ looppkg.ActionToolWorkspaceRootResolver = (*loopActionToolWorkspaceRootResolver)(nil)

func (r *loopActionToolWorkspaceRootResolver) AcquireActionToolWorkspaceRoot(
	ctx context.Context,
	req looppkg.ActionToolWorkspaceRootRequest,
) (string, func(), error) {
	if req.Environment.Mode != dsl.EnvironmentWorktree {
		return "", nil, nil
	}
	if r.worktrees == nil {
		return "", nil, fmt.Errorf("%w: loop worktree service is unavailable", worktree.ErrNotFound)
	}
	item, release, err := r.worktrees.AcquireUsage(
		ctx,
		strings.TrimSpace(string(req.WorkspaceID)),
		req.Environment.WorktreeRef,
	)
	if err != nil {
		return "", nil, err
	}
	if release == nil {
		return "", nil, fmt.Errorf("%w: loop worktree usage lease has no release function", worktree.ErrNotReady)
	}
	if item == nil {
		release()
		return "", nil, fmt.Errorf("%w: loop worktree %q", worktree.ErrNotFound, req.Environment.WorktreeRef)
	}
	if item.State != worktree.StateReady || strings.TrimSpace(item.Path) == "" {
		release()
		return "", nil, fmt.Errorf(
			"%w: loop worktree %q is %q",
			worktree.ErrNotReady,
			req.Environment.WorktreeRef,
			item.State,
		)
	}
	return item.Path, release, nil
}

func resolveReadyLoopActionWorktree(
	ctx context.Context,
	worktrees loopActionWorktreeReader,
	workspaceID string,
	worktreeRef string,
) (*worktree.Worktree, error) {
	ref := strings.TrimSpace(worktreeRef)
	if worktrees == nil {
		return nil, fmt.Errorf("%w: loop worktree service is unavailable", worktree.ErrNotFound)
	}
	item, err := worktrees.Get(ctx, strings.TrimSpace(workspaceID), ref)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fmt.Errorf("%w: loop worktree %q", worktree.ErrNotFound, ref)
	}
	if item.State != worktree.StateReady || strings.TrimSpace(item.Path) == "" {
		return nil, fmt.Errorf("%w: loop worktree %q is %q", worktree.ErrNotReady, ref, item.State)
	}
	return item, nil
}
