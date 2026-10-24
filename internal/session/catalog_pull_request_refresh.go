package session

import (
	"context"
	"fmt"
	"slices"

	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) refreshCatalogPullRequests(ctx context.Context, query store.SessionCatalogPageQuery) error {
	m.mu.RLock()
	reader := m.pullRequestReader
	m.mu.RUnlock()
	branches, ok := m.sessionCatalog.(store.SessionCatalogBranchReader)
	if !ok || reader == nil {
		return nil
	}
	keys, err := branches.SessionCatalogBranches(ctx, query)
	if err != nil {
		return fmt.Errorf("session: catalog refresh branches: %w", err)
	}
	eligible := make([]BranchKey, 0, len(keys))
	for _, key := range keys {
		if !slices.Contains(query.DefaultBranches[key.WorkspaceID], key.Branch) {
			eligible = append(eligible, BranchKey{WorkspaceID: key.WorkspaceID, Branch: key.Branch})
		}
	}
	if len(eligible) != 0 {
		reader.PullRequests(ctx, eligible)
	}
	return nil
}
