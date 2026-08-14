package globaldb

import (
	"context"
	"fmt"
	"strings"
)

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
