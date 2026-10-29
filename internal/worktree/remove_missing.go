package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
)

func (s *Service) removeMissing(ctx context.Context, item Worktree) error {
	if _, err := os.Lstat(item.Path); !errors.Is(err, os.ErrNotExist) {
		if err != nil {
			return fmt.Errorf("worktree: inspect missing removal path: %w", err)
		}
		return ErrNotReady
	}
	if err := s.requireNoActiveSession(ctx, item); err != nil {
		return err
	}
	removed, err := s.store.RemoveMissing(ctx, item.WorkspaceID, item.ID, s.now().UTC())
	if err != nil {
		return fmt.Errorf("worktree: remove missing registration: %w", err)
	}
	if !removed {
		return ErrNotReady
	}
	item.State = StateDismissed
	s.invalidateDiscovery(item.WorkspaceID)
	s.emit(ctx, EventDismissed, item)
	return nil
}
