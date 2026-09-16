package worktree

import (
	"context"
	"errors"
	"fmt"
)

func (s *Service) Dismiss(ctx context.Context, workspaceID, id string) error {
	item, err := s.store.Get(ctx, workspaceID, id)
	if errors.Is(err, ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("worktree: read dismiss target: %w", err)
	}
	if item == nil {
		return ErrNotFound
	}
	if item.State == StateDismissed {
		return nil
	}
	release, acquired := s.usage.tryAcquireExclusive(worktreeUsageKey(workspaceID, item.ID))
	if !acquired {
		return ErrOperationInProgress
	}
	defer release()
	item, err = s.store.Get(ctx, workspaceID, item.ID)
	if err != nil {
		return fmt.Errorf("worktree: reread dismiss target: %w", err)
	}
	if item == nil {
		return ErrNotFound
	}
	switch item.State {
	case StateDismissed:
		return nil
	case StateMissing, StateRemoved, StateFailed:
	default:
		return ErrNotReady
	}
	if err := s.requireNoActiveSession(ctx, *item); err != nil {
		return err
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
