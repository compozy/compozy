package session

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/compozy/compozy/internal/store"
)

// CatalogFacets combines exact durable metadata with authoritative active badges.
// Search and selected badge do not change population chips; row queries own them.
func (m *Manager) CatalogFacets(ctx context.Context, query ListQuery) (store.SessionCatalogFacetResult, error) {
	if ctx == nil {
		return store.SessionCatalogFacetResult{}, fmt.Errorf("%w: context is required", ErrListQueryInvalid)
	}
	query.Search = ""
	query.Badges = nil
	query.AttentionOnly = false
	query.Cursor = ""
	query.Limit = DefaultListLimit
	normalized, err := normalizeListQuery(query)
	if err != nil {
		return store.SessionCatalogFacetResult{}, err
	}
	reader, ok := m.sessionCatalog.(store.SessionCatalogFacetReader)
	if !ok {
		return store.SessionCatalogFacetResult{}, errors.New("session: metadata catalog facets are required")
	}
	activeByID, activeIDs, activeMatches := m.activeSessionCatalogRows(normalized)
	durable, err := reader.SessionCatalogFacets(ctx, sessionCatalogPageQuery(normalized, nil, activeIDs))
	if err != nil {
		return store.SessionCatalogFacetResult{}, fmt.Errorf("session: durable catalog facets: %w", err)
	}
	workspaces := make(map[string]store.SessionCatalogFacets, len(durable.ByWorkspace))
	for _, workspace := range durable.ByWorkspace {
		workspaces[workspace.WorkspaceID] = workspace.Facets
	}
	for index := range activeMatches {
		info := sessionInfoFromCatalog(&activeMatches[index])
		// Use the live Info, including supervision, rather than the durable projection.
		if active := activeByID[info.ID]; active != nil {
			info = active
		}
		facets := workspaces[info.WorkspaceID]
		addSessionFacet(&facets, BadgeForInfo(info))
		addSessionFacet(&durable.Facets, BadgeForInfo(info))
		workspaces[info.WorkspaceID] = facets
	}
	durable.ByWorkspace = make([]store.WorkspaceSessionCatalogFacets, 0, len(workspaces))
	for workspaceID, facets := range workspaces {
		durable.ByWorkspace = append(
			durable.ByWorkspace,
			store.WorkspaceSessionCatalogFacets{WorkspaceID: workspaceID, Facets: facets},
		)
	}
	sort.Slice(
		durable.ByWorkspace,
		func(i, j int) bool { return durable.ByWorkspace[i].WorkspaceID < durable.ByWorkspace[j].WorkspaceID },
	)
	return durable, nil
}

func addSessionFacet(facets *store.SessionCatalogFacets, badge Badge) {
	facets.All++
	switch ClassForBadge(badge) {
	case AttentionNeedsYou:
		facets.NeedsYou++
	case AttentionFinished:
		facets.Finished++
	}
	if badge == BadgeRunning {
		facets.Working++
	}
	if badge == BadgeIdle {
		facets.Idle++
	}
}
