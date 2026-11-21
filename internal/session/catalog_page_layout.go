package session

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/compozy/compozy/internal/store"
)

// sessionCatalogPlacements reads folder placements for every page candidate in
// one bounded lookup so manual ordering and folder decoration share one view.
func (m *Manager) sessionCatalogPlacements(
	ctx context.Context,
	durable []store.SessionInfo,
	active []store.SessionInfo,
) (map[string]store.SessionPlacement, error) {
	layout, ok := m.sessionCatalog.(store.SessionLayoutStore)
	if !ok || layout == nil {
		return map[string]store.SessionPlacement{}, nil
	}
	ids := make([]string, 0, len(durable)+len(active))
	for index := range durable {
		ids = append(ids, durable[index].ID)
	}
	for index := range active {
		ids = append(ids, active[index].ID)
	}
	placements := make(map[string]store.SessionPlacement, len(ids))
	for start := 0; start < len(ids); start += store.MaxSessionPlacementLookup {
		end := min(start+store.MaxSessionPlacementLookup, len(ids))
		chunk, err := layout.SessionPlacements(ctx, ids[start:end])
		if err != nil {
			return nil, fmt.Errorf("session: read catalog placements: %w", err)
		}
		maps.Copy(placements, chunk)
	}
	return placements, nil
}

func sessionCatalogPlacedPosition(
	info *store.SessionInfo,
	sortKey string,
	placements map[string]store.SessionPlacement,
) store.SessionCatalogPosition {
	position := sessionCatalogPosition(info, sortKey)
	if sortKey != ListSortManual {
		return position
	}
	rank := store.SessionManualRootRank
	manual := store.SessionManualUnplacedPosition
	if placement, ok := placements[info.ID]; ok {
		manual = placement.Position
		if placement.FolderID != "" {
			rank = placement.FolderPosition
		}
	}
	position.ManualFolderRank = &rank
	position.ManualPosition = &manual
	position.PrimaryAt = info.CreatedAt.UTC()
	return position
}

func compareSessionCatalogManualPosition(left store.SessionCatalogPosition, right store.SessionCatalogPosition) int {
	if *left.ManualFolderRank != *right.ManualFolderRank {
		return *left.ManualFolderRank - *right.ManualFolderRank
	}
	if *left.ManualPosition != *right.ManualPosition {
		return *left.ManualPosition - *right.ManualPosition
	}
	if !left.CreatedAt.Equal(right.CreatedAt) {
		if left.CreatedAt.After(right.CreatedAt) {
			return -1
		}
		return 1
	}
	return compareStringsDescending(left.ID, right.ID)
}

func pageSessionPlacements(
	page []store.SessionInfo,
	placements map[string]store.SessionPlacement,
) map[string]store.SessionPlacement {
	selected := make(map[string]store.SessionPlacement)
	for index := range page {
		if placement, ok := placements[page[index].ID]; ok {
			selected[placement.SessionID] = placement
		}
	}
	return selected
}

// readCatalogPageRows excludes live identities from the durable count, then
// decorates the active rows that the requested partition can return or summarize.
func (m *Manager) readCatalogPageRows(
	ctx context.Context,
	pager store.SessionCatalogPager,
	query ListQuery,
	durableQuery store.SessionCatalogPageQuery,
) (store.SessionCatalogPage, map[string]*Info, []store.SessionInfo, error) {
	infos, excluded := m.activeCatalogSnapshots(ctx, query)
	activeIDs := make([]string, 0, len(excluded)+len(infos)+len(query.InstanceMainIDs))
	activeIDs = append(activeIDs, excluded...)
	for _, info := range infos {
		activeIDs = append(activeIDs, info.ID)
	}
	if query.ExcludeInstanceMain {
		activeIDs = append(activeIDs, query.InstanceMainIDs...)
	}
	durableQuery.ExcludeIDs = activeIDs
	durable, err := pager.PageSessions(ctx, durableQuery)
	if err != nil {
		return store.SessionCatalogPage{}, nil, nil, fmt.Errorf("session: page durable catalog: %w", err)
	}
	belongs := catalogFolderMembership(query.Folder, durable.FolderMemberIDs)
	var decorate func(*Info) bool
	if query.Folder != "" {
		decorate = catalogFolderDecoration(infos, durable.Sessions, belongs)
	}
	activeByID, _, activeMatches, err := m.projectActiveCatalogRows(ctx, query, infos, excluded, decorate)
	if err != nil {
		return store.SessionCatalogPage{}, nil, nil, err
	}
	activeMatches = slices.DeleteFunc(activeMatches, func(info store.SessionInfo) bool {
		return !belongs(info.ID) || query.ExcludeInstanceMain && slices.Contains(query.InstanceMainIDs, info.ID)
	})
	return durable, activeByID, activeMatches, nil
}

func catalogFolderMembership(folder string, members map[string]struct{}) func(string) bool {
	return func(id string) bool {
		_, member := members[id]
		return folder == "" || member != (folder == store.SessionCatalogRootFolder)
	}
}

// An active child outside the partition can still contribute to a visible
// parent's child counts, for example when the child has its own placement.
func catalogFolderDecoration(
	active []*Info, durable []store.SessionInfo, belongs func(string) bool,
) func(*Info) bool {
	relevantParents := make(map[string]struct{}, len(active)+len(durable))
	for _, info := range active {
		if belongs(info.ID) {
			relevantParents[info.ID] = struct{}{}
		}
	}
	for i := range durable {
		relevantParents[durable[i].ID] = struct{}{}
	}
	return func(info *Info) bool {
		_, countsForParent := relevantParents[store.HierarchyParentSessionID(info.Lineage)]
		return belongs(info.ID) || countsForParent
	}
}
