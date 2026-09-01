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

type loopActionToolWorkspaceRootResolver struct {
	worktrees loopActionWorktreeReader
}

var _ looppkg.ActionToolWorkspaceRootResolver = (*loopActionToolWorkspaceRootResolver)(nil)

func (r *loopActionToolWorkspaceRootResolver) ResolveActionToolWorkspaceRoot(
	ctx context.Context,
	req looppkg.ActionToolWorkspaceRootRequest,
) (string, error) {
	if req.Environment.Mode != dsl.EnvironmentWorktree {
		return "", nil
	}
	item, err := resolveReadyLoopActionWorktree(
		ctx,
		r.worktrees,
		strings.TrimSpace(string(req.WorkspaceID)),
		req.Environment.WorktreeRef,
	)
	if err != nil {
		return "", err
	}
	return item.Path, nil
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
