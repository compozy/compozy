package worktree

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

// ListAggregate reads authorized stored state without Git discovery or status refresh.
func (s *Service) ListAggregate(
	ctx context.Context,
	scope store.ReadScope,
	allowedWorkspaces map[string]bool,
) (*DetailedListing, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	result := &DetailedListing{Worktrees: []Inspection{}}
	if len(allowedWorkspaces) == 0 {
		return result, nil
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
	result.Worktrees = make([]Inspection, 0, len(rows))
	for _, row := range rows {
		if !scope.Matches(row.ProfileID) || !allowedWorkspaces[row.WorkspaceID] {
			continue
		}
		item, err := s.inspectItem(ctx, row)
		if err != nil {
			return nil, err
		}
		result.Worktrees = append(result.Worktrees, item)
	}
	s.hydrateDisplayPullRequests(ctx, result.Worktrees)
	return result, nil
}
