package bridges

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	bridgecontract "github.com/compozy/agh/internal/bridges/contract"
)

const (
	defaultBridgeTargetFreshnessWindow = 6 * time.Hour
	defaultBridgeTargetListLimit       = 50
	maxBridgeTargetListLimit           = 200
)

// BridgeTargetType identifies the provider target family used by a bridge adapter.
type BridgeTargetType string

const (
	BridgeTargetTypeChannel BridgeTargetType = BridgeTargetType(bridgecontract.BridgeTargetTypeChannel)
	BridgeTargetTypeUser    BridgeTargetType = BridgeTargetType(bridgecontract.BridgeTargetTypeUser)
	BridgeTargetTypeRoom    BridgeTargetType = BridgeTargetType(bridgecontract.BridgeTargetTypeRoom)
	BridgeTargetTypeThread  BridgeTargetType = BridgeTargetType(bridgecontract.BridgeTargetTypeThread)
	BridgeTargetTypeGroup   BridgeTargetType = BridgeTargetType(bridgecontract.BridgeTargetTypeGroup)
)

// Normalize returns the canonical target type representation.
func (t BridgeTargetType) Normalize() BridgeTargetType {
	return BridgeTargetType(bridgecontract.BridgeTargetType(t).Normalize())
}

// Validate reports whether the target type belongs to the supported set.
func (t BridgeTargetType) Validate() error {
	return bridgecontract.BridgeTargetType(t).Validate()
}

// BridgeTargetSnapshotRequest asks a bridge-capable extension to enumerate targets for one instance.
type BridgeTargetSnapshotRequest struct {
	BridgeInstanceID string `json:"bridge_instance_id"`
}

// Validate reports whether the snapshot request identifies a bridge instance.
func (r BridgeTargetSnapshotRequest) Validate() error {
	return bridgeTargetSnapshotRequestToContract(r).Validate()
}

// BridgeTargetSnapshotResponse carries adapter-enumerated targets back to the daemon.
type BridgeTargetSnapshotResponse struct {
	Targets []BridgeTargetSnapshot `json:"targets"`
}

// BridgeTargetSnapshot is adapter-provided target data before daemon-owned normalization.
type BridgeTargetSnapshot struct {
	CanonicalRoute string           `json:"canonical_route"`
	DisplayName    string           `json:"display_name"`
	TargetType     BridgeTargetType `json:"target_type"`
	Qualifier      string           `json:"qualifier,omitempty"`
	Capabilities   []string         `json:"capabilities,omitempty"`
	LastSeenAt     time.Time        `json:"last_seen_at,omitzero"`
}

// Validate reports whether the adapter snapshot has a stable provider identity.
func (s BridgeTargetSnapshot) Validate() error {
	return bridgeTargetSnapshotToContract(s).Validate()
}

// BridgeTarget is the daemon-normalized persisted directory row.
type BridgeTarget struct {
	BridgeID       string           `json:"bridge_id"`
	CanonicalRoute string           `json:"canonical_route"`
	DisplayName    string           `json:"display_name"`
	Normalized     string           `json:"normalized"`
	TargetType     BridgeTargetType `json:"target_type"`
	Qualifier      string           `json:"qualifier,omitempty"`
	Capabilities   []string         `json:"capabilities"`
	UpdatedAt      time.Time        `json:"updated_at"`
	LastSeenAt     time.Time        `json:"last_seen_at,omitzero"`
}

// Validate reports whether the target row is ready for persistence.
func (t BridgeTarget) Validate() error {
	if err := requireField(strings.TrimSpace(t.BridgeID), "bridge target bridge id"); err != nil {
		return err
	}
	if err := requireField(strings.TrimSpace(t.CanonicalRoute), "bridge target canonical route"); err != nil {
		return err
	}
	if err := requireField(strings.TrimSpace(t.DisplayName), "bridge target display name"); err != nil {
		return err
	}
	if err := requireField(strings.TrimSpace(t.Normalized), "bridge target normalized name"); err != nil {
		return err
	}
	if err := t.TargetType.Validate(); err != nil {
		return err
	}
	if t.UpdatedAt.IsZero() {
		return errors.New("bridges: bridge target updated_at is required")
	}
	return nil
}

// BridgeTargetQuery filters the persisted target directory for one bridge instance.
type BridgeTargetQuery struct {
	BridgeID        string        `json:"bridge_id"`
	Query           string        `json:"query,omitempty"`
	Limit           int           `json:"limit,omitempty"`
	FreshnessWindow time.Duration `json:"-"`
	GeneratedAt     time.Time     `json:"-"`
}

// BridgeTargetPage is the persistence-layer page for target directory reads.
type BridgeTargetPage struct {
	Items                   []BridgeTarget `json:"items"`
	Total                   int            `json:"total"`
	LastSuccessfulRefreshAt time.Time      `json:"last_successful_refresh_at,omitzero"`
}

