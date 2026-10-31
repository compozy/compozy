package session

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

// normalizeListBreadth requires exactly one workspace or the All projects read with its narrowing.
func normalizeListBreadth(query *ListQuery) error {
	query.WorkspaceID = strings.TrimSpace(query.WorkspaceID)
	if (query.WorkspaceID != "") == query.AllWorkspaces {
		return fmt.Errorf("%w: choose exactly one workspace or all workspaces", ErrListQueryInvalid)
	}
	ids, err := NormalizeWorkspaceIDs(query.WorkspaceIDs, query.AllWorkspaces)
	if err != nil {
		return err
	}
	query.WorkspaceIDs = ids
	return nil
}

func normalizeListQuery(query ListQuery) (ListQuery, error) {
	filter, err := normalizeSubagentListFilter(query.Subagents)
	if err != nil {
		return ListQuery{}, err
	}
	query.Subagents = filter
	columns, err := ParseBoardColumns(query.Columns)
	if err != nil {
		return ListQuery{}, err
	}
	slices.Sort(columns)
	query.Columns = columns
	query.ReadScope.ProfileID = strings.TrimSpace(query.ReadScope.ProfileID)
	if err := query.ReadScope.Validate(); err != nil {
		return ListQuery{}, fmt.Errorf("%w: %w", ErrListQueryInvalid, err)
	}
	if err := normalizeListBreadth(&query); err != nil {
		return ListQuery{}, err
	}
	query.WorktreeID = strings.TrimSpace(query.WorktreeID)
	query.State = strings.TrimSpace(query.State)
	query.SessionType = Type(strings.TrimSpace(string(query.SessionType)))
	query.AgentName = strings.TrimSpace(query.AgentName)
	query.ParentSessionID = strings.TrimSpace(query.ParentSessionID)
	query.RootSessionID = strings.TrimSpace(query.RootSessionID)
	if err := normalizeListSearch(&query); err != nil {
		return ListQuery{}, err
	}
	badges, err := ParseBadgeFilters(badgeStrings(query.Badges))
	if err != nil {
		return ListQuery{}, fmt.Errorf("%w: %w", ErrListQueryInvalid, err)
	}
	query.Badges = badges
	query.Archive = store.SessionArchiveFilter(strings.TrimSpace(string(query.Archive)))
	query.Sort = strings.TrimSpace(query.Sort)
	query.Cursor = strings.TrimSpace(query.Cursor)
	if (query.AttentionOnly || len(query.Badges) > 0) && query.Sort == "" {
		query.Sort = ListSortAttention
	} else if query.Sort == "" {
		query.Sort = ListSortRecent
	}
	if query.Archive == "" {
		query.Archive = ArchiveExclude
	}
	if err := query.Archive.Validate(); err != nil {
		return ListQuery{}, fmt.Errorf("%w: %w", ErrListQueryInvalid, err)
	}
	later, err := normalizeLaterFilter(query.Later)
	if err != nil {
		return ListQuery{}, err
	}
	query.Later = later
	if query.Limit == 0 {
		query.Limit = DefaultListLimit
	}
	if query.Limit < 1 || query.Limit > MaxListLimit {
		return ListQuery{}, fmt.Errorf("%w: limit must be between 1 and %d", ErrListQueryInvalid, MaxListLimit)
	}
	switch query.Sort {
	case ListSortRecent, ListSortCreated, ListSortName, ListSortNavigator, ListSortLastActivity, ListSortAttention,
		ListSortManual:
	default:
		return ListQuery{}, fmt.Errorf("%w: unsupported sort %q", ErrListQueryInvalid, query.Sort)
	}
	switch State(query.State) {
	case "", StateStarting, StateActive, StateStopping, StateStopped:
	default:
		return ListQuery{}, fmt.Errorf("%w: unsupported state %q", ErrListQueryInvalid, query.State)
	}
	switch query.SessionType {
	case "", SessionTypeUser, SessionTypeSystem, SessionTypeCoordinator, SessionTypeSpawned:
	default:
		return ListQuery{}, fmt.Errorf("%w: unsupported type %q", ErrListQueryInvalid, query.SessionType)
	}
	return query, nil
}

func normalizeListSearch(query *ListQuery) error {
	query.SearchFields = strings.TrimSpace(query.SearchFields)
	if query.SearchFields != "" && query.SearchFields != sessionCatalogSearchTitleAgent {
		return fmt.Errorf("%w: unsupported search_fields %q", ErrListQueryInvalid, query.SearchFields)
	}
	query.Search = strings.TrimSpace(query.Search)
	if query.SearchFields == sessionCatalogSearchTitleAgent {
		query.Search = store.LowerSessionCatalogSearchText(query.Search)
	} else {
		query.Search = strings.ToLower(query.Search)
	}
	return nil
}

