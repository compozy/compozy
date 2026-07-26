package bridges

import (
	"fmt"
	"sort"
	"strings"

	"github.com/compozy/agh/internal/listcursor"
)

// BridgeCatalogRecord is the lean persisted projection used before page hydration.
type BridgeCatalogRecord struct {
	ID            string
	Scope         Scope
	WorkspaceID   string
	Platform      string
	ExtensionName string
	DisplayName   string
	Enabled       bool
	Status        BridgeStatus
}

// BridgeCatalogRecordItem combines lean persisted metadata with effective runtime status.
type BridgeCatalogRecordItem struct {
	Record          BridgeCatalogRecord
	EffectiveStatus BridgeStatus
}

// BridgeCatalogRecordPage is the stable page cut before full instance hydration.
type BridgeCatalogRecordPage struct {
	Items      []BridgeCatalogRecordItem
	Facets     BridgeCatalogFacets
	NextCursor string
	HasMore    bool
	Total      int
	Limit      int
}

// BridgeCatalogRecordFromInstance projects one full instance into catalog-only metadata.
func BridgeCatalogRecordFromInstance(instance BridgeInstance) BridgeCatalogRecord {
	return BridgeCatalogRecord{
		ID:            strings.TrimSpace(instance.ID),
		Scope:         instance.Scope.Normalize(),
		WorkspaceID:   strings.TrimSpace(instance.WorkspaceID),
		Platform:      strings.TrimSpace(instance.Platform),
		ExtensionName: strings.TrimSpace(instance.ExtensionName),
		DisplayName:   strings.TrimSpace(instance.DisplayName),
		Enabled:       instance.Enabled,
		Status:        instance.Status.Normalize(),
	}
}

// Normalized returns the canonical catalog record representation.
func (r BridgeCatalogRecord) Normalized() BridgeCatalogRecord {
	r.ID = strings.TrimSpace(r.ID)
	r.Scope = r.Scope.Normalize()
	r.WorkspaceID = strings.TrimSpace(r.WorkspaceID)
	r.Platform = strings.TrimSpace(r.Platform)
	r.ExtensionName = strings.TrimSpace(r.ExtensionName)
	r.DisplayName = strings.TrimSpace(r.DisplayName)
	r.Status = r.Status.Normalize()
	return r
}

// Validate reports whether the lean projection contains every catalog-owned field.
func (r BridgeCatalogRecord) Validate() error {
	normalized := r.Normalized()
	if err := requireField(normalized.ID, "bridge instance id"); err != nil {
		return err
	}
	if err := ValidateScopeWorkspaceID(normalized.Scope, normalized.WorkspaceID); err != nil {
		return err
	}
	if err := requireField(normalized.Platform, "bridge instance platform"); err != nil {
		return err
	}
	if err := requireField(normalized.ExtensionName, "bridge instance extension name"); err != nil {
		return err
	}
	if err := requireField(normalized.DisplayName, "bridge instance display name"); err != nil {
		return err
	}
	if err := normalized.Status.Validate(); err != nil {
		return err
	}
	return validateInstanceLifecycle(normalized.Enabled, normalized.Status)
}

