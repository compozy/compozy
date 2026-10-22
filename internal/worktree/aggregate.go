package worktree

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

// ListAggregate reads stored worktree state without invoking Git discovery or refresh.
func (s *Service) ListAggregate(ctx context.Context, scope store.ReadScope) (*DetailedListing, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	reader, ok := s.store.(interface {
		ListAggregate(context.Context) ([]Worktree, error)
	})
	if !ok {
		return nil, fmt.Errorf("worktree aggregate store unavailable")
	}
	rows, err := reader.ListAggregate(ctx)
	if err != nil {
		return nil, err
	}
	result := &DetailedListing{Worktrees: make([]Inspection, 0, len(rows))}
	for _, row := range rows {
		if !scope.Matches(row.ProfileID) {
			continue
		}
		item, err := s.inspectItem(ctx, row)
		if err != nil {
			return nil, err
		}
		result.Worktrees = append(result.Worktrees, item)
	}
	return result, nil
}
