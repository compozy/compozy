package session

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/listcursor"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
)

const (
	// DefaultListLimit is the server-owned page size for public session catalogs.
	DefaultListLimit = 50
	// MaxListLimit is the largest public session catalog page.
	MaxListLimit = 100

	ListSortRecent       = "recent"
	ListSortLastActivity = "last_activity"
	ListSortAttention    = "attention"

	ArchiveExclude = store.SessionArchiveExclude
	ArchiveOnly    = store.SessionArchiveOnly
	ArchiveInclude = store.SessionArchiveInclude

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
	ReadScope       store.ReadScope
	WorkspaceID     string
	AllWorkspaces   bool
	WorktreeID      string
	State           string
	SessionType     Type
	AgentName       string
	ParentSessionID string
	RootSessionID   string
	Search          string
	Resumable       bool
	AttentionOnly   bool
	Badges          []Badge
	Archive         store.SessionArchiveFilter
	Sort            string
	Cursor          string
	Limit           int
}

// ListPage contains one bounded public session catalog result.
type ListPage struct {
	Sessions   []*Info
	NextCursor string
	HasMore    bool
	Total      int
	Limit      int
}

type sessionListFingerprint struct {
	ProfileID       string                     `json:"profile_id"`
	AllProfiles     bool                       `json:"all_profiles"`
	WorkspaceID     string                     `json:"workspace_id"`
	AllWorkspaces   bool                       `json:"all_workspaces"`
	WorktreeID      string                     `json:"worktree_id"`
	State           string                     `json:"state"`
	SessionType     Type                       `json:"type"`
	AgentName       string                     `json:"agent"`
	ParentSessionID string                     `json:"parent"`
	RootSessionID   string                     `json:"root"`
	Search          string                     `json:"q"`
	Resumable       bool                       `json:"resumable"`
	AttentionOnly   bool                       `json:"attention"`
	Badges          []Badge                    `json:"badges"`
	Archive         store.SessionArchiveFilter `json:"archive"`
	Sort            string                     `json:"sort"`
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
	after, err := decodeSessionListCursor(normalized.Cursor, fingerprint)
	if err != nil {
		return ListPage{}, err
	}
	if normalized.AttentionOnly || len(normalized.Badges) > 0 {
		return m.listAttentionPage(ctx, normalized, pager, fingerprint, after)
	}

	activeByID, activeIDs, activeMatches := m.activeSessionCatalogRows(normalized)

	durable, err := pager.PageSessions(ctx, sessionCatalogPageQuery(normalized, after, activeIDs))
	if err != nil {
		return ListPage{}, fmt.Errorf("session: page durable catalog: %w", err)
	}

	candidates := make([]store.SessionInfo, 0, len(durable.Sessions)+len(activeMatches))
	candidates = append(candidates, durable.Sessions...)
	for _, info := range activeMatches {
		if after == nil || compareSessionCatalogPosition(sessionCatalogPosition(info, normalized.Sort), *after) > 0 {
			candidates = append(candidates, info)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return compareSessionCatalogPosition(
			sessionCatalogPosition(candidates[i], normalized.Sort),
			sessionCatalogPosition(candidates[j], normalized.Sort),
		) < 0
	})

	page := ListPage{
		Total: durable.Total + len(activeMatches),
		Limit: normalized.Limit,
	}
	if len(candidates) > normalized.Limit {
		page.HasMore = true
		candidates = candidates[:normalized.Limit]
	}
	page.Sessions = projectSessionCatalogPage(candidates, activeByID)
	if page.HasMore && len(candidates) > 0 {
		page.NextCursor, err = encodeSessionListCursor(
			fingerprint,
			sessionCatalogPosition(candidates[len(candidates)-1], normalized.Sort),
		)
		if err != nil {
			return ListPage{}, err
		}
	}
	return page, nil
}

func sessionCatalogPageQuery(
	normalized ListQuery,
	after *store.SessionCatalogPosition,
	activeIDs []string,
) store.SessionCatalogPageQuery {
	return store.SessionCatalogPageQuery{
		ReadScope:           normalized.ReadScope,
		WorkspaceID:         normalized.WorkspaceID,
		WorktreeID:          normalized.WorktreeID,
		State:               normalized.State,
		SessionType:         string(normalized.SessionType),
		AgentName:           normalized.AgentName,
		ParentSessionID:     normalized.ParentSessionID,
		RootSessionID:       normalized.RootSessionID,
		Search:              normalized.Search,
		Resumable:           normalized.Resumable,
		Archive:             normalized.Archive,
		Sort:                normalized.Sort,
		Limit:               normalized.Limit + 1,
		After:               after,
		ExcludeIDs:          activeIDs,
		ExcludeSessionTypes: []string{string(SessionTypeDream)},
		ExcludeSpawnRoles:   []string{SpawnRoleMemoryExtractor, SpawnRoleAutoTitle},
	}
}