// BridgeTargetsResult is the service-level target directory response.
type BridgeTargetsResult struct {
	BridgeID                string         `json:"bridge_id"`
	Items                   []BridgeTarget `json:"items"`
	Total                   int            `json:"total"`
	CacheStale              bool           `json:"cache_stale"`
	GeneratedAt             time.Time      `json:"generated_at"`
	LastSuccessfulRefreshAt time.Time      `json:"last_successful_refresh_at,omitzero"`
}

// TargetDirectoryStore is the persistence surface for daemon-owned bridge target refreshes.
type TargetDirectoryStore interface {
	RefreshBridgeTargets(ctx context.Context, bridgeID string, targets []BridgeTarget, refreshedAt time.Time) error
	ListBridgeTargets(ctx context.Context, query BridgeTargetQuery) (BridgeTargetPage, error)
	GetBridgeTargetByCanonical(ctx context.Context, bridgeID string, canonicalRoute string) (BridgeTarget, error)
	FindBridgeTargetsByNormalized(ctx context.Context, bridgeID string, normalized string) ([]BridgeTarget, error)
	FindBridgeTargetsByQualifiedName(
		ctx context.Context,
		bridgeID string,
		qualifier string,
		normalized string,
	) ([]BridgeTarget, error)
	FindBridgeTargetsByPrefix(ctx context.Context, bridgeID string, normalizedPrefix string) ([]BridgeTarget, error)
}

// TargetSnapshotTransport calls bridge-capable extension adapters for target snapshots.
type TargetSnapshotTransport interface {
	BridgeTargetSnapshots(
		ctx context.Context,
		extensionName string,
		req BridgeTargetSnapshotRequest,
	) ([]BridgeTargetSnapshot, error)
}

// TargetDirectory is the daemon-owned bridge target directory behavior.
type TargetDirectory interface {
	RefreshBridgeTargets(
		ctx context.Context,
		bridgeID string,
		snapshots []BridgeTargetSnapshot,
	) ([]BridgeTarget, error)
	ListBridgeTargets(ctx context.Context, query BridgeTargetQuery) (BridgeTargetsResult, error)
	ResolveBridgeTarget(ctx context.Context, bridgeID string, query string) (ResolveBridgeTargetResult, error)
}

var _ TargetDirectory = (*Service)(nil)

// RefreshBridgeTargets validates adapter snapshots and persists them as one daemon-owned refresh.
func (s *Service) RefreshBridgeTargets(
	ctx context.Context,
	bridgeID string,
	snapshots []BridgeTargetSnapshot,
) ([]BridgeTarget, error) {
	if err := s.checkReady(ctx, "refresh bridge target directory"); err != nil {
		return nil, err
	}
	store, err := s.targetDirectoryStore()
	if err != nil {
		return nil, err
	}
	trimmedBridgeID := strings.TrimSpace(bridgeID)
	if err := requireField(trimmedBridgeID, "bridge target bridge id"); err != nil {
		return nil, err
	}
	if _, err := s.GetInstance(ctx, trimmedBridgeID); err != nil {
		return nil, err
	}
	refreshedAt := s.now().UTC()
	targets := make([]BridgeTarget, 0, len(snapshots))
	seen := make(map[string]struct{}, len(snapshots))
	for index, snapshot := range snapshots {
		target, normalizeErr := normalizeBridgeTargetSnapshot(trimmedBridgeID, snapshot, refreshedAt)
		if normalizeErr != nil {
			return nil, fmt.Errorf("bridges: normalize bridge target snapshot %d: %w", index, normalizeErr)
		}
		if _, ok := seen[target.CanonicalRoute]; ok {
			return nil, fmt.Errorf("bridges: duplicate bridge target canonical route %q", target.CanonicalRoute)
		}
		seen[target.CanonicalRoute] = struct{}{}
		targets = append(targets, target)
	}
	if err := store.RefreshBridgeTargets(ctx, trimmedBridgeID, targets, refreshedAt); err != nil {
		return nil, err
	}
	return cloneBridgeTargets(targets), nil
}

// ListBridgeTargets lists persisted bridge targets and reports cache freshness.
func (s *Service) ListBridgeTargets(ctx context.Context, query BridgeTargetQuery) (BridgeTargetsResult, error) {
	if err := s.checkReady(ctx, "list bridge targets"); err != nil {
		return BridgeTargetsResult{}, err
	}
	store, err := s.targetDirectoryStore()
	if err != nil {
		return BridgeTargetsResult{}, err
	}
	normalizedQuery := normalizeBridgeTargetQuery(query, s.now)
	if err := requireField(normalizedQuery.BridgeID, "bridge target bridge id"); err != nil {
		return BridgeTargetsResult{}, err
	}
	if _, err := s.GetInstance(ctx, normalizedQuery.BridgeID); err != nil {
		return BridgeTargetsResult{}, err
	}
	page, err := store.ListBridgeTargets(ctx, normalizedQuery)
	if err != nil {
		return BridgeTargetsResult{}, err
	}
	return BridgeTargetsResult{
		BridgeID:                normalizedQuery.BridgeID,
		Items:                   cloneBridgeTargets(page.Items),
		Total:                   page.Total,
		CacheStale:              bridgeTargetsCacheStale(page.LastSuccessfulRefreshAt, normalizedQuery),
		GeneratedAt:             normalizedQuery.GeneratedAt,
		LastSuccessfulRefreshAt: page.LastSuccessfulRefreshAt,
	}, nil
}

