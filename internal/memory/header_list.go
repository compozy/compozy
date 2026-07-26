package memory

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/compozy/agh/internal/listcursor"
	memcontract "github.com/compozy/agh/internal/memory/contract"
)

const (
	// HeaderListSortRecent orders memory headers from newest to oldest.
	HeaderListSortRecent = "recent"
	// HeaderListSortName orders memory headers by normalized display name.
	HeaderListSortName = "name"

	// DefaultHeaderListLimit bounds public memory catalog reads by default.
	DefaultHeaderListLimit = 50
	// MaxHeaderListLimit caps one public memory catalog page.
	MaxHeaderListLimit = 200

	headerListCursorVersion = 1
	headerListCursorKind    = "memory_headers"
)

var (
	// ErrHeaderListCursorInvalid reports a malformed cursor or one reused with a different query.
	ErrHeaderListCursorInvalid = errors.New("memory: header list cursor is invalid")
)

// HeaderListQuery captures one canonical memory catalog page request.
type HeaderListQuery struct {
	Scope         memcontract.Scope
	WorkspaceID   string
	AgentName     string
	AgentTier     memcontract.AgentTier
	Type          memcontract.Type
	IncludeSystem bool
	Sort          string
	Cursor        string
	Limit         int
}

// HeaderListPage is one filtered, ordered, counted memory header page.
type HeaderListPage struct {
	Headers    []memcontract.Header
	NextCursor string
	HasMore    bool
	Total      int
	Limit      int
}

type headerListFingerprint struct {
	Scope         memcontract.Scope     `json:"scope"`
	WorkspaceID   string                `json:"workspace_id"`
	AgentName     string                `json:"agent_name"`
	AgentTier     memcontract.AgentTier `json:"agent_tier"`
	Type          memcontract.Type      `json:"type"`
	IncludeSystem bool                  `json:"include_system"`
	Sort          string                `json:"sort"`
}

type headerListCursorPosition struct {
	ModTime     time.Time             `json:"mod_time"`
	Name        string                `json:"name"`
	Scope       memcontract.Scope     `json:"scope"`
	WorkspaceID string                `json:"workspace_id"`
	AgentName   string                `json:"agent_name"`
	AgentTier   memcontract.AgentTier `json:"agent_tier"`
	Filename    string                `json:"filename"`
}

// BuildHeaderListPage filters and orders the complete visible catalog before applying a stable cut.
func BuildHeaderListPage(headers []memcontract.Header, query HeaderListQuery) (HeaderListPage, error) {
	normalized, err := normalizeHeaderListQuery(query)
	if err != nil {
		return HeaderListPage{}, err
	}

	filtered := make([]memcontract.Header, 0, len(headers))
	for _, header := range headers {
		if headerMatchesListQuery(header, normalized) {
			filtered = append(filtered, header)
		}
	}
	sort.Slice(filtered, func(left int, right int) bool {
		return compareHeaderListKeys(
			headerListCursorPositionFromHeader(filtered[left]),
			headerListCursorPositionFromHeader(filtered[right]),
			normalized.Sort,
		) < 0
	})

	fingerprint, err := headerListQueryFingerprint(normalized)
	if err != nil {
		return HeaderListPage{}, err
	}
	start, err := headerListPageStart(filtered, normalized, fingerprint)
	if err != nil {
		return HeaderListPage{}, err
	}
	end := min(start+normalized.Limit, len(filtered))
	page := HeaderListPage{
		Headers: append([]memcontract.Header(nil), filtered[start:end]...),
		HasMore: end < len(filtered),
		Total:   len(filtered),
		Limit:   normalized.Limit,
	}
	if !page.HasMore {
		return page, nil
	}
	page.NextCursor, err = listcursor.Encode(
		headerListCursorVersion,
		headerListCursorKind,
		fingerprint,
		headerListCursorPositionFromHeader(page.Headers[len(page.Headers)-1]),
	)
	if err != nil {
		return HeaderListPage{}, fmt.Errorf("memory: encode header list cursor: %w", err)
	}
	return page, nil
}

