package loop

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/agh/internal/listcursor"
	"github.com/compozy/agh/internal/resources"
)

const (
	CatalogKindReadOnly  CatalogKind = "read_only"
	CatalogKindWorkspace CatalogKind = "workspace"

	CatalogSortName     = "name"
	DefaultCatalogLimit = 50
	MaxCatalogLimit     = 200

	catalogCursorVersion = 1
	catalogCursorKind    = "loops"
)

var (
	ErrCatalogQueryInvalid  = errors.New("loop: catalog query is invalid")
	ErrCatalogCursorInvalid = errors.New("loop: catalog cursor is invalid")
	ErrCatalogUnavailable   = errors.New("loop: catalog is unavailable")
)

type CatalogKind string

type CatalogQuery struct {
	WorkspaceID string
	Search      string
	Kind        CatalogKind
	Category    string
	Status      Status
	Sort        string
	Cursor      string
	Limit       int
}

type CatalogRunAggregate struct {
	Runs      int
	Succeeded int
	Failed    int
}

type CatalogRunSummary struct {
	LoopName     string
	LastRun      *CatalogRunHead
	Aggregate30d CatalogRunAggregate
}

type CatalogItem struct {
	Record  resources.Record[ResourceSpec]
	Summary CatalogRunSummary
}

type CatalogFacets struct {
	Kinds      map[CatalogKind]int
	Categories map[string]int
	Statuses   map[Status]int
}

type CatalogPage struct {
	Items      []CatalogItem
	Facets     CatalogFacets
	NextCursor string
	HasMore    bool
	Total      int
	Limit      int
}

type catalogFingerprint struct {
	WorkspaceID string      `json:"workspace_id"`
	Search      string      `json:"q"`
	Kind        CatalogKind `json:"kind"`
	Category    string      `json:"category"`
	Status      Status      `json:"status"`
	Sort        string      `json:"sort"`
}

type catalogPosition struct {
	Kind CatalogKind `json:"kind"`
	Name string      `json:"name"`
	ID   string      `json:"id"`
}

type catalogFacet int

const (
	catalogFacetNone catalogFacet = iota
	catalogFacetKind
	catalogFacetCategory
	catalogFacetStatus
)

func NormalizeCatalogQuery(query CatalogQuery) (CatalogQuery, error) {
	query.WorkspaceID = strings.TrimSpace(query.WorkspaceID)
	query.Search = strings.ToLower(strings.TrimSpace(query.Search))
	query.Kind = CatalogKind(strings.ToLower(strings.TrimSpace(string(query.Kind))))
	query.Category = strings.TrimSpace(query.Category)
	query.Status = Status(strings.TrimSpace(string(query.Status)))
	query.Sort = strings.ToLower(strings.TrimSpace(query.Sort))
	query.Cursor = strings.TrimSpace(query.Cursor)
	if query.Sort == "" {
		query.Sort = CatalogSortName
	}
	if query.Limit == 0 {
		query.Limit = DefaultCatalogLimit
	}
	if query.WorkspaceID == "" {
		return CatalogQuery{}, fmt.Errorf("%w: workspace_id is required", ErrCatalogQueryInvalid)
	}
	switch query.Kind {
	case "", CatalogKindReadOnly, CatalogKindWorkspace:
	default:
		return CatalogQuery{}, fmt.Errorf("%w: unsupported kind %q", ErrCatalogQueryInvalid, query.Kind)
	}
	if query.Status != "" && !query.Status.Valid() {
		return CatalogQuery{}, fmt.Errorf("%w: unsupported status %q", ErrCatalogQueryInvalid, query.Status)
	}
	if query.Sort != CatalogSortName {
		return CatalogQuery{}, fmt.Errorf("%w: unsupported sort %q", ErrCatalogQueryInvalid, query.Sort)
	}
	if query.Limit < 1 || query.Limit > MaxCatalogLimit {
		return CatalogQuery{}, fmt.Errorf(
			"%w: limit must be between 1 and %d: %d",
			ErrCatalogQueryInvalid,
			MaxCatalogLimit,
			query.Limit,
		)
	}
	return query, nil
}

func BuildCatalogPage(
	records []resources.Record[ResourceSpec],
	summaries map[string]CatalogRunSummary,
	query CatalogQuery,
) (CatalogPage, error) {
	normalized, err := NormalizeCatalogQuery(query)
	if err != nil {
		return CatalogPage{}, err
	}
	items := make([]CatalogItem, 0, len(records))
	for _, record := range records {
		name := strings.TrimSpace(record.Spec.Name)
		summary := summaries[name]
		if summary.LoopName == "" {
			summary.LoopName = name
		}
		items = append(items, CatalogItem{Record: record, Summary: summary})
	}
	slices.SortFunc(items, func(left CatalogItem, right CatalogItem) int {
		return compareCatalogPositions(catalogItemPosition(left), catalogItemPosition(right))
	})

	filtered := make([]CatalogItem, 0, len(items))
	for _, item := range items {
		if catalogItemMatches(item, normalized, catalogFacetNone) {
			filtered = append(filtered, item)
		}
	}
	fingerprint, err := catalogQueryFingerprint(normalized)
	if err != nil {
		return CatalogPage{}, err
	}
	start, err := catalogPageStart(filtered, normalized.Cursor, fingerprint)
	if err != nil {
		return CatalogPage{}, err
	}
	end := min(start+normalized.Limit, len(filtered))
	page := CatalogPage{
		Items:   append([]CatalogItem(nil), filtered[start:end]...),
		Facets:  catalogSelfFilteredFacets(items, normalized),
		HasMore: end < len(filtered),
		Total:   len(filtered),
		Limit:   normalized.Limit,
	}
	if !page.HasMore {
		return page, nil
	}
	page.NextCursor, err = listcursor.Encode(
		catalogCursorVersion,
		catalogCursorKind,
		fingerprint,
		catalogItemPosition(page.Items[len(page.Items)-1]),
	)
	if err != nil {
		return CatalogPage{}, fmt.Errorf("loop: encode catalog cursor: %w", err)
	}
	return page, nil
}

