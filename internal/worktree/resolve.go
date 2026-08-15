package worktree

import (
	"context"
	"errors"
	"fmt"
)

// Resolve returns the canonical registry record for an ID-or-name reference
// without probing repository capability or derived status caches.
func (s *Service) Resolve(ctx context.Context, workspaceID, ref string) (*Worktree, error) {
	item, err := s.store.Get(ctx, workspaceID, ref)
	if errors.Is(err, ErrNotFound) || item == nil {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("worktree: resolve registry row: %w", err)
	}
	return item, nil
}
