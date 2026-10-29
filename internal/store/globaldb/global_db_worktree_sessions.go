package globaldb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func (g *WorktreeRepo) RemoveMissing(ctx context.Context, workspaceID, id string, at time.Time) (bool, error) {
	if err := g.checkReady(ctx, "remove missing worktree"); err != nil {
		return false, err
	}
	count, err := g.queries.RemoveMissingWorktree(ctx, sqlcgen.RemoveMissingWorktreeParams{
		WorkspaceID: strings.TrimSpace(workspaceID), WorktreeID: strings.TrimSpace(id),
		UpdatedAt: store.FormatTimestamp(at),
	})
	if err != nil {
		return false, fmt.Errorf("store: remove missing worktree: %w", err)
	}
	return count == 1, nil
}

// HasActiveWorktreeSession reports whether a live session still owns a
// workspace-scoped worktree binding.
func (g *GlobalDB) HasActiveWorktreeSession(
	ctx context.Context,
	workspaceID string,
	worktreeID string,
) (bool, error) {
	if g == nil || g.Worktrees == nil {
		return false, fmt.Errorf("store: worktree repository is required")
	}
	if err := g.Worktrees.checkReady(ctx, "inspect active worktree sessions"); err != nil {
		return false, err
	}
	var found bool
	err := g.Worktrees.db.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1 FROM sessions
  WHERE workspace_id = ? AND worktree_id = ?
    AND state IN ('starting', 'active', 'stopping')
)`, strings.TrimSpace(workspaceID), strings.TrimSpace(worktreeID)).Scan(&found)
	if err != nil {
		return false, fmt.Errorf("store: inspect active worktree sessions: %w", err)
	}
	return found, nil
}

// HasOtherActiveWorktreeSession excludes only the exact managed-delivery caller.
func (g *GlobalDB) HasOtherActiveWorktreeSession(
	ctx context.Context,
	workspaceID, worktreeID, sessionID string,
) (bool, error) {
	if g == nil || g.Worktrees == nil {
		return false, fmt.Errorf("store: worktree repository is required")
	}
	if err := g.Worktrees.checkReady(ctx, "inspect other worktree sessions"); err != nil {
		return false, err
	}
	var found bool
	err := g.Worktrees.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM sessions WHERE workspace_id=? AND worktree_id=? AND id<>? AND state IN ('starting','active','stopping'))`, workspaceID, worktreeID, sessionID).
		Scan(&found)
	if err != nil {
		return false, fmt.Errorf("store: inspect other worktree sessions: %w", err)
	}
	return found, nil
}
