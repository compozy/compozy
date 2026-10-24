package worktree

import (
	"context"

	"github.com/compozy/compozy/internal/diagnostics"
)

// refreshAfterExit refreshes local truth and makes one best-effort forge
// refresh. A remote outage cannot undo Git work that already completed.
func (s *Service) refreshAfterExit(ctx context.Context, item Worktree) {
	if _, err := s.refreshStatus(ctx, item); err != nil {
		s.logger.WarnContext(
			ctx, "worktree status refresh after exit failed", "worktree_id", item.ID,
			"error", diagnostics.RedactAndBound(err.Error(), 2048),
		)
	}
	s.requestPullRequestRefresh(ctx, item)
}