func sessionMatchesListQuery(info *Info, query ListQuery, now time.Time) bool {
	if !sessionMatchesIdentityFilters(info, query) || !sessionMatchesLineageFilters(info, query) ||
		query.Resumable && !AttachableForInfo(info, now) || !sessionMatchesArchiveFilter(info, query.Archive) ||
		!sessionMatchesLaterFilter(info, query.Later) {
		return false
	}
	badge := BadgeForInfo(info)
	column := info.BoardColumn
	if column == "" {
		column = BoardColumnFor(badge, "")
	}
	if len(query.Columns) > 0 && !slices.Contains(query.Columns, string(column)) {
		return false
	}
	if query.AttentionOnly && ClassForBadge(badge) != AttentionNeedsYou {
		return false
	}
	if len(query.Badges) > 0 && !badgeFilterContains(query.Badges, badge) {
		return false
	}
	search := strings.TrimSpace(query.Search)
	if search == "" {
		return true
	}
	if query.SearchFields == sessionCatalogSearchTitleAgent {
		search = store.LowerSessionCatalogSearchText(search)
		return strings.Contains(store.LowerSessionCatalogSearchText(sessionCatalogDisplayTitle(info)), search) ||
			strings.Contains(store.LowerSessionCatalogSearchText(strings.TrimSpace(info.AgentName)), search)
	}
	search = strings.ToLower(search)
	for _, value := range []string{
		info.ID,
		info.Name,
		info.AgentName,
		info.Provider,
	} {
		if strings.Contains(strings.ToLower(strings.TrimSpace(value)), search) {
			return true
		}
	}
	return false
}

func sessionMatchesIdentityFilters(info *Info, query ListQuery) bool {
	if !isPublicSessionCatalogInfo(info) ||
		(query.SessionType == "" && isInternalVerificationSession(info)) {
		return false
	}
	return query.ReadScope.Matches(info.ProfileID) &&
		(query.WorkspaceID == "" || strings.TrimSpace(info.WorkspaceID) == query.WorkspaceID ||
			(query.WorkspaceID == workspaceaccess.UserSelector && info.WorkspaceID == "")) &&
		sessionMatchesWorkspaceIDs(info.WorkspaceID, query.WorkspaceIDs) &&
		(query.WorktreeID == "" || strings.TrimSpace(info.WorktreeID) == query.WorktreeID) &&
		(query.State == "" || strings.TrimSpace(string(info.State)) == query.State) &&
		(query.SessionType == "" || normalizeSessionType(info.Type) == query.SessionType) &&
		(query.AgentName == "" || strings.TrimSpace(info.AgentName) == query.AgentName)
}

func isPublicSessionCatalogInfo(info *Info) bool {
	if info == nil {
		return false
	}
	lineage := info.Lineage
	return lineage == nil || !IsInternalSpawnRole(lineage.SpawnRole)
}

func sessionMatchesLineageFilters(info *Info, query ListQuery) bool {
	if query.HideChildren && store.HierarchyParentSessionID(info.Lineage) != "" {
		return false
	}
	isSubagent := info.Lineage != nil && info.Lineage.SpawnRole == store.SubagentSpawnRole
	if (query.Subagents == childrenExclude && isSubagent) || (query.Subagents == "only" && !isSubagent) {
		return false
	}
	if query.ParentSessionID == "" && query.RootSessionID == "" {
		return true
	}
	lineage := store.NormalizeSessionLineage(info.ID, info.Lineage)
	if query.ParentSessionID != "" && lineage.ParentSessionID != query.ParentSessionID {
		return false
	}
	if query.RootSessionID != "" && lineage.RootSessionID != query.RootSessionID {
		return false
	}
	return true
}

// normalizeLaterFilter hides parked sessions unless the read asks for them.
func normalizeLaterFilter(filter store.SessionLaterFilter) (store.SessionLaterFilter, error) {
	filter = store.SessionLaterFilter(strings.TrimSpace(string(filter)))
	if filter == "" {
		filter = LaterExclude
	}
	if err := filter.Validate(); err != nil {
		return "", fmt.Errorf("%w: %w", ErrListQueryInvalid, err)
	}
	return filter, nil
}

func sessionMatchesLaterFilter(info *Info, filter store.SessionLaterFilter) bool {
	parked := info != nil && info.Later != nil
	switch filter {
	case LaterExclude:
		return !parked
	case LaterOnly:
		return parked
	default:
		return true
	}
}

func laterFingerprint(filter store.SessionLaterFilter) store.SessionLaterFilter {
	if filter == LaterExclude {
		return ""
	}
	return filter
}

func sessionMatchesArchiveFilter(info *Info, filter store.SessionArchiveFilter) bool {
	archived := info != nil && info.ArchivedAt != nil
	switch filter {
	case ArchiveExclude:
		return !archived
	case ArchiveOnly:
		return archived
	default:
		return true
	}
}
