package session

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/listcursor"
	"github.com/compozy/compozy/internal/store"
)

const (
	// DefaultListLimit is the server-owned page size for public session catalogs.
	DefaultListLimit = 50
	// MaxListLimit is the largest public session catalog page.
	MaxListLimit = 100

	sessionCatalogSearchTitleAgent = "title_agent"

	ListSortRecent       = "recent"
	ListSortCreated      = "created"
	ListSortName         = "name"
	ListSortNavigator    = "navigator"
	ListSortLastActivity = "last_activity"
	ListSortAttention    = "attention"
	ListSortManual       = "manual"

	ArchiveExclude = store.SessionArchiveExclude
	ArchiveOnly    = store.SessionArchiveOnly
	ArchiveInclude = store.SessionArchiveInclude

	LaterExclude = store.SessionLaterExclude
	LaterOnly    = store.SessionLaterOnly
	LaterInclude = store.SessionLaterInclude

	sessionListCursorVersion = 2
	sessionListCursorKind    = "sessions"
)

var (
	// ErrListQueryInvalid reports unsupported filters, sorts, or limits.
	ErrListQueryInvalid = errors.New("session: list query is invalid")
	// ErrListCursorInvalid reports malformed cursors or cursors reused with another query.
	ErrListCursorInvalid = errors.New("session: list cursor is invalid")
)

// ListQuery describes one public session catalog page.
type ListQuery struct {
	Columns         []string
	HideChildren    bool
	Subagents       string
	ReadScope       store.ReadScope
	WorkspaceID     string
	WorkspaceIDs    []string
	AllWorkspaces   bool
	WorktreeID      string
	State           string
	SessionType     Type
	AgentName       string
	ParentSessionID string
	RootSessionID   string
	SearchFields    string
	Search          string
	Resumable       bool
	AttentionOnly   bool
	Badges          []Badge
	Archive         store.SessionArchiveFilter
	Later           store.SessionLaterFilter
	Sort            string
	Cursor          string
	Limit           int
	SkipTotal       bool
}

// ListPage contains one bounded public session catalog result.
type ListPage struct {
	Sessions   []*Info
	NextCursor string
	HasMore    bool
	Total      int
	Limit      int
	// Placements carries folder membership for page sessions that have one.
	Placements map[string]store.SessionPlacement
}

type sessionListFingerprint struct {
	Columns         []string                   `json:"columns,omitempty"`
	HideChildren    bool                       `json:"hide_children,omitempty"`
	Subagents       string                     `json:"subagents,omitempty"`
	ProfileID       string                     `json:"profile_id"`
	AllProfiles     bool                       `json:"all_profiles"`
	WorkspaceID     string                     `json:"workspace_id"`
	WorkspaceIDs    []string                   `json:"workspace_ids,omitempty"`
	AllWorkspaces   bool                       `json:"all_workspaces"`
	WorktreeID      string                     `json:"worktree_id"`
	State           string                     `json:"state"`
	SessionType     Type                       `json:"type"`
	AgentName       string                     `json:"agent"`
	ParentSessionID string                     `json:"parent"`
	RootSessionID   string                     `json:"root"`
	SearchFields    string                     `json:"search_fields,omitempty"`
	Search          string                     `json:"q"`
	Resumable       bool                       `json:"resumable"`
	AttentionOnly   bool                       `json:"attention"`
	Badges          []Badge                    `json:"badges"`
	Archive         store.SessionArchiveFilter `json:"archive"`
	// Later stays empty for the default exclusion so pre-Later cursors remain valid.
	Later store.SessionLaterFilter `json:"later,omitempty"`
	Sort  string                   `json:"sort"`
}

