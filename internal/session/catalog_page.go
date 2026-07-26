package session

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/compozy/agh/internal/listcursor"
	"github.com/compozy/agh/internal/store"
)

const (
	// DefaultListLimit is the server-owned page size for public session catalogs.
	DefaultListLimit = 50
	// MaxListLimit is the largest public session catalog page.
	MaxListLimit = 100

	ListSortRecent       = "recent"
	ListSortLastActivity = "last_activity"

	sessionListCursorVersion = 1
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
	WorkspaceID string
	State       string
	SessionType Type
	AgentName   string
	Search      string
	Resumable   bool
	Sort        string
	Cursor      string
	Limit       int
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
	WorkspaceID string `json:"workspace_id"`
	State       string `json:"state"`
	SessionType Type   `json:"type"`
	AgentName   string `json:"agent"`
	Search      string `json:"q"`
	Resumable   bool   `json:"resumable"`
	Sort        string `json:"sort"`
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

	activeByID, activeIDs, activeMatches := m.activeSessionCatalogRows(normalized)

	durable, err := pager.PageSessions(ctx, store.SessionCatalogPageQuery{
		WorkspaceID:         normalized.WorkspaceID,
		State:               normalized.State,
		SessionType:         string(normalized.SessionType),
		AgentName:           normalized.AgentName,
		Search:              normalized.Search,
		Resumable:           normalized.Resumable,
		Sort:                normalized.Sort,
		Limit:               normalized.Limit + 1,
		After:               after,
		ExcludeIDs:          activeIDs,
		ExcludeSessionTypes: []string{string(SessionTypeDream)},
		ExcludeSpawnRoles:   []string{SpawnRoleMemoryExtractor, SpawnRoleAutoTitle},
	})
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
	query.WorkspaceID = strings.TrimSpace(query.WorkspaceID)
	query.State = strings.TrimSpace(query.State)
	query.SessionType = Type(strings.TrimSpace(string(query.SessionType)))
	query.AgentName = strings.TrimSpace(query.AgentName)
	query.Search = strings.ToLower(strings.TrimSpace(query.Search))
	query.Sort = strings.TrimSpace(query.Sort)
	query.Cursor = strings.TrimSpace(query.Cursor)
	if query.Sort == "" {
		query.Sort = ListSortRecent
	}
	if query.Limit == 0 {
		query.Limit = DefaultListLimit
	}
	if query.Limit < 1 || query.Limit > MaxListLimit {
		return ListQuery{}, fmt.Errorf("%w: limit must be between 1 and %d", ErrListQueryInvalid, MaxListLimit)
	}
	switch query.Sort {
	case ListSortRecent, ListSortLastActivity:
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
	if info == nil || info.Type == SessionTypeDream {
		return false
	}
	if lineage := info.Lineage; lineage != nil && IsInternalSpawnRole(lineage.SpawnRole) {
		return false
	}
	if query.WorkspaceID != "" && strings.TrimSpace(info.WorkspaceID) != query.WorkspaceID {
		return false
	}
	if query.State != "" && strings.TrimSpace(string(info.State)) != query.State {
		return false
	}
	if query.SessionType != "" && normalizeSessionType(info.Type) != query.SessionType {
		return false
	}
	if query.AgentName != "" && strings.TrimSpace(info.AgentName) != query.AgentName {
		return false
	}
	if query.Resumable && !AttachableForInfo(info, now) {
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

func sessionCatalogPosition(info store.SessionInfo, sortKey string) store.SessionCatalogPosition {
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
	return position
}

func compareSessionCatalogPosition(left store.SessionCatalogPosition, right store.SessionCatalogPosition) int {
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

func sessionListFingerprintForQuery(query ListQuery) (string, error) {
	fingerprint, err := listcursor.Fingerprint(sessionListFingerprint{
		WorkspaceID: query.WorkspaceID,
		State:       query.State,
		SessionType: query.SessionType,
		AgentName:   query.AgentName,
		Search:      query.Search,
		Resumable:   query.Resumable,
		Sort:        query.Sort,
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
	return &Info{
		ID:                   strings.TrimSpace(info.ID),
		Name:                 strings.TrimSpace(info.Name),
		AgentName:            strings.TrimSpace(info.AgentName),
		Provider:             strings.TrimSpace(info.Provider),
		WorkspaceID:          strings.TrimSpace(info.WorkspaceID),
		NetworkParticipation: info.NetworkSpecSnapshot(),
		Type:                 Type(strings.TrimSpace(info.SessionType)),
		Lineage:              store.CloneSessionLineage(info.Lineage),
		State:                State(strings.TrimSpace(info.State)),
		StopReason:           info.StopReason,
		StopDetail:           strings.TrimSpace(info.StopDetail),
		Failure:              store.CloneSessionFailure(info.Failure),
		ACPSessionID:         stringValue(info.ACPSessionID),
		Liveness:             store.CloneSessionLivenessMeta(info.Liveness),
		Sandbox:              cloneSessionSandboxMeta(info.Sandbox),
		SoulSnapshotID:       strings.TrimSpace(info.SoulSnapshotID),
		SoulDigest:           strings.TrimSpace(info.SoulDigest),
		ParentSoulDigest:     strings.TrimSpace(info.ParentSoulDigest),
		AttachedTo:           strings.TrimSpace(info.AttachedTo),
		AttachExpiresAt:      cloneTimePointer(info.AttachExpiresAt),
		TranscriptEpoch:      info.TranscriptEpoch,
		CreatedAt:            info.CreatedAt,
		UpdatedAt:            info.UpdatedAt,
	}
}