func normalizeHeaderListQuery(query HeaderListQuery) (HeaderListQuery, error) {
	query.Scope = query.Scope.Normalize()
	query.WorkspaceID = strings.TrimSpace(query.WorkspaceID)
	query.AgentName = strings.TrimSpace(query.AgentName)
	query.AgentTier = query.AgentTier.Normalize()
	query.Type = query.Type.Normalize()
	query.Sort = strings.ToLower(strings.TrimSpace(query.Sort))
	query.Cursor = strings.TrimSpace(query.Cursor)
	if query.Sort == "" {
		query.Sort = HeaderListSortRecent
	}
	if query.Limit == 0 {
		query.Limit = DefaultHeaderListLimit
	}

	if query.Scope != "" {
		if err := query.Scope.Validate(); err != nil {
			return HeaderListQuery{}, fmt.Errorf("memory: invalid header list scope: %w", err)
		}
	}
	if query.AgentTier != "" {
		if err := query.AgentTier.Validate(); err != nil {
			return HeaderListQuery{}, fmt.Errorf("memory: invalid header list agent tier: %w", err)
		}
	}
	if query.Type != "" {
		if err := query.Type.Validate(); err != nil {
			return HeaderListQuery{}, fmt.Errorf("memory: invalid header list type: %w", err)
		}
	}
	if query.Sort != HeaderListSortRecent && query.Sort != HeaderListSortName {
		return HeaderListQuery{}, fmt.Errorf("memory: unsupported header list sort %q", query.Sort)
	}
	if query.Limit < 1 || query.Limit > MaxHeaderListLimit {
		return HeaderListQuery{}, fmt.Errorf(
			"memory: header list limit must be between 1 and %d: %d",
			MaxHeaderListLimit,
			query.Limit,
		)
	}
	return query, nil
}

func headerMatchesListQuery(header memcontract.Header, query HeaderListQuery) bool {
	if query.Scope != "" && header.Scope.Normalize() != query.Scope {
		return false
	}
	if query.AgentName != "" && strings.TrimSpace(header.AgentName) != query.AgentName {
		return false
	}
	if query.AgentTier != "" && header.AgentTier.Normalize() != query.AgentTier {
		return false
	}
	if query.Type != "" && header.Type.Normalize() != query.Type {
		return false
	}
	return query.IncludeSystem || !IsSystemManagedFilename(header.Filename)
}

// IsSystemManagedFilename reports whether a memory path belongs to the reserved system namespace.
func IsSystemManagedFilename(filename string) bool {
	return strings.HasPrefix(strings.TrimSpace(filename), "_system")
}

func headerListQueryFingerprint(query HeaderListQuery) (string, error) {
	fingerprint, err := listcursor.Fingerprint(headerListFingerprint{
		Scope:         query.Scope,
		WorkspaceID:   query.WorkspaceID,
		AgentName:     query.AgentName,
		AgentTier:     query.AgentTier,
		Type:          query.Type,
		IncludeSystem: query.IncludeSystem,
		Sort:          query.Sort,
	})
	if err != nil {
		return "", fmt.Errorf("memory: fingerprint header list query: %w", err)
	}
	return fingerprint, nil
}

func headerListPageStart(
	headers []memcontract.Header,
	query HeaderListQuery,
	fingerprint string,
) (int, error) {
	if query.Cursor == "" {
		return 0, nil
	}
	cursor, err := listcursor.Decode[headerListCursorPosition](
		query.Cursor,
		headerListCursorVersion,
		headerListCursorKind,
		fingerprint,
		listcursor.DefaultMaxEncodedSize,
	)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrHeaderListCursorInvalid, err)
	}
	if strings.TrimSpace(cursor.Filename) == "" {
		return 0, fmt.Errorf("%w: cursor filename is required", ErrHeaderListCursorInvalid)
	}
	return sort.Search(len(headers), func(index int) bool {
		return compareHeaderListKeys(
			headerListCursorPositionFromHeader(headers[index]),
			cursor,
			query.Sort,
		) > 0
	}), nil
}

func headerListCursorPositionFromHeader(header memcontract.Header) headerListCursorPosition {
	return headerListCursorPosition{
		ModTime:     header.ModTime.UTC(),
		Name:        strings.ToLower(strings.TrimSpace(header.Name)),
		Scope:       header.Scope.Normalize(),
		WorkspaceID: strings.TrimSpace(header.WorkspaceID),
		AgentName:   strings.TrimSpace(header.AgentName),
		AgentTier:   header.AgentTier.Normalize(),
		Filename:    strings.TrimSpace(header.Filename),
	}
}

func compareHeaderListKeys(left headerListCursorPosition, right headerListCursorPosition, sortKey string) int {
	if sortKey == HeaderListSortRecent && !left.ModTime.Equal(right.ModTime) {
		if left.ModTime.After(right.ModTime) {
			return -1
		}
		return 1
	}
	if sortKey == HeaderListSortName {
		if comparison := strings.Compare(left.Name, right.Name); comparison != 0 {
			return comparison
		}
	}
	if comparison := strings.Compare(string(left.Scope), string(right.Scope)); comparison != 0 {
		return comparison
	}
	if comparison := strings.Compare(left.WorkspaceID, right.WorkspaceID); comparison != 0 {
		return comparison
	}
	if comparison := strings.Compare(left.AgentName, right.AgentName); comparison != 0 {
		return comparison
	}
	if comparison := strings.Compare(string(left.AgentTier), string(right.AgentTier)); comparison != 0 {
		return comparison
	}
	return strings.Compare(left.Filename, right.Filename)
}
