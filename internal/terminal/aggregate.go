package terminal

import (
	"cmp"
	"context"
	"slices"

	"github.com/compozy/compozy/internal/store"
)

// ListAggregate snapshots live terminals across workspaces within the requested profile scope.
func (m *Service) ListAggregate(ctx context.Context, scope store.ReadScope) ([]Info, error) {
	if err := requestContextError(ctx, "list aggregate"); err != nil {
		return nil, err
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	m.mu.RLock()
	sessions := make([]*session, 0, len(m.terminals))
	for key, item := range m.terminals {
		if scope.Matches(key.profileID) {
			sessions = append(sessions, item)
		}
	}
	m.mu.RUnlock()
	items := make([]Info, 0, len(sessions))
	for _, item := range sessions {
		items = append(items, item.Info())
	}
	slices.SortFunc(items, func(a, b Info) int { return cmp.Or(a.CreatedAt.Compare(b.CreatedAt), cmp.Compare(a.ID, b.ID)) })
	return items, nil
}

// InputRequestsAggregate reads pending input across workspaces without per-project fan-out.
func (m *Service) InputRequestsAggregate(ctx context.Context, scope store.ReadScope) ([]PendingInputRequest, error) {
	if err := requestContextError(ctx, "list aggregate input"); err != nil {
		return nil, err
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	return m.inputs.listSelection(scope, "", func(string) bool { return true }), nil
}