// ListPage returns a stable, bounded union of durable and active sessions.
// Active snapshots replace their durable rows before filtering, counting, and cutting.
func (m *Manager) ListPage(ctx context.Context, query ListQuery) (ListPage, error) {
	if ctx == nil {
		return ListPage{}, fmt.Errorf("%w: context is required", ErrListQueryInvalid)
	}
	normalized, err := normalizeListQuery(query)
	if err != nil {
		return ListPage{}, err
	}
	pager, ok := m.sessionCatalog.(store.SessionCatalogPager)
	if !ok || pager == nil {
		return ListPage{}, errors.New("session: paged session catalog is required")
	}
	fingerprint, err := sessionListFingerprintForQuery(normalized)
	if err != nil {
		return ListPage{}, err
	}
	after, err := decodeSessionListCursor(normalized.Cursor, fingerprint, normalized.Sort)
	if err != nil {
		return ListPage{}, err
	}

	activeByID, activeIDs, activeMatches, err := m.activeBoardSessionCatalogRows(ctx, normalized)
	if err != nil {
		return ListPage{}, err
	}
	repositoryKeys := m.catalogRepositoryKeys(ctx, normalized)
	defaultBranches := m.catalogDefaultBranches(ctx, normalized)
	durableQuery := sessionCatalogPageQuery(normalized, after, activeIDs)
	durableQuery.RepositoryKeys = repositoryKeys
	durableQuery.DefaultBranches = defaultBranches

	if err := m.refreshCatalogPullRequests(ctx, durableQuery); err != nil {
		return ListPage{}, err
	}
	durable, err := pager.PageSessions(ctx, durableQuery)
	if err != nil {
		return ListPage{}, fmt.Errorf("session: page durable catalog: %w", err)
	}

	placements, err := m.sessionCatalogPlacements(ctx, durable.Sessions, activeMatches)
	if err != nil {
		return ListPage{}, err
	}
	position := func(info *store.SessionInfo) store.SessionCatalogPosition {
		return sessionCatalogPlacedPosition(info, normalized.Sort, placements)
	}

	candidates := mergeSessionCatalogCandidates(durable.Sessions, activeMatches, after, position)

	page := ListPage{
		Total: durable.Total + len(activeMatches),
		Limit: normalized.Limit,
	}
	if len(candidates) > normalized.Limit {
		page.HasMore = true
		candidates = candidates[:normalized.Limit]
	}
	page.Sessions = projectSessionCatalogPage(candidates, activeByID)
	if err := m.populateCatalogRelations(
		ctx,
		normalized,
		page.Sessions,
		activeByID,
		repositoryKeys,
		defaultBranches,
	); err != nil {
		return ListPage{}, err
	}
	page.Placements = pageSessionPlacements(candidates, placements)
	if page.HasMore && len(candidates) > 0 {
		page.NextCursor, err = encodeSessionListCursor(
			fingerprint,
			position(&candidates[len(candidates)-1]),
		)
		if err != nil {
			return ListPage{}, err
		}
	}
	return page, nil
}

func mergeSessionCatalogCandidates(
	durable, activeMatches []store.SessionInfo,
	after *store.SessionCatalogPosition,
	position func(*store.SessionInfo) store.SessionCatalogPosition,
) []store.SessionInfo {
	type candidate struct {
		info     *store.SessionInfo
		position store.SessionCatalogPosition
	}
	candidates := make([]candidate, 0, len(durable)+len(activeMatches))
	for index := range durable {
		info := &durable[index]
		candidates = append(candidates, candidate{info: info, position: position(info)})
	}
	for index := range activeMatches {
		info := &activeMatches[index]
		key := position(info)
		if after == nil || compareSessionCatalogPosition(key, *after) > 0 {
			candidates = append(candidates, candidate{info: info, position: key})
		}
	}
	slices.SortFunc(candidates, func(a, b candidate) int {
		return compareSessionCatalogPosition(a.position, b.position)
	})

	result := make([]store.SessionInfo, len(candidates))
	for index, candidate := range candidates {
		result[index] = *candidate.info
	}
	return result
}

