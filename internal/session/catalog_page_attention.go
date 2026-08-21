package session

import (
	"context"
	"fmt"
	"slices"
	"sort"

	"github.com/compozy/compozy/internal/store"
)

const sessionAttentionScanBatch = MaxListLimit

func (m *Manager) listAttentionPage(
	ctx context.Context,
	query ListQuery,
	pager store.SessionCatalogPager,
	fingerprint string,
	after *store.SessionCatalogPosition,
) (ListPage, error) {
	activeByID, activeIDs, activeMatches := m.activeSessionCatalogRows(query)
	durableMatches, err := m.scanAttentionCatalog(ctx, query, pager, activeIDs)
	if err != nil {
		return ListPage{}, err
	}
	candidates := make([]store.SessionInfo, 0, len(durableMatches)+len(activeMatches))
	candidates = append(candidates, durableMatches...)
	candidates = append(candidates, activeMatches...)
	sort.Slice(candidates, func(i, j int) bool {
		return compareSessionCatalogPosition(
			sessionCatalogPosition(candidates[i], query.Sort),
			sessionCatalogPosition(candidates[j], query.Sort),
		) < 0
	})
	page := ListPage{Total: len(candidates), Limit: query.Limit}
	if after != nil {
		candidates = sessionsAfterCatalogPosition(candidates, query.Sort, *after)
	}
	if len(candidates) > query.Limit {
		page.HasMore = true
		candidates = candidates[:query.Limit]
	}
	page.Sessions = projectSessionCatalogPage(candidates, activeByID)
	if page.HasMore && len(candidates) > 0 {
		page.NextCursor, err = encodeSessionListCursor(
			fingerprint,
			sessionCatalogPosition(candidates[len(candidates)-1], query.Sort),
		)
		if err != nil {
			return ListPage{}, err
		}
	}
	return page, nil
}

func (m *Manager) scanAttentionCatalog(
	ctx context.Context,
	query ListQuery,
	pager store.SessionCatalogPager,
	activeIDs []string,
) ([]store.SessionInfo, error) {
	matches := make([]store.SessionInfo, 0)
	var cursor *store.SessionCatalogPosition
	now := m.now().UTC()
	for {
		page, err := pager.PageSessions(ctx, attentionCatalogPageQuery(query, activeIDs, cursor))
		if err != nil {
			return nil, fmt.Errorf("session: scan durable attention catalog: %w", err)
		}
		for index := range page.Sessions {
			if sessionMatchesListQuery(sessionInfoFromCatalog(page.Sessions[index]), query, now) {
				matches = append(matches, page.Sessions[index])
			}
		}
		if len(page.Sessions) < sessionAttentionScanBatch {
			return matches, nil
		}
		last := page.Sessions[len(page.Sessions)-1]
		position := sessionCatalogPosition(last, query.Sort)
		cursor = &position
	}
}

func attentionCatalogPageQuery(
	query ListQuery,
	activeIDs []string,
	after *store.SessionCatalogPosition,
) store.SessionCatalogPageQuery {
	return store.SessionCatalogPageQuery{
		ReadScope:           query.ReadScope,
		WorkspaceID:         query.WorkspaceID,
		WorktreeID:          query.WorktreeID,
		State:               query.State,
		SessionType:         string(query.SessionType),
		AgentName:           query.AgentName,
		ParentSessionID:     query.ParentSessionID,
		RootSessionID:       query.RootSessionID,
		Search:              query.Search,
		Resumable:           query.Resumable,
		Archive:             query.Archive,
		Sort:                query.Sort,
		Limit:               sessionAttentionScanBatch,
		After:               after,
		ExcludeIDs:          activeIDs,
		ExcludeSessionTypes: []string{string(SessionTypeDream)},
		ExcludeSpawnRoles:   []string{SpawnRoleMemoryExtractor, SpawnRoleAutoTitle},
	}
}

func sessionsAfterCatalogPosition(
	values []store.SessionInfo,
	sortKey string,
	after store.SessionCatalogPosition,
) []store.SessionInfo {
	for index := range values {
		if compareSessionCatalogPosition(sessionCatalogPosition(values[index], sortKey), after) > 0 {
			return values[index:]
		}
	}
	return nil
}

func badgeStrings(values []Badge) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, string(value))
	}
	return result
}

func badgeFilterContains(values []Badge, target Badge) bool {
	return slices.Contains(values, target)
}
