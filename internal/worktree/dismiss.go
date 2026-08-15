package worktree

import (
	"context"
	"errors"
	"fmt"
)

func (s *Service) Dismiss(ctx context.Context, workspaceID, id string) error {
	item, err := s.store.Get(ctx, workspaceID, id)
	if errors.Is(err, ErrNotFound) || item == nil {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("worktree: read dismiss target: %w", err)
	}
	switch item.State {
	case StateMissing, StateRemoved, StateFailed:
	default:
		return ErrNotReady
	}
	swapped, err := s.store.CompareAndSwapState(
		ctx, workspaceID, item.ID, item.State, StateDismissed, s.now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("worktree: dismiss tombstone: %w", err)
	}
	if !swapped {
		return ErrNotReady
	}
	item.State = StateDismissed
	s.invalidateDiscovery(workspaceID)
	s.emit(ctx, EventDismissed, *item)
	return nil
}