func (s *Service) targetDirectoryStore() (TargetDirectoryStore, error) {
	if s == nil || s.store == nil {
		return nil, ErrBridgeTargetDirectoryUnavailable
	}
	store, ok := s.store.(TargetDirectoryStore)
	if !ok || store == nil {
		return nil, ErrBridgeTargetDirectoryUnavailable
	}
	return store, nil
}

func normalizeBridgeTargetSnapshot(
	bridgeID string,
	snapshot BridgeTargetSnapshot,
	refreshedAt time.Time,
) (BridgeTarget, error) {
	if err := bridgeTargetSnapshotToContract(snapshot).Validate(); err != nil {
		return BridgeTarget{}, err
	}
	trimmedBridgeID := strings.TrimSpace(bridgeID)
	trimmedCanonical := strings.TrimSpace(snapshot.CanonicalRoute)
	trimmedDisplayName := strings.TrimSpace(snapshot.DisplayName)
	targetType := snapshot.TargetType.Normalize()
	updatedAt := refreshedAt.UTC()
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	lastSeenAt := snapshot.LastSeenAt.UTC()
	if lastSeenAt.IsZero() {
		lastSeenAt = updatedAt
	}
	target := BridgeTarget{
		BridgeID:       trimmedBridgeID,
		CanonicalRoute: trimmedCanonical,
		DisplayName:    trimmedDisplayName,
		Normalized:     NormalizeBridgeTargetName(trimmedDisplayName),
		TargetType:     targetType,
		Qualifier:      NormalizeBridgeTargetQualifier(snapshot.Qualifier),
		Capabilities:   normalizeBridgeTargetCapabilities(snapshot.Capabilities),
		UpdatedAt:      updatedAt,
		LastSeenAt:     lastSeenAt,
	}
	if trimmedBridgeID != "" {
		if err := target.Validate(); err != nil {
			return BridgeTarget{}, err
		}
	} else if target.Normalized == "" {
		return BridgeTarget{}, errors.New("bridges: bridge target normalized name is required")
	}
	return target, nil
}

func normalizeBridgeTargetQuery(query BridgeTargetQuery, now func() time.Time) BridgeTargetQuery {
	clock := now
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	normalized := query
	normalized.BridgeID = strings.TrimSpace(normalized.BridgeID)
	normalized.Query = strings.TrimSpace(normalized.Query)
	if normalized.Limit <= 0 {
		normalized.Limit = defaultBridgeTargetListLimit
	}
	if normalized.Limit > maxBridgeTargetListLimit {
		normalized.Limit = maxBridgeTargetListLimit
	}
	if normalized.FreshnessWindow <= 0 {
		normalized.FreshnessWindow = defaultBridgeTargetFreshnessWindow
	}
	if normalized.GeneratedAt.IsZero() {
		normalized.GeneratedAt = clock().UTC()
	} else {
		normalized.GeneratedAt = normalized.GeneratedAt.UTC()
	}
	return normalized
}

func bridgeTargetsCacheStale(lastSuccessfulRefreshAt time.Time, query BridgeTargetQuery) bool {
	if lastSuccessfulRefreshAt.IsZero() {
		return true
	}
	return query.GeneratedAt.Sub(lastSuccessfulRefreshAt.UTC()) > query.FreshnessWindow
}

// NormalizeBridgeTargetName canonicalizes display-name hints for lookup.
func NormalizeBridgeTargetName(value string) string {
	return bridgecontract.NormalizeBridgeTargetName(value)
}

// NormalizeBridgeTargetQualifier canonicalizes workspace/guild qualifiers for lookup.
func NormalizeBridgeTargetQualifier(value string) string {
	return bridgecontract.NormalizeBridgeTargetQualifier(value)
}

func normalizeBridgeTargetCapabilities(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	slices.Sort(normalized)
	return normalized
}

func cloneBridgeTargets(values []BridgeTarget) []BridgeTarget {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]BridgeTarget, 0, len(values))
	for _, value := range values {
		cloned = append(cloned, cloneBridgeTarget(value))
	}
	return cloned
}

func cloneBridgeTarget(value BridgeTarget) BridgeTarget {
	cloned := value
	if len(value.Capabilities) > 0 {
		cloned.Capabilities = append([]string(nil), value.Capabilities...)
	}
	return cloned
}
