package session

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"
	"time"

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
	// Page position and board columns must not hide historical refresh candidates.
	query.After, query.Columns = nil, nil
	query.Limit, query.Sort, query.SkipTotal = 1, "", false
	query.ExcludeIDs = slices.Sorted(slices.Values(query.ExcludeIDs))
	query.WorkspaceIDs = slices.Sorted(slices.Values(query.WorkspaceIDs))
	encoded, err := json.Marshal(query)
	if err != nil {
		return fmt.Errorf("session: catalog refresh scope: %w", err)
	}
	key := sha256.Sum256(encoded)
	m.mu.Lock()
	now := m.now()
	if until, ok := m.catalogRefreshes[key]; ok && now.Before(until) {
		m.mu.Unlock()
		return nil
	}
	for scope, until := range m.catalogRefreshes {
		if !now.Before(until) {
			delete(m.catalogRefreshes, scope)
		}
	}
	if m.catalogRefreshes == nil {
		m.catalogRefreshes = make(map[[32]byte]time.Time)
	}
	// Bound retained scopes as well as repeated discovery under concurrent reads.
	if len(m.catalogRefreshes) >= 128 {
		m.mu.Unlock()
		return nil
	}
	m.catalogRefreshes[key] = now.Add(time.Minute)
	m.mu.Unlock()
	keys, err := branches.SessionCatalogBranches(ctx, query)
	if err != nil {
		m.mu.Lock()
		delete(m.catalogRefreshes, key)
		m.mu.Unlock()
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
