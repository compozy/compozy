package worktree

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// RollbackRunMaterialization removes the exact per-run worktree produced for a
// session that could not be bound under its active task-run lease.
func (s *Service) RollbackRunMaterialization(
	ctx context.Context,
	workspaceID string,
	worktreeID string,
	runID string,
) error {
	workspaceID = strings.TrimSpace(workspaceID)
	worktreeID = strings.TrimSpace(worktreeID)
	runID = strings.TrimSpace(runID)
	item, err := s.store.Get(ctx, workspaceID, worktreeID)
	if err != nil {
		return fmt.Errorf("worktree: read run materialization rollback target: %w", err)
	}
	if item.Origin != OriginPerRun || strings.TrimSpace(item.RunID) != runID ||
		(item.State != StatePending && item.State != StateReady) {
		return ErrNotFound
	}
	workspace, err := s.resolveWorkspace(ctx, workspaceID)
	if err != nil {
		return err
	}
	commonDir, err := s.commonDir(ctx, workspace.Root)
	if err != nil {
		return err
	}
	release, err := s.locks.Acquire(ctx, commonDir)
	if err != nil {
		return err
	}
	defer release()
	current, err := s.store.Get(ctx, workspaceID, worktreeID)
	if err != nil {
		return fmt.Errorf("worktree: reread run materialization rollback target: %w", err)
	}
	if current.Origin != OriginPerRun || strings.TrimSpace(current.RunID) != runID ||
		(current.State != StatePending && current.State != StateReady) {
		return ErrNotFound
	}
	rollbackErr := s.rollbackCreate(context.WithoutCancel(ctx), workspace, commonDir, *current)
	if rollbackErr != nil {
		return rollbackErr
	}
	deleteErr := s.store.DeleteRunMaterialization(
		context.WithoutCancel(ctx),
		workspaceID,
		worktreeID,
		runID,
	)
	if errors.Is(deleteErr, ErrNotFound) {
		return ErrNotFound
	}
	if deleteErr != nil {
		return fmt.Errorf("worktree: delete run materialization rollback target: %w", deleteErr)
	}
	s.invalidateDiscovery(workspaceID)
	s.publishCatalogEvent(CatalogEventDeleted, *current)
	return nil
}