// BuildBridgeCatalogRecordPage filters lean metadata before selecting full rows to hydrate.
func BuildBridgeCatalogRecordPage(
	records []BridgeCatalogRecord,
	effectiveStatuses map[string]BridgeStatus,
	query BridgeCatalogQuery,
) (BridgeCatalogRecordPage, error) {
	normalized, err := NormalizeBridgeCatalogQuery(query)
	if err != nil {
		return BridgeCatalogRecordPage{}, err
	}

	items := make([]BridgeCatalogRecordItem, 0, len(records))
	for _, rawRecord := range records {
		record := rawRecord.Normalized()
		item := BridgeCatalogRecordItem{
			Record:          record,
			EffectiveStatus: bridgeCatalogRecordEffectiveStatus(record, effectiveStatuses),
		}
		if bridgeCatalogRecordMatches(item, normalized) {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(left int, right int) bool {
		return compareBridgeCatalogPositions(
			bridgeCatalogRecordPosition(items[left]),
			bridgeCatalogRecordPosition(items[right]),
		) < 0
	})

	fingerprint, err := bridgeCatalogQueryFingerprint(normalized)
	if err != nil {
		return BridgeCatalogRecordPage{}, err
	}
	start, err := bridgeCatalogRecordPageStart(items, normalized.Cursor, fingerprint)
	if err != nil {
		return BridgeCatalogRecordPage{}, err
	}
	end := min(start+normalized.Limit, len(items))
	page := BridgeCatalogRecordPage{
		Items:   append([]BridgeCatalogRecordItem(nil), items[start:end]...),
		Facets:  bridgeCatalogRecordFacets(items),
		Total:   len(items),
		Limit:   normalized.Limit,
		HasMore: end < len(items),
	}
	if !page.HasMore {
		return page, nil
	}
	page.NextCursor, err = listcursor.Encode(
		bridgeCatalogCursorVersion,
		bridgeCatalogCursorKind,
		fingerprint,
		bridgeCatalogRecordPosition(page.Items[len(page.Items)-1]),
	)
	if err != nil {
		return BridgeCatalogRecordPage{}, fmt.Errorf("bridges: encode bridge catalog cursor: %w", err)
	}
	return page, nil
}

// HydrateBridgeCatalogPage binds one lean page to its bounded full-instance batch.
func HydrateBridgeCatalogPage(
	selection BridgeCatalogRecordPage,
	instances []BridgeInstance,
) (BridgeCatalogPage, error) {
	byID := make(map[string]BridgeInstance, len(instances))
	for _, instance := range instances {
		byID[strings.TrimSpace(instance.ID)] = instance
	}

	page := BridgeCatalogPage{
		Items:      make([]BridgeCatalogItem, 0, len(selection.Items)),
		Facets:     selection.Facets,
		NextCursor: selection.NextCursor,
		HasMore:    selection.HasMore,
		Total:      selection.Total,
		Limit:      selection.Limit,
	}
	for _, item := range selection.Items {
		instance, ok := byID[item.Record.ID]
		if !ok {
			return BridgeCatalogPage{}, fmt.Errorf(
				"%w: bridge instance %q",
				ErrBridgeCatalogHydrationIncomplete,
				item.Record.ID,
			)
		}
		page.Items = append(page.Items, BridgeCatalogItem{
			Instance:        instance,
			EffectiveStatus: item.EffectiveStatus,
		})
	}
	return page, nil
}

func bridgeCatalogRecordEffectiveStatus(
	record BridgeCatalogRecord,
	effectiveStatuses map[string]BridgeStatus,
) BridgeStatus {
	persisted := record.Status.Normalize()
	if !record.Enabled || persisted == BridgeStatusDisabled {
		return BridgeStatusDisabled
	}
	if persisted == BridgeStatusAuthRequired {
		return BridgeStatusAuthRequired
	}
	switch effectiveStatuses[record.ID].Normalize() {
	case BridgeStatusError:
		return BridgeStatusError
	case BridgeStatusDegraded:
		return BridgeStatusDegraded
	default:
		return persisted
	}
}

func bridgeCatalogRecordMatches(item BridgeCatalogRecordItem, query BridgeCatalogQuery) bool {
	record := item.Record
	switch query.Scope {
	case string(ScopeGlobal):
		if record.Scope != ScopeGlobal {
			return false
		}
	case string(ScopeWorkspace):
		if record.Scope != ScopeWorkspace || record.WorkspaceID != query.WorkspaceID {
			return false
		}
	case bridgeCatalogScopeAll:
		if query.WorkspaceID != "" && record.Scope != ScopeGlobal &&
			(record.Scope != ScopeWorkspace || record.WorkspaceID != query.WorkspaceID) {
			return false
		}
	}
	platform := strings.ToLower(record.Platform)
	if query.Platform != "" && platform != query.Platform {
		return false
	}
	if query.Status != "" && item.EffectiveStatus != query.Status {
		return false
	}
	if query.Search == "" {
		return true
	}
	for _, field := range []string{
		record.DisplayName,
		record.Platform,
		record.ExtensionName,
		string(item.EffectiveStatus),
	} {
		if strings.Contains(strings.ToLower(field), query.Search) {
			return true
		}
	}
	return false
}

func bridgeCatalogRecordFacets(items []BridgeCatalogRecordItem) BridgeCatalogFacets {
	facets := BridgeCatalogFacets{
		Platforms: make(map[string]int),
		Statuses:  make(map[BridgeStatus]int),
	}
	for _, item := range items {
		platform := strings.ToLower(item.Record.Platform)
		if platform != "" {
			facets.Platforms[platform]++
		}
		facets.Statuses[item.EffectiveStatus.Normalize()]++
	}
	return facets
}

func bridgeCatalogRecordPageStart(
	items []BridgeCatalogRecordItem,
	rawCursor string,
	fingerprint string,
) (int, error) {
	if rawCursor == "" {
		return 0, nil
	}
	position, err := listcursor.Decode[bridgeCatalogCursorPosition](
		rawCursor,
		bridgeCatalogCursorVersion,
		bridgeCatalogCursorKind,
		fingerprint,
		listcursor.DefaultMaxEncodedSize,
	)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrBridgeCatalogCursorInvalid, err)
	}
	if strings.TrimSpace(position.ID) == "" {
		return 0, fmt.Errorf("%w: cursor id is required", ErrBridgeCatalogCursorInvalid)
	}
	if strings.TrimSpace(position.DisplayName) == "" {
		return 0, fmt.Errorf("%w: cursor display name is required", ErrBridgeCatalogCursorInvalid)
	}
	if err := position.Scope.Validate(); err != nil {
		return 0, fmt.Errorf("%w: cursor scope: %v", ErrBridgeCatalogCursorInvalid, err)
	}
	position.Scope = position.Scope.Normalize()
	position.DisplayName = strings.ToLower(strings.TrimSpace(position.DisplayName))
	position.ID = strings.TrimSpace(position.ID)
	return sort.Search(len(items), func(index int) bool {
		return compareBridgeCatalogPositions(bridgeCatalogRecordPosition(items[index]), position) > 0
	}), nil
}

func bridgeCatalogRecordPosition(item BridgeCatalogRecordItem) bridgeCatalogCursorPosition {
	return bridgeCatalogCursorPosition{
		Scope:       item.Record.Scope,
		DisplayName: strings.ToLower(item.Record.DisplayName),
		ID:          item.Record.ID,
	}
}

func compareBridgeCatalogPositions(left bridgeCatalogCursorPosition, right bridgeCatalogCursorPosition) int {
	if comparison := bridgeCatalogScopeRank(left.Scope) - bridgeCatalogScopeRank(right.Scope); comparison != 0 {
		return comparison
	}
	if comparison := strings.Compare(left.DisplayName, right.DisplayName); comparison != 0 {
		return comparison
	}
	return strings.Compare(left.ID, right.ID)
}

func bridgeCatalogScopeRank(scope Scope) int {
	switch scope.Normalize() {
	case ScopeGlobal:
		return 0
	case ScopeWorkspace:
		return 1
	default:
		return 2
	}
}