func (m *Manager) activeSessionCatalogRows(
	query ListQuery,
) (map[string]*Info, []string, []store.SessionInfo) {
	activeInfos := m.List()
	activeByID := make(map[string]*Info, len(activeInfos))
	activeIDs := make([]string, 0, len(activeInfos))
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

func normalizeListQuery(query ListQuery) (ListQuery, error) {
	query.ReadScope.ProfileID = strings.TrimSpace(query.ReadScope.ProfileID)
	if err := query.ReadScope.Validate(); err != nil {
		return ListQuery{}, fmt.Errorf("%w: %w", ErrListQueryInvalid, err)
	}
	query.WorkspaceID = strings.TrimSpace(query.WorkspaceID)
	if (query.WorkspaceID != "") == query.AllWorkspaces {
		return ListQuery{}, fmt.Errorf(
			"%w: choose exactly one workspace or all workspaces",
			ErrListQueryInvalid,
		)
	}
	query.WorktreeID = strings.TrimSpace(query.WorktreeID)
	query.State = strings.TrimSpace(query.State)
	query.SessionType = Type(strings.TrimSpace(string(query.SessionType)))
	query.AgentName = strings.TrimSpace(query.AgentName)
	query.ParentSessionID = strings.TrimSpace(query.ParentSessionID)
	query.RootSessionID = strings.TrimSpace(query.RootSessionID)
	query.Search = strings.ToLower(strings.TrimSpace(query.Search))
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
	if query.Limit == 0 {
		query.Limit = DefaultListLimit
	}
	if query.Limit < 1 || query.Limit > MaxListLimit {
		return ListQuery{}, fmt.Errorf("%w: limit must be between 1 and %d", ErrListQueryInvalid, MaxListLimit)
	}
	switch query.Sort {
	case ListSortRecent, ListSortLastActivity, ListSortAttention:
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

func sessionMatchesListQuery(info *Info, query ListQuery, now time.Time) bool {
	if !sessionMatchesIdentityFilters(info, query) || !sessionMatchesLineageFilters(info, query) ||
		query.Resumable && !AttachableForInfo(info, now) || !sessionMatchesArchiveFilter(info, query.Archive) {
		return false
	}
	badge := BadgeForInfo(info)
	if query.AttentionOnly && ClassForBadge(badge) != AttentionNeedsYou {
		return false
	}
	if len(query.Badges) > 0 && !badgeFilterContains(query.Badges, badge) {
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
		info.NetworkParticipation.ChannelID,
	} {
		if strings.Contains(strings.ToLower(strings.TrimSpace(value)), search) {
			return true
		}
	}
	return false
}

func sessionMatchesIdentityFilters(info *Info, query ListQuery) bool {
	if !isPublicSessionCatalogInfo(info) {
		return false
	}
	return query.ReadScope.Matches(info.ProfileID) &&
		(query.WorkspaceID == "" || strings.TrimSpace(info.WorkspaceID) == query.WorkspaceID) &&
		(query.WorktreeID == "" || strings.TrimSpace(info.WorktreeID) == query.WorktreeID) &&
		(query.State == "" || strings.TrimSpace(string(info.State)) == query.State) &&
		(query.SessionType == "" || normalizeSessionType(info.Type) == query.SessionType) &&
		(query.AgentName == "" || strings.TrimSpace(info.AgentName) == query.AgentName)
}

func isPublicSessionCatalogInfo(info *Info) bool {
	if info == nil || info.Type == SessionTypeDream {
		return false
	}
	lineage := info.Lineage
	return lineage == nil || !IsInternalSpawnRole(lineage.SpawnRole)
}

func sessionMatchesLineageFilters(info *Info, query ListQuery) bool {
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

func sessionListFingerprintForQuery(query ListQuery) (string, error) {
	fingerprint, err := listcursor.Fingerprint(sessionListFingerprint{
		ProfileID:       query.ReadScope.ProfileID,
		AllProfiles:     query.ReadScope.AllProfiles,
		WorkspaceID:     query.WorkspaceID,
		AllWorkspaces:   query.AllWorkspaces,
		WorktreeID:      query.WorktreeID,
		State:           query.State,
		SessionType:     query.SessionType,
		AgentName:       query.AgentName,
		ParentSessionID: query.ParentSessionID,
		RootSessionID:   query.RootSessionID,
		Search:          query.Search,
		Resumable:       query.Resumable,
		AttentionOnly:   query.AttentionOnly,
		Badges:          append([]Badge(nil), query.Badges...),
		Archive:         query.Archive,
		Sort:            query.Sort,
	})
	if err != nil {
		return "", fmt.Errorf("session: fingerprint list query: %w", err)
	}
	return fingerprint, nil
}

func decodeSessionListCursor(raw string, fingerprint string) (*store.SessionCatalogPosition, error) {
	if raw == "" {
		return nil, nil
	}
	position, err := listcursor.Decode[store.SessionCatalogPosition](
		raw,
		sessionListCursorVersion,
		sessionListCursorKind,
		fingerprint,
		listcursor.DefaultMaxEncodedSize,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrListCursorInvalid, err)
	}
	if err := position.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrListCursorInvalid, err)
	}
	position.PrimaryAt = position.PrimaryAt.UTC()
	position.SecondaryAt = position.SecondaryAt.UTC()
	position.CreatedAt = position.CreatedAt.UTC()
	position.ID = strings.TrimSpace(position.ID)
	return &position, nil
}

func encodeSessionListCursor(fingerprint string, position store.SessionCatalogPosition) (string, error) {
	encoded, err := listcursor.Encode(
		sessionListCursorVersion,
		sessionListCursorKind,
		fingerprint,
		position,
	)
	if err != nil {
		return "", fmt.Errorf("session: encode list cursor: %w", err)
	}
	return encoded, nil
}

func projectSessionCatalogPage(
	sessions []store.SessionInfo,
	activeByID map[string]*Info,
) []*Info {
	infos := make([]*Info, 0, len(sessions))
	for _, stored := range sessions {
		if active := activeByID[stored.ID]; active != nil {
			infos = append(infos, active)
			continue
		}
		infos = append(infos, sessionInfoFromCatalog(stored))
	}
	return infos
}

func sessionInfoFromCatalog(info store.SessionInfo) *Info {
	attention := info.AttentionSnapshot()
	return &Info{
		ID:                       strings.TrimSpace(info.ID),
		ProfileID:                strings.TrimSpace(info.ProfileID),
		Name:                     strings.TrimSpace(info.Name),
		AgentName:                strings.TrimSpace(info.AgentName),
		Provider:                 strings.TrimSpace(info.Provider),
		Model:                    strings.TrimSpace(info.Model),
		ReasoningEffort:          strings.TrimSpace(info.ReasoningEffort),
		Speed:                    info.Speed,
		ACPOptions:               ACPOptionSelectionsFromStore(info.ACPOptionsValue()),
		SpeedResolution:          speedpkg.CloneResolution(info.SpeedResolution),
		RuntimeStatus:            info.RuntimeStatus,
		RuntimeTransition:        info.RuntimeTransition,
		RuntimeFailure:           strings.TrimSpace(info.RuntimeFailure),
		RuntimeGeneration:        info.RuntimeGeneration,
		RuntimeRecovery:          info.RuntimeRecoveryValue(),
		SelectedRuntime:          runtimeSelectionFromSessionStore(info.SelectedRuntime),
		RuntimeSelectionRevision: info.RuntimeSelectionRevision,
		WorkspaceID:              strings.TrimSpace(info.WorkspaceID),
		WorktreeID:               strings.TrimSpace(info.WorktreeID),
		NetworkParticipation:     info.NetworkSpecSnapshot(),
		Type:                     Type(strings.TrimSpace(info.SessionType)),
		Lineage:                  store.CloneSessionLineage(info.Lineage),
		State:                    State(strings.TrimSpace(info.State)),
		StopReason:               info.StopReason,
		StopDetail:               strings.TrimSpace(info.StopDetail),
		Failure:                  store.CloneSessionFailure(info.Failure),
		ACPSessionID:             stringValue(info.ACPSessionID),
		Liveness:                 store.CloneSessionLivenessMeta(info.Liveness),
		Sandbox:                  cloneSessionSandboxMeta(info.Sandbox),
		SoulSnapshotID:           strings.TrimSpace(info.SoulSnapshotID),
		SoulDigest:               strings.TrimSpace(info.SoulDigest),
		ParentSoulDigest:         strings.TrimSpace(info.ParentSoulDigest),
		AttachedTo:               strings.TrimSpace(info.AttachedTo),
		AttachExpiresAt:          cloneTimePointer(info.AttachExpiresAt),
		TranscriptEpoch:          info.TranscriptEpoch,
		PendingPermissionCount:   attention.PendingPermissionCount,
		PendingClarifyCount:      attention.PendingClarifyCount,
		AttentionRevision:        attention.AttentionRevision,
		LastSettledRevision:      attention.LastSettledRevision,
		LastSeenRevision:         attention.LastSeenRevision,
		LastSeenAt:               cloneTimePointer(attention.LastSeenAt),
		AttentionChangedAt:       cloneTimePointer(attention.AttentionChangedAt),
		ArchivedAt:               cloneTimePointer(info.ArchivedAt),
		CreatedAt:                info.CreatedAt,
		UpdatedAt:                info.UpdatedAt,
	}
}
