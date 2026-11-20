package worktree

import (
	"context"
	"fmt"
)

func (s *Service) sideSessionRemovalRefusal(ctx context.Context, item Worktree, activeOnly bool, cause error) error {
	reader, ok := s.sessions.(interface {
		SideSessionWorktreeBinding(context.Context, string, string, bool) (string, string, error)
	})
	if !ok {
		return cause
	}
	side, source, err := reader.SideSessionWorktreeBinding(ctx, item.WorkspaceID, item.ID, activeOnly)
	if err != nil {
		return err
	}
	if side != "" {
		return fmt.Errorf("%w: side chat %s belongs to source session %s", cause, side, source)
	}
	return cause
}
