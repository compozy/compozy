package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

const (
	globalDBBridgeTargetDefaultLimit       = 50
	globalDBBridgeTargetMaxLimit           = 200
	globalDBBridgeTargetResolverMaxMatches = 201
)

var _ bridges.TargetDirectoryStore = (*BridgeRepo)(nil)

// RefreshBridgeTargets transactionally persists one daemon-owned target-directory snapshot.
func (g *BridgeRepo) RefreshBridgeTargets(
	ctx context.Context,
	bridgeID string,
	targets []bridges.BridgeTarget,
	refreshedAt time.Time,
) error {
	if err := g.checkReady(ctx, "refresh bridge targets"); err != nil {
		return err
	}
	trimmedBridgeID := strings.TrimSpace(bridgeID)
	if trimmedBridgeID == "" {
		return errors.New("store: bridge target directory bridge id is required")
	}
	if refreshedAt.IsZero() {
		refreshedAt = g.now()
	}
	refreshedAt = refreshedAt.UTC()

	return g.withImmediateTransaction(ctx, "bridge target refresh", func(exec globalSQLExecutor) error {
		queries := sqlcgen.New(exec)
		seen := make(map[string]struct{}, len(targets))
		for index, target := range targets {
			normalized, normalizeErr := normalizeGlobalDBBridgeTarget(trimmedBridgeID, target, refreshedAt)
			if normalizeErr != nil {
				return fmt.Errorf("store: normalize bridge target %d: %w", index, normalizeErr)
			}
			if _, ok := seen[normalized.CanonicalRoute]; ok {
				return fmt.Errorf("store: duplicate bridge target canonical route %q", normalized.CanonicalRoute)
			}
			seen[normalized.CanonicalRoute] = struct{}{}
			if err := upsertBridgeTarget(ctx, queries, normalized); err != nil {
				return err
			}
		}

		if err := queries.UpsertBridgeTargetRefresh(ctx, sqlcgen.UpsertBridgeTargetRefreshParams{
			BridgeID: trimmedBridgeID, LastSuccessfulRefreshAt: store.FormatTimestamp(refreshedAt),
		}); err != nil {
			return fmt.Errorf(
				"store: update bridge target refresh state for %q: %w",
				trimmedBridgeID,
				mapBridgeChildConstraintError(err),
			)
		}
		return nil
	})
}

func upsertBridgeTarget(
	ctx context.Context,
	queries *sqlcgen.Queries,
	target bridges.BridgeTarget,
) error {
	capabilitiesJSON, err := encodeBridgeTargetCapabilities(target.Capabilities)
	if err != nil {
		return err
	}
	if err := queries.UpsertBridgeTarget(ctx, sqlcgen.UpsertBridgeTargetParams{
		BridgeID: target.BridgeID, CanonicalRoute: target.CanonicalRoute,
		DisplayName: target.DisplayName, Normalized: target.Normalized,
		TargetType: string(target.TargetType), Qualifier: target.Qualifier,
		Capabilities: capabilitiesJSON, UpdatedAt: store.FormatTimestamp(target.UpdatedAt),
		LastSeenAt: nullableBridgeTimestamp(target.LastSeenAt),
	}); err != nil {
		return fmt.Errorf(
			"store: refresh bridge target %q/%q: %w",
			target.BridgeID,
			target.CanonicalRoute,
			mapBridgeChildConstraintError(err),
		)
	}
	return nil
}

// ListBridgeTargets returns target-directory rows plus bridge-level refresh freshness.
func (g *BridgeRepo) ListBridgeTargets(
	ctx context.Context,
	query bridges.BridgeTargetQuery,
) (bridges.BridgeTargetPage, error) {
	if err := g.checkReady(ctx, "list bridge targets"); err != nil {
		return bridges.BridgeTargetPage{}, err
	}
	normalized := normalizeGlobalDBBridgeTargetQuery(query)
	if normalized.BridgeID == "" {
		return bridges.BridgeTargetPage{}, errors.New("store: bridge target directory bridge id is required")
	}

	lastRefresh, err := g.getBridgeTargetLastRefresh(ctx, normalized.BridgeID)
	if err != nil {
		return bridges.BridgeTargetPage{}, err
	}
	targets, total, err := g.listBridgeTargets(ctx, normalized)
	if err != nil {
		return bridges.BridgeTargetPage{}, err
	}
	return bridges.BridgeTargetPage{
		Items:                   targets,
		Total:                   total,
		LastSuccessfulRefreshAt: lastRefresh,
	}, nil
}

