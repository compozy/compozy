package testutil

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/session"
)

func stubSessionListPage(infos []*session.Info, query session.ListQuery) session.ListPage {
	limit := query.Limit
	if limit <= 0 {
		limit = session.DefaultListLimit
	}
	filtered := make([]*session.Info, 0, len(infos))
	now := time.Now().UTC()
	for _, info := range infos {
		if !stubSessionListMatch(info, query, now) {
			continue
		}
		filtered = append(filtered, info)
	}
	slices.SortFunc(filtered, func(a, b *session.Info) int {
		return cmp.Or(
			stubSessionListTime(b, query.Sort).Compare(stubSessionListTime(a, query.Sort)),
			cmp.Compare(b.ID, a.ID),
		)
	})
	total := len(filtered)
	hasMore := total > limit
	if hasMore {
		filtered = filtered[:limit]
	}
	return session.ListPage{Sessions: filtered, HasMore: hasMore, Total: total, Limit: limit}
}

func stubSessionListMatch(info *session.Info, query session.ListQuery, now time.Time) bool {
	if info == nil {
		return false
	}
	if lineage := info.Lineage; lineage != nil && session.IsInternalSpawnRole(lineage.SpawnRole) {
		return false
	}
	if query.WorkspaceID != "" && strings.TrimSpace(info.WorkspaceID) != strings.TrimSpace(query.WorkspaceID) {
		return false
	}
	if query.State != "" && strings.TrimSpace(string(info.State)) != strings.TrimSpace(query.State) {
		return false
	}
	if query.AgentName != "" && strings.TrimSpace(info.AgentName) != strings.TrimSpace(query.AgentName) {
		return false
	}
	if query.Resumable && !session.AttachableForInfo(info, now) {
		return false
	}
	search := strings.ToLower(strings.TrimSpace(query.Search))
	if search == "" {
		return true
	}
	for _, value := range []string{
		info.ID,
		info.Name,
		info.AgentName,
		info.Provider,
	} {
		if strings.Contains(strings.ToLower(value), search) {
			return true
		}
	}
	return false
}

func stubSessionListTime(info *session.Info, sortKey string) time.Time {
	if info == nil {
		return time.Time{}
	}
	if strings.TrimSpace(sortKey) == session.ListSortCreated {
		return info.CreatedAt.UTC()
	}
	if strings.TrimSpace(sortKey) == session.ListSortLastActivity &&
		info.Liveness != nil && info.Liveness.LastUpdateAt != nil {
		return info.Liveness.LastUpdateAt.UTC()
	}
	return info.UpdatedAt.UTC()
}
