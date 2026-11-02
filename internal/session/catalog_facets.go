package session

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/compozy/compozy/internal/store"
)

// CatalogFacets combines exact durable metadata with authoritative active badges.
// Search and selected badge do not change population chips; row queries own them.
func (m *Manager) CatalogFacets(ctx context.Context, query ListQuery) (store.SessionCatalogFacetResult, error) {
	if ctx == nil {
		return store.SessionCatalogFacetResult{}, fmt.Errorf("%w: context is required", ErrListQueryInvalid)
	}
	normalized, err := normalizeCatalogFacetQuery(query)
	if err != nil {
		return store.SessionCatalogFacetResult{}, err
	}
	reader, ok := m.sessionCatalog.(store.SessionCatalogFacetReader)
	if !ok {
		return store.SessionCatalogFacetResult{}, errors.New("session: metadata catalog facets are required")
	}
	activeByID, activeIDs, activeMatches, err := m.activeBoardSessionCatalogRows(ctx, normalized)
	if err != nil {
		return store.SessionCatalogFacetResult{}, err
	}
	durableQuery := sessionCatalogPageQuery(normalized, nil, activeIDs)
	durableQuery.BoardExcludedIDs = query.BoardExcludedIDs
	durableQuery.RepositoryKeys = m.catalogRepositoryKeys(ctx, normalized)
	durableQuery.DefaultBranches = m.catalogDefaultBranches(ctx, normalized)
	if err := m.refreshCatalogPullRequests(ctx, durableQuery); err != nil {
		return store.SessionCatalogFacetResult{}, err
	}
	durable, err := reader.SessionCatalogFacets(ctx, durableQuery)
	if err != nil {
		return store.SessionCatalogFacetResult{}, fmt.Errorf("session: durable catalog facets: %w", err)
	}
	workspaces := make(map[string]store.SessionCatalogFacets, len(durable.ByWorkspace))
	for _, workspace := range durable.ByWorkspace {
		workspaces[workspace.WorkspaceID] = workspace.Facets
	}
	profiles := make(map[string]store.SessionCatalogFacets, len(durable.ByProfile))
	for _, profile := range durable.ByProfile {
		profiles[profile.ProfileID] = profile.Facets
	}
	for index := range activeMatches {
		info := sessionInfoFromCatalog(&activeMatches[index])
		// Use the live Info, including supervision, rather than the durable projection.
		if active := activeByID[info.ID]; active != nil {
			info = active
		}
		facets := workspaces[info.WorkspaceID]
		addSessionFacet(&facets, BadgeForInfo(info))
		if !slices.Contains(query.BoardExcludedIDs, info.ID) {
			facets.Board.Add(string(info.BoardColumn), 1)
		}
		addSessionFacet(&durable.Facets, BadgeForInfo(info))
		if !slices.Contains(query.BoardExcludedIDs, info.ID) {
			durable.Facets.Board.Add(string(info.BoardColumn), 1)
		}
		workspaces[info.WorkspaceID] = facets
		profile := profiles[info.ProfileID]
		addSessionFacet(&profile, BadgeForInfo(info))
		if !slices.Contains(query.BoardExcludedIDs, info.ID) {
			profile.Board.Add(string(info.BoardColumn), 1)
		}
		profiles[info.ProfileID] = profile
	}
	durable.ByWorkspace = make([]store.WorkspaceSessionCatalogFacets, 0, len(workspaces))
	for workspaceID, facets := range workspaces {
		durable.ByWorkspace = append(
			durable.ByWorkspace,
			store.WorkspaceSessionCatalogFacets{WorkspaceID: workspaceID, Facets: facets},
		)
	}
	slices.SortFunc(
		durable.ByWorkspace,
		func(a, b store.WorkspaceSessionCatalogFacets) int { return cmp.Compare(a.WorkspaceID, b.WorkspaceID) },
	)
	durable.ByProfile = make([]store.ProfileSessionCatalogFacets, 0, len(profiles))
	for id, facets := range profiles {
		durable.ByProfile = append(durable.ByProfile, store.ProfileSessionCatalogFacets{ProfileID: id, Facets: facets})
	}
	slices.SortFunc(
		durable.ByProfile,
		func(a, b store.ProfileSessionCatalogFacets) int { return cmp.Compare(a.ProfileID, b.ProfileID) },
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

func normalizeCatalogFacetQuery(query ListQuery) (ListQuery, error) {
	query.Search = ""
	query.Badges = nil
	query.Columns = nil
	query.AttentionOnly = false
	query.Cursor = ""
	query.Limit = DefaultListLimit
	return normalizeListQuery(query)
}
