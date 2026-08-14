package daemon

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/worktree"
)

// executionWorktreeService is the shared task/loop materialization surface.
// Boot constructs its consumers before the concrete worktree service, so the
// adapter resolves the current service at call time.
type executionWorktreeService interface {
	Get(context.Context, string, string) (*worktree.Worktree, error)
	MaterializeForRun(context.Context, string, worktree.RunWorktreeRequest) (*worktree.Worktree, error)
	RollbackRunMaterialization(context.Context, string, string, string) error
}

type daemonExecutionWorktrees struct {
	lookup func() executionWorktreeService
}

func executionWorktreesForState(state *bootState) daemonExecutionWorktrees {
	return daemonExecutionWorktrees{lookup: func() executionWorktreeService {
		if state == nil {
			return nil
		}
		return state.worktrees
	}}
}

func (r daemonExecutionWorktrees) service() executionWorktreeService {
	if r.lookup == nil {
		return nil
	}
	return r.lookup()
}

func (r daemonExecutionWorktrees) Get(
	ctx context.Context,
	workspaceID string,
	ref string,
) (*worktree.Worktree, error) {
	service := r.service()
	if service == nil {
		return nil, fmt.Errorf("daemon: worktree service is unavailable: %w", worktree.ErrNotFound)
	}
	return service.Get(ctx, workspaceID, ref)
}

func (r daemonExecutionWorktrees) MaterializeForRun(
	ctx context.Context,
	workspaceID string,
	request worktree.RunWorktreeRequest,
) (*worktree.Worktree, error) {
	service := r.service()
	if service == nil {
		return nil, fmt.Errorf("daemon: worktree service is unavailable: %w", worktree.ErrPerRunMaterialization)
	}
	return service.MaterializeForRun(ctx, workspaceID, request)
}

func (r daemonExecutionWorktrees) RollbackRunMaterialization(
	ctx context.Context,
	workspaceID string,
	worktreeID string,
	runID string,
) error {
	service := r.service()
	if service == nil {
		return fmt.Errorf("daemon: worktree service is unavailable: %w", worktree.ErrPerRunMaterialization)
	}
	return service.RollbackRunMaterialization(ctx, workspaceID, worktreeID, runID)
}