func catalogItemMatches(item CatalogItem, query CatalogQuery, omitted catalogFacet) bool {
	if query.Search != "" {
		name := strings.ToLower(strings.TrimSpace(item.Record.Spec.Name))
		goal := strings.ToLower(strings.TrimSpace(item.Record.Spec.ContractGoal))
		if !strings.Contains(name, query.Search) && !strings.Contains(goal, query.Search) {
			return false
		}
	}
	if omitted != catalogFacetKind && query.Kind != "" && catalogItemKind(item) != query.Kind {
		return false
	}
	category := strings.TrimSpace(item.Record.Spec.Catalog.Category)
	if omitted != catalogFacetCategory && query.Category != "" && category != query.Category {
		return false
	}
	if omitted != catalogFacetStatus && query.Status != "" {
		if item.Summary.LastRun == nil || item.Summary.LastRun.Status != query.Status {
			return false
		}
	}
	return true
}

func catalogSelfFilteredFacets(items []CatalogItem, query CatalogQuery) CatalogFacets {
	facets := CatalogFacets{
		Kinds:      make(map[CatalogKind]int),
		Categories: make(map[string]int),
		Statuses:   make(map[Status]int),
	}
	for _, item := range items {
		if catalogItemMatches(item, query, catalogFacetKind) {
			facets.Kinds[catalogItemKind(item)]++
		}
		if catalogItemMatches(item, query, catalogFacetCategory) {
			if category := strings.TrimSpace(item.Record.Spec.Catalog.Category); category != "" {
				facets.Categories[category]++
			}
		}
		if catalogItemMatches(item, query, catalogFacetStatus) && item.Summary.LastRun != nil {
			facets.Statuses[item.Summary.LastRun.Status]++
		}
	}
	return facets
}

func catalogItemKind(item CatalogItem) CatalogKind {
	if item.Record.Spec.Source.Normalize() == SourceWorkspace {
		return CatalogKindWorkspace
	}
	return CatalogKindReadOnly
}

func catalogQueryFingerprint(query CatalogQuery) (string, error) {
	fingerprint, err := listcursor.Fingerprint(catalogFingerprint{
		WorkspaceID: query.WorkspaceID,
		Search:      query.Search,
		Kind:        query.Kind,
		Category:    query.Category,
		Status:      query.Status,
		Sort:        query.Sort,
	})
	if err != nil {
		return "", fmt.Errorf("loop: fingerprint catalog query: %w", err)
	}
	return fingerprint, nil
}

func catalogPageStart(items []CatalogItem, rawCursor string, fingerprint string) (int, error) {
	if rawCursor == "" {
		return 0, nil
	}
	position, err := listcursor.Decode[catalogPosition](
		rawCursor,
		catalogCursorVersion,
		catalogCursorKind,
		fingerprint,
		listcursor.DefaultMaxEncodedSize,
	)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrCatalogCursorInvalid, err)
	}
	position.Kind = CatalogKind(strings.ToLower(strings.TrimSpace(string(position.Kind))))
	position.Name = strings.ToLower(strings.TrimSpace(position.Name))
	position.ID = strings.TrimSpace(position.ID)
	if (position.Kind != CatalogKindReadOnly && position.Kind != CatalogKindWorkspace) ||
		position.Name == "" || position.ID == "" {
		return 0, fmt.Errorf("%w: cursor position is incomplete", ErrCatalogCursorInvalid)
	}
	return sortSearchCatalog(items, position), nil
}

func sortSearchCatalog(items []CatalogItem, position catalogPosition) int {
	low := 0
	high := len(items)
	for low < high {
		middle := int(uint(low+high) >> 1)
		if compareCatalogPositions(catalogItemPosition(items[middle]), position) <= 0 {
			low = middle + 1
		} else {
			high = middle
		}
	}
	return low
}

func catalogItemPosition(item CatalogItem) catalogPosition {
	return catalogPosition{
		Kind: catalogItemKind(item),
		Name: strings.ToLower(strings.TrimSpace(item.Record.Spec.Name)),
		ID:   strings.TrimSpace(item.Record.ID),
	}
}

func compareCatalogPositions(left catalogPosition, right catalogPosition) int {
	if comparison := catalogKindRank(left.Kind) - catalogKindRank(right.Kind); comparison != 0 {
		return comparison
	}
	if comparison := strings.Compare(left.Name, right.Name); comparison != 0 {
		return comparison
	}
	return strings.Compare(left.ID, right.ID)
}

func catalogKindRank(kind CatalogKind) int {
	if kind == CatalogKindReadOnly {
		return 0
	}
	return 1
}
