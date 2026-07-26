package bridges

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/listcursor"
)

const (
	// BridgeCatalogSortName preserves the operator-facing scope and display-name order.
	BridgeCatalogSortName = "name"

	// DefaultBridgeCatalogLimit bounds public bridge catalog reads by default.
	DefaultBridgeCatalogLimit = 50
	// MaxBridgeCatalogLimit caps one public bridge catalog page.
	MaxBridgeCatalogLimit = 200

	bridgeCatalogCursorVersion = 1
	bridgeCatalogCursorKind    = "bridge_instances"
	bridgeCatalogScopeAll      = "all"
)

var (
	// ErrBridgeCatalogQueryInvalid reports an invalid public bridge catalog query.
	ErrBridgeCatalogQueryInvalid = errors.New("bridges: bridge catalog query is invalid")
	// ErrBridgeCatalogCursorInvalid reports a malformed or query-mismatched catalog cursor.
	ErrBridgeCatalogCursorInvalid = errors.New("bridges: bridge catalog cursor is invalid")
	// ErrBridgeCatalogHydrationIncomplete reports a selected row lost before bounded hydration.
	ErrBridgeCatalogHydrationIncomplete = errors.New("bridges: bridge catalog hydration is incomplete")
)

// BridgeCatalogQuery describes one bounded public bridge catalog page.
type BridgeCatalogQuery struct {
	Scope       string
	WorkspaceID string
	Search      string
	Platform    string
	Status      BridgeStatus
	Sort        string
	Cursor      string
	Limit       int
}

// BridgeCatalogItem combines persisted bridge configuration with its effective runtime status.
type BridgeCatalogItem struct {
	Instance        BridgeInstance
	EffectiveStatus BridgeStatus
}

// BridgeCatalogFacets reports exact counts for the complete filtered authority.
type BridgeCatalogFacets struct {
	Platforms map[string]int
	Statuses  map[BridgeStatus]int
}

// BridgeCatalogPage is one filtered, counted, stable bridge catalog page.
type BridgeCatalogPage struct {
	Items      []BridgeCatalogItem
	Facets     BridgeCatalogFacets
	NextCursor string
	HasMore    bool
	Total      int
	Limit      int
}

type bridgeCatalogFingerprint struct {
	Scope       string       `json:"scope"`
	WorkspaceID string       `json:"workspace_id"`
	Search      string       `json:"q"`
	Platform    string       `json:"platform"`
	Status      BridgeStatus `json:"status"`
	Sort        string       `json:"sort"`
}

type bridgeCatalogCursorPosition struct {
	Scope       Scope  `json:"scope"`
	DisplayName string `json:"display_name"`
	ID          string `json:"id"`
}

// BuildBridgeCatalogPage filters and orders the visible authority before applying a stable cut.
func BuildBridgeCatalogPage(
	instances []BridgeInstance,
	effectiveStatuses map[string]BridgeStatus,
	query BridgeCatalogQuery,
) (BridgeCatalogPage, error) {
	records := make([]BridgeCatalogRecord, 0, len(instances))
	for _, instance := range instances {
		records = append(records, BridgeCatalogRecordFromInstance(instance))
	}
	selection, err := BuildBridgeCatalogRecordPage(records, effectiveStatuses, query)
	if err != nil {
		return BridgeCatalogPage{}, err
	}
	return HydrateBridgeCatalogPage(selection, instances)
}

// NormalizeBridgeCatalogQuery validates and canonicalizes one public catalog query.
func NormalizeBridgeCatalogQuery(query BridgeCatalogQuery) (BridgeCatalogQuery, error) {
	query.Scope = strings.ToLower(strings.TrimSpace(query.Scope))
	query.WorkspaceID = strings.TrimSpace(query.WorkspaceID)
	query.Search = strings.ToLower(strings.TrimSpace(query.Search))
	query.Platform = strings.ToLower(strings.TrimSpace(query.Platform))
	query.Status = query.Status.Normalize()
	query.Sort = strings.ToLower(strings.TrimSpace(query.Sort))
	query.Cursor = strings.TrimSpace(query.Cursor)
	if query.Scope == "" {
		query.Scope = bridgeCatalogScopeAll
	}
	if query.Sort == "" {
		query.Sort = BridgeCatalogSortName
	}
	if query.Limit == 0 {
		query.Limit = DefaultBridgeCatalogLimit
	}

	switch query.Scope {
	case bridgeCatalogScopeAll, string(ScopeGlobal), string(ScopeWorkspace):
	default:
		return BridgeCatalogQuery{}, fmt.Errorf(
			"%w: unsupported scope %q",
			ErrBridgeCatalogQueryInvalid,
			query.Scope,
		)
	}
	if query.Scope == string(ScopeGlobal) && query.WorkspaceID != "" {
		return BridgeCatalogQuery{}, fmt.Errorf(
			"%w: global scope cannot include workspace id",
			ErrBridgeCatalogQueryInvalid,
		)
	}
	if query.Scope == string(ScopeWorkspace) && query.WorkspaceID == "" {
		return BridgeCatalogQuery{}, fmt.Errorf(
			"%w: workspace scope requires workspace id",
			ErrBridgeCatalogQueryInvalid,
		)
	}
	if query.Status != "" {
		if err := query.Status.Validate(); err != nil {
			return BridgeCatalogQuery{}, fmt.Errorf("%w: %w", ErrBridgeCatalogQueryInvalid, err)
		}
	}
	if query.Sort != BridgeCatalogSortName {
		return BridgeCatalogQuery{}, fmt.Errorf(
			"%w: unsupported sort %q",
			ErrBridgeCatalogQueryInvalid,
			query.Sort,
		)
	}
	if query.Limit < 1 || query.Limit > MaxBridgeCatalogLimit {
		return BridgeCatalogQuery{}, fmt.Errorf(
			"%w: limit must be between 1 and %d: %d",
			ErrBridgeCatalogQueryInvalid,
			MaxBridgeCatalogLimit,
			query.Limit,
		)
	}
	return query, nil
}

func bridgeCatalogQueryFingerprint(query BridgeCatalogQuery) (string, error) {
	fingerprint, err := listcursor.Fingerprint(bridgeCatalogFingerprint{
		Scope:       query.Scope,
		WorkspaceID: query.WorkspaceID,
		Search:      query.Search,
		Platform:    query.Platform,
		Status:      query.Status,
		Sort:        query.Sort,
	})
	if err != nil {
		return "", fmt.Errorf("bridges: fingerprint bridge catalog query: %w", err)
	}
	return fingerprint, nil
}
