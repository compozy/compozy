package session

import (
	"context"
	"fmt"
	"maps"

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
