package session

import (
	"strings"

	"github.com/compozy/compozy/internal/store"
)

func sessionCatalogPosition(info *store.SessionInfo, sortKey string) store.SessionCatalogPosition {
	position := store.SessionCatalogPosition{
		PrimaryAt:   info.UpdatedAt.UTC(),
		SecondaryAt: info.CreatedAt.UTC(),
		CreatedAt:   info.CreatedAt.UTC(),
		ID:          strings.TrimSpace(info.ID),
	}
	if sortKey == ListSortLastActivity {
		position.SecondaryAt = info.UpdatedAt.UTC()
		if info.Liveness != nil && info.Liveness.LastUpdateAt != nil && !info.Liveness.LastUpdateAt.IsZero() {
			position.PrimaryAt = info.Liveness.LastUpdateAt.UTC()
		}
	}
	if sortKey == ListSortAttention {
		attention := info.AttentionSnapshot()
		position.AttentionRank = sessionCatalogAttentionRank(info)
		if attention.AttentionChangedAt != nil && !attention.AttentionChangedAt.IsZero() {
			position.PrimaryAt = attention.AttentionChangedAt.UTC()
		}
		position.SecondaryAt = info.UpdatedAt.UTC()
	}
	return position
}

func sessionCatalogAttentionRank(info *store.SessionInfo) store.SessionCatalogAttentionRank {
	switch ClassForBadge(BadgeForInfo(sessionInfoFromCatalog(info))) {
	case AttentionNeedsYou:
		return store.SessionCatalogAttentionRankNeedsYou
	case AttentionFinished:
		return store.SessionCatalogAttentionRankFinished
	default:
		return store.SessionCatalogAttentionRankNone
	}
}

func compareSessionCatalogPosition(left store.SessionCatalogPosition, right store.SessionCatalogPosition) int {
	if left.AttentionRank != right.AttentionRank {
		return int(left.AttentionRank - right.AttentionRank)
	}
	if !left.PrimaryAt.Equal(right.PrimaryAt) {
		if left.PrimaryAt.After(right.PrimaryAt) {
			return -1
		}
		return 1
	}
	if !left.SecondaryAt.Equal(right.SecondaryAt) {
		if left.SecondaryAt.After(right.SecondaryAt) {
			return -1
		}
		return 1
	}
	if !left.CreatedAt.Equal(right.CreatedAt) {
		if left.CreatedAt.After(right.CreatedAt) {
			return -1
		}
		return 1
	}
	return strings.Compare(strings.TrimSpace(right.ID), strings.TrimSpace(left.ID))
}