func (g *BridgeRepo) listBridgeTargets(
	ctx context.Context,
	query bridges.BridgeTargetQuery,
) ([]bridges.BridgeTarget, int, error) {
	if strings.TrimSpace(query.Query) == "" {
		total, err := g.countBridgeTargets(ctx, `bridge_id = ?`, query.BridgeID)
		if err != nil {
			return nil, 0, err
		}
		targets, err := g.queryBridgeTargets(
			ctx,
			"list bridge targets",
			`bridge_id = ?`,
			query.BridgeID,
			query.Limit,
		)
		return targets, total, err
	}

	lookup := bridges.NormalizeBridgeTargetName(query.Query)
	like := "%" + escapeBridgeTargetLike(lookup) + "%"
	canonicalLike := "%" + escapeBridgeTargetLike(strings.ToLower(strings.TrimSpace(query.Query))) + "%"
	where := `bridge_id = ?
		AND (normalized LIKE ? ESCAPE '\'
			OR qualifier LIKE ? ESCAPE '\'
			OR lower(display_name) LIKE ? ESCAPE '\'
			OR lower(canonical_route) LIKE ? ESCAPE '\')`
	args := []any{query.BridgeID, like, like, like, canonicalLike}
	total, err := g.countBridgeTargets(ctx, where, args...)
	if err != nil {
		return nil, 0, err
	}
	targets, err := g.queryBridgeTargets(
		ctx,
		"list bridge targets",
		where,
		append(args, query.Limit)...,
	)
	return targets, total, err
}

// GetBridgeTargetByCanonical returns one target by immutable provider-derived identity.
func (g *BridgeRepo) GetBridgeTargetByCanonical(
	ctx context.Context,
	bridgeID string,
	canonicalRoute string,
) (bridges.BridgeTarget, error) {
	if err := g.checkReady(ctx, "get bridge target by canonical"); err != nil {
		return bridges.BridgeTarget{}, err
	}
	trimmedBridgeID := strings.TrimSpace(bridgeID)
	trimmedCanonical := strings.TrimSpace(canonicalRoute)
	if trimmedBridgeID == "" {
		return bridges.BridgeTarget{}, errors.New("store: bridge target bridge id is required")
	}
	if trimmedCanonical == "" {
		return bridges.BridgeTarget{}, errors.New("store: bridge target canonical route is required")
	}
	row, err := g.queries.GetBridgeTargetByCanonical(ctx, sqlcgen.GetBridgeTargetByCanonicalParams{
		BridgeID: trimmedBridgeID, CanonicalRoute: trimmedCanonical,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return bridges.BridgeTarget{}, fmt.Errorf(
				"store: bridge target %q: %w",
				trimmedCanonical,
				bridges.ErrBridgeTargetUnknown,
			)
		}
		return bridges.BridgeTarget{}, fmt.Errorf("store: get bridge target %q: %w", trimmedCanonical, err)
	}
	target, err := bridgeTargetFromGenerated(row)
	if err != nil {
		return bridges.BridgeTarget{}, fmt.Errorf("store: map bridge target %q: %w", trimmedCanonical, err)
	}
	return target, nil
}

// FindBridgeTargetsByNormalized returns exact normalized display-name matches.
func (g *BridgeRepo) FindBridgeTargetsByNormalized(
	ctx context.Context,
	bridgeID string,
	normalized string,
) ([]bridges.BridgeTarget, error) {
	return g.queryBridgeTargets(
		ctx,
		"find bridge targets by normalized name",
		`bridge_id = ? AND normalized = ?`,
		strings.TrimSpace(bridgeID),
		bridges.NormalizeBridgeTargetName(normalized),
		globalDBBridgeTargetResolverMaxMatches,
	)
}

// FindBridgeTargetsByQualifiedName returns exact qualifier plus normalized-name matches.
func (g *BridgeRepo) FindBridgeTargetsByQualifiedName(
	ctx context.Context,
	bridgeID string,
	qualifier string,
	normalized string,
) ([]bridges.BridgeTarget, error) {
	return g.queryBridgeTargets(
		ctx,
		"find bridge targets by qualified name",
		`bridge_id = ? AND qualifier = ? AND normalized = ?`,
		strings.TrimSpace(bridgeID),
		bridges.NormalizeBridgeTargetQualifier(qualifier),
		bridges.NormalizeBridgeTargetName(normalized),
		globalDBBridgeTargetResolverMaxMatches,
	)
}

