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
	if s.forge == nil {
		return
	}
	remoteURLs, err := s.readOriginRemoteURLs(ctx, item.Path)
	if err != nil {
		s.logger.WarnContext(
			ctx, "worktree remote refresh after exit failed", "worktree_id", item.ID,
			"error", diagnostics.RedactAndBound(err.Error(), 2048),
		)
		return
	}
	if len(remoteURLs) == 0 {
		return
	}
	status, err := s.forge.Status(ctx, ForgeStatusRequest{
		WorkspaceID: item.WorkspaceID, WorktreeID: item.ID,
		RemoteURLs: remoteURLs, Branch: item.Branch,
	})
	if err != nil || status == nil {
		message := "provider returned no status"
		if err != nil {
			message = err.Error()
		}
		s.logger.WarnContext(
			ctx, "worktree forge refresh after exit failed", "worktree_id", item.ID,
			"error", diagnostics.RedactAndBound(message, 2048),
		)
		return
	}
	if status.PRURL != "" {
		status.PRURL, err = sanitizeForgeWebURL(status.PRURL)
		if err != nil {
			s.logger.WarnContext(
				ctx, "worktree forge refresh returned invalid pull request URL", "worktree_id", item.ID,
			)
			return
		}
	}
	if err := s.store.SaveForgeStatus(ctx, item.WorkspaceID, item.ID, *status); err != nil {
		s.logger.WarnContext(
			ctx, "worktree forge refresh persistence after exit failed", "worktree_id", item.ID,
			"error", diagnostics.RedactAndBound(err.Error(), 2048),
		)
	}
}
