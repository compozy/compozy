package core

import (
	"context"
	"fmt"
	"time"
)

const sessionWorktreeRollbackTimeout = 30 * time.Second

type materializedSessionWorktree struct {
	ID          string
	WorkspaceID string
	Created     bool
}

func (h *BaseHandlers) rollbackMaterializedSessionWorktree(
	ctx context.Context,
	target materializedSessionWorktree,
) error {
	if !target.Created || h.Worktrees == nil {
		return nil
	}
	rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sessionWorktreeRollbackTimeout)
	defer cancel()
	if _, err := h.Worktrees.Remove(rollbackCtx, target.WorkspaceID, target.ID, true); err != nil {
		return fmt.Errorf("api: roll back materialized session worktree: %w", err)
	}
	return nil
}