// FindBridgeTargetsByPrefix returns normalized display-name prefix matches.
func (g *BridgeRepo) FindBridgeTargetsByPrefix(
	ctx context.Context,
	bridgeID string,
	normalizedPrefix string,
) ([]bridges.BridgeTarget, error) {
	prefix := bridges.NormalizeBridgeTargetName(normalizedPrefix)
	if prefix == "" {
		return nil, nil
	}
	return g.queryBridgeTargets(
		ctx,
		"find bridge targets by prefix",
		`bridge_id = ? AND normalized LIKE ? ESCAPE '\'`,
		strings.TrimSpace(bridgeID),
		escapeBridgeTargetLike(prefix)+"%",
		globalDBBridgeTargetResolverMaxMatches,
	)
}

func (g *BridgeRepo) countBridgeTargets(ctx context.Context, where string, args ...any) (int, error) {
	// dynamic-sql: the package selects one of a fixed set of target-search predicates at runtime.
	// #nosec G202 -- where fragments are package-local constants; user input stays parameterized.
	row := g.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bridge_target_directory WHERE `+where, args...)
	var total int
	if err := row.Scan(&total); err != nil {
		return 0, fmt.Errorf("store: count bridge targets: %w", err)
	}
	return total, nil
}

func (g *BridgeRepo) queryBridgeTargets(
	ctx context.Context,
	action string,
	where string,
	args ...any,
) (targets []bridges.BridgeTarget, err error) {
	if err := g.checkReady(ctx, action); err != nil {
		return nil, err
	}
	if len(args) == 0 || strings.TrimSpace(fmt.Sprint(args[0])) == "" {
		return nil, errors.New("store: bridge target bridge id is required")
	}
	// dynamic-sql: the package selects one of a fixed set of target-search predicates at runtime.
	// #nosec G202 -- where fragments are package-local constants; user input stays parameterized.
	rows, err := g.db.QueryContext(
		ctx,
		`SELECT bridge_id, canonical_route, display_name, normalized, target_type,
			qualifier, capabilities, updated_at, last_seen_at
		 FROM bridge_target_directory
		 WHERE `+where+`
		 ORDER BY normalized ASC, qualifier ASC, canonical_route ASC
		 LIMIT ?`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("store: %s: %w", action, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close bridge target rows: %w", closeErr))
		}
	}()

	targets = make([]bridges.BridgeTarget, 0)
	for rows.Next() {
		target, scanErr := scanBridgeTarget(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		targets = append(targets, target)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate bridge targets: %w", err)
	}
	return targets, nil
}

func (g *BridgeRepo) getBridgeTargetLastRefresh(ctx context.Context, bridgeID string) (time.Time, error) {
	row, err := g.queries.GetBridgeTargetRefresh(ctx, strings.TrimSpace(bridgeID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return time.Time{}, bridges.ErrBridgeInstanceNotFound
		}
		return time.Time{}, fmt.Errorf("store: get bridge target refresh state for %q: %w", bridgeID, err)
	}
	raw, err := bridgeGeneratedString(row.LastSuccessfulRefreshAt, "bridge target refresh timestamp")
	if err != nil {
		return time.Time{}, err
	}
	if strings.TrimSpace(row.ID) == "" || strings.TrimSpace(raw) == "" {
		return time.Time{}, nil
	}
	parsed, err := store.ParseTimestamp(raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("store: parse bridge target refresh timestamp for %q: %w", bridgeID, err)
	}
	return parsed.UTC(), nil
}

func normalizeGlobalDBBridgeTarget(
	bridgeID string,
	target bridges.BridgeTarget,
	refreshedAt time.Time,
) (bridges.BridgeTarget, error) {
	normalized := target
	normalized.BridgeID = strings.TrimSpace(bridgeID)
	normalized.CanonicalRoute = strings.TrimSpace(normalized.CanonicalRoute)
	normalized.DisplayName = strings.TrimSpace(normalized.DisplayName)
	if strings.TrimSpace(normalized.Normalized) == "" {
		normalized.Normalized = bridges.NormalizeBridgeTargetName(normalized.DisplayName)
	} else {
		normalized.Normalized = bridges.NormalizeBridgeTargetName(normalized.Normalized)
	}
	normalized.TargetType = normalized.TargetType.Normalize()
	normalized.Qualifier = bridges.NormalizeBridgeTargetQualifier(normalized.Qualifier)
	normalized.Capabilities = normalizeGlobalDBBridgeTargetCapabilities(normalized.Capabilities)
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = refreshedAt
	}
	normalized.UpdatedAt = normalized.UpdatedAt.UTC()
	if normalized.LastSeenAt.IsZero() {
		normalized.LastSeenAt = refreshedAt
	}
	normalized.LastSeenAt = normalized.LastSeenAt.UTC()
	if err := normalized.Validate(); err != nil {
		return bridges.BridgeTarget{}, err
	}
	return normalized, nil
}

func normalizeGlobalDBBridgeTargetQuery(query bridges.BridgeTargetQuery) bridges.BridgeTargetQuery {
	normalized := query
	normalized.BridgeID = strings.TrimSpace(normalized.BridgeID)
	normalized.Query = strings.TrimSpace(normalized.Query)
	if normalized.Limit <= 0 {
		normalized.Limit = globalDBBridgeTargetDefaultLimit
	}
	if normalized.Limit > globalDBBridgeTargetMaxLimit {
		normalized.Limit = globalDBBridgeTargetMaxLimit
	}
	return normalized
}

type bridgeTargetScanner interface {
	Scan(dest ...any) error
}

func scanBridgeTarget(scanner bridgeTargetScanner) (bridges.BridgeTarget, error) {
	var (
		target          bridges.BridgeTarget
		targetType      string
		capabilitiesRaw string
		updatedAtRaw    string
		lastSeenRaw     sql.NullString
	)
	if err := scanner.Scan(
		&target.BridgeID,
		&target.CanonicalRoute,
		&target.DisplayName,
		&target.Normalized,
		&targetType,
		&target.Qualifier,
		&capabilitiesRaw,
		&updatedAtRaw,
		&lastSeenRaw,
	); err != nil {
		return bridges.BridgeTarget{}, fmt.Errorf("store: scan bridge target: %w", err)
	}
	capabilities, err := decodeBridgeTargetCapabilities(capabilitiesRaw)
	if err != nil {
		return bridges.BridgeTarget{}, err
	}
	updatedAt, err := store.ParseTimestamp(updatedAtRaw)
	if err != nil {
		return bridges.BridgeTarget{}, fmt.Errorf("store: parse bridge target updated_at: %w", err)
	}
	var lastSeenAt time.Time
	if lastSeenRaw.Valid && strings.TrimSpace(lastSeenRaw.String) != "" {
		parsed, parseErr := store.ParseTimestamp(lastSeenRaw.String)
		if parseErr != nil {
			return bridges.BridgeTarget{}, fmt.Errorf("store: parse bridge target last_seen_at: %w", parseErr)
		}
		lastSeenAt = parsed.UTC()
	}
	target.TargetType = bridges.BridgeTargetType(targetType).Normalize()
	target.Capabilities = capabilities
	target.UpdatedAt = updatedAt.UTC()
	target.LastSeenAt = lastSeenAt
	normalized, err := normalizeGlobalDBBridgeTarget(target.BridgeID, target, target.UpdatedAt)
	if err != nil {
		return bridges.BridgeTarget{}, err
	}
	return normalized, nil
}

func encodeBridgeTargetCapabilities(capabilities []string) (string, error) {
	normalized := normalizeGlobalDBBridgeTargetCapabilities(capabilities)
	if normalized == nil {
		normalized = []string{}
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("store: encode bridge target capabilities: %w", err)
	}
	return string(encoded), nil
}

func decodeBridgeTargetCapabilities(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	var capabilities []string
	if err := json.Unmarshal([]byte(trimmed), &capabilities); err != nil {
		return nil, fmt.Errorf("store: decode bridge target capabilities: %w", err)
	}
	return normalizeGlobalDBBridgeTargetCapabilities(capabilities), nil
}

func normalizeGlobalDBBridgeTargetCapabilities(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	capabilities := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		capabilities = append(capabilities, trimmed)
	}
	slices.Sort(capabilities)
	return capabilities
}

func escapeBridgeTargetLike(value string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_")
	return replacer.Replace(strings.ToLower(strings.TrimSpace(value)))
}