func sessionCatalogPageQuery(
	normalized ListQuery,
	after *store.SessionCatalogPosition,
	activeIDs []string,
) store.SessionCatalogPageQuery {
	query := store.SessionCatalogPageQuery{
		Columns:           normalized.Columns,
		HideChildren:      normalized.HideChildren,
		Subagents:         normalized.Subagents,
		SkipTotal:         normalized.SkipTotal,
		ReadScope:         normalized.ReadScope,
		WorkspaceID:       normalized.WorkspaceID,
		WorkspaceIDs:      normalized.WorkspaceIDs,
		WorktreeID:        normalized.WorktreeID,
		State:             normalized.State,
		SessionType:       string(normalized.SessionType),
		AgentName:         normalized.AgentName,
		ParentSessionID:   normalized.ParentSessionID,
		RootSessionID:     normalized.RootSessionID,
		SearchFields:      normalized.SearchFields,
		Search:            normalized.Search,
		AttentionOnly:     normalized.AttentionOnly,
		Badges:            badgeStrings(normalized.Badges),
		Resumable:         normalized.Resumable,
		Archive:           normalized.Archive,
		Later:             normalized.Later,
		Sort:              normalized.Sort,
		Limit:             normalized.Limit + 1,
		After:             after,
		ExcludeIDs:        activeIDs,
		ExcludeSpawnRoles: []string{SpawnRoleAutoTitle},
	}
	if normalized.SessionType == "" {
		query.ExcludeVerdictOnly = true
	}
	return query
}

func (m *Manager) activeSessionCatalogRows(
	query ListQuery,
) (map[string]*Info, []string, []store.SessionInfo) {
	infos, excludedIDs := m.activeCatalogSnapshots(query)
	return m.activeSessionCatalogRowsFromInfos(query, infos, excludedIDs)
}

func (m *Manager) activeSessionCatalogRowsFromInfos(
	query ListQuery,
	activeInfos []*Info,
	excludedIDs []string,
) (map[string]*Info, []string, []store.SessionInfo) {
	activeByID := make(map[string]*Info, len(activeInfos)+len(excludedIDs))
	activeIDs := make([]string, 0, len(activeInfos)+len(excludedIDs))
	for _, id := range excludedIDs {
		// Exclude stale durable identities even when their live owner moved outside the requested scope.
		activeByID[id] = nil
		activeIDs = append(activeIDs, id)
	}
	activeMatches := make([]store.SessionInfo, 0, len(activeInfos))
	now := m.now().UTC()
	for _, info := range activeInfos {
		if info == nil || strings.TrimSpace(info.ID) == "" {
			continue
		}
		info = normalizeExpiredSessionAttach(info, now)
		activeByID[info.ID] = info
		activeIDs = append(activeIDs, info.ID)
		if sessionMatchesListQuery(info, query, now) {
			activeMatches = append(activeMatches, sessionCatalogInfoFromRuntime(info))
		}
	}
	return activeByID, activeIDs, activeMatches
}

func sessionListFingerprintForQuery(query ListQuery) (string, error) {
	fingerprint, err := listcursor.Fingerprint(sessionListFingerprint{
		Columns:         query.Columns,
		HideChildren:    query.HideChildren,
		Subagents:       query.Subagents,
		ProfileID:       query.ReadScope.ProfileID,
		AllProfiles:     query.ReadScope.AllProfiles,
		WorkspaceID:     query.WorkspaceID,
		WorkspaceIDs:    query.WorkspaceIDs,
		AllWorkspaces:   query.AllWorkspaces,
		WorktreeID:      query.WorktreeID,
		State:           query.State,
		SessionType:     query.SessionType,
		AgentName:       query.AgentName,
		ParentSessionID: query.ParentSessionID,
		RootSessionID:   query.RootSessionID,
		SearchFields:    query.SearchFields,
		Search:          query.Search,
		Resumable:       query.Resumable,
		AttentionOnly:   query.AttentionOnly,
		Badges:          append([]Badge(nil), query.Badges...),
		Archive:         query.Archive,
		Later:           laterFingerprint(query.Later),
		Sort:            query.Sort,
	})
	if err != nil {
		return "", fmt.Errorf("session: fingerprint list query: %w", err)
	}
	return fingerprint, nil
}

func projectSessionCatalogPage(
	sessions []store.SessionInfo,
	activeByID map[string]*Info,
) []*Info {
	infos := make([]*Info, 0, len(sessions))
	for index := range sessions {
		stored := &sessions[index]
		if active := activeByID[stored.ID]; active != nil {
			infos = append(infos, active)
			continue
		}
		infos = append(infos, sessionInfoFromCatalog(stored))
	}
	return infos
}

func normalizeSubagentListFilter(value string) (string, error) {
	switch value {
	case "", "include":
		return "", nil
	case childrenExclude, "only":
		return value, nil
	default:
		return "", fmt.Errorf("%w: unsupported subagents %q", ErrListQueryInvalid, value)
	}
}
