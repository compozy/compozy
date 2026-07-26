package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func (g *BridgeRepo) PutBridgeSecretBinding(ctx context.Context, binding bridges.BridgeSecretBinding) error {
	if err := g.checkReady(ctx, "put bridge secret binding"); err != nil {
		return err
	}

	normalized := binding
	if err := normalized.Validate(); err != nil {
		return err
	}
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = g.now()
	}
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = normalized.CreatedAt
	}

	if err := g.queries.UpsertBridgeSecretBinding(ctx, sqlcgen.UpsertBridgeSecretBindingParams{
		BridgeInstanceID: normalized.BridgeInstanceID, BindingName: normalized.BindingName,
		SecretRef: normalized.SecretRef, Kind: normalized.Kind,
		CreatedAt: store.FormatTimestamp(normalized.CreatedAt), UpdatedAt: store.FormatTimestamp(normalized.UpdatedAt),
	}); err != nil {
		return fmt.Errorf(
			"store: put bridge secret binding %q/%q: %w",
			normalized.BridgeInstanceID,
			normalized.BindingName,
			mapBridgeChildConstraintError(err),
		)
	}

	return nil
}

// GetBridgeSecretBinding loads one persisted secret binding by composite primary key.
func (g *BridgeRepo) GetBridgeSecretBinding(
	ctx context.Context,
	bridgeInstanceID string,
	bindingName string,
) (bridges.BridgeSecretBinding, error) {
	if err := g.checkReady(ctx, "get bridge secret binding"); err != nil {
		return bridges.BridgeSecretBinding{}, err
	}

	trimmedInstanceID := strings.TrimSpace(bridgeInstanceID)
	trimmedBindingName := strings.TrimSpace(bindingName)
	if trimmedInstanceID == "" {
		return bridges.BridgeSecretBinding{}, errors.New("store: bridge instance id is required")
	}
	if trimmedBindingName == "" {
		return bridges.BridgeSecretBinding{}, errors.New("store: bridge secret binding name is required")
	}

	row, err := g.queries.GetBridgeSecretBinding(ctx, sqlcgen.GetBridgeSecretBindingParams{
		BridgeInstanceID: trimmedInstanceID, BindingName: trimmedBindingName,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return bridges.BridgeSecretBinding{}, bridges.ErrBridgeSecretBindingNotFound
		}
		return bridges.BridgeSecretBinding{}, err
	}
	return bridgeBindingFromGenerated(row)
}

// ListBridgeSecretBindings returns the persisted secret bindings for one bridge instance.
func (g *BridgeRepo) ListBridgeSecretBindings(
	ctx context.Context,
	bridgeInstanceID string,
) (bindings []bridges.BridgeSecretBinding, err error) {
	if err := g.checkReady(ctx, "list bridge secret bindings"); err != nil {
		return nil, err
	}

	trimmedInstanceID := strings.TrimSpace(bridgeInstanceID)
	if trimmedInstanceID == "" {
		return nil, errors.New("store: bridge instance id is required")
	}

	rows, err := g.queries.ListBridgeSecretBindings(ctx, trimmedInstanceID)
	if err != nil {
		return nil, fmt.Errorf("store: query bridge secret bindings for %q: %w", trimmedInstanceID, err)
	}
	bindings = make([]bridges.BridgeSecretBinding, 0, len(rows))
	for _, row := range rows {
		binding, scanErr := bridgeBindingFromGenerated(row)
		if scanErr != nil {
			return nil, scanErr
		}
		bindings = append(bindings, binding)
	}
	return bindings, nil
}

// DeleteBridgeSecretBinding removes one persisted secret binding row.
func (g *BridgeRepo) DeleteBridgeSecretBinding(ctx context.Context, bridgeInstanceID string, bindingName string) error {
	if err := g.checkReady(ctx, "delete bridge secret binding"); err != nil {
		return err
	}

	trimmedInstanceID := strings.TrimSpace(bridgeInstanceID)
	trimmedBindingName := strings.TrimSpace(bindingName)
	if trimmedInstanceID == "" {
		return errors.New("store: bridge instance id is required")
	}
	if trimmedBindingName == "" {
		return errors.New("store: bridge secret binding name is required")
	}

	affected, err := g.queries.DeleteBridgeSecretBinding(ctx, sqlcgen.DeleteBridgeSecretBindingParams{
		BridgeInstanceID: trimmedInstanceID, BindingName: trimmedBindingName,
	})
	if err != nil {
		return fmt.Errorf("store: delete bridge secret binding %q/%q: %w", trimmedInstanceID, trimmedBindingName, err)
	}

	if affected == 0 {
		return fmt.Errorf(
			"store: bridge secret binding %q/%q: %w",
			trimmedInstanceID,
			trimmedBindingName,
			bridges.ErrBridgeSecretBindingNotFound,
		)
	}

	return nil
}

// PutBridgeRoute inserts or refreshes a persisted bridge route row.
func (g *BridgeRepo) PutBridgeRoute(ctx context.Context, route bridges.BridgeRoute) error {
	if err := g.checkReady(ctx, "put bridge route"); err != nil {
		return err
	}

	normalized, err := route.Canonicalize()
	if err != nil {
		return err
	}
	if normalized.LastActivityAt.IsZero() {
		normalized.LastActivityAt = g.now()
	}
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = normalized.LastActivityAt
	}
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = normalized.LastActivityAt
	}

	if err := g.queries.UpsertBridgeRoute(ctx, sqlcgen.UpsertBridgeRouteParams{
		RoutingKeyHash:   normalized.RoutingKeyHash,
		Scope:            string(normalized.Scope),
		WorkspaceID:      nullableBridgeString(normalized.WorkspaceID),
		BridgeInstanceID: normalized.BridgeInstanceID,
		PeerID:           nullableBridgeString(normalized.PeerID),
		ThreadID:         nullableBridgeString(normalized.ThreadID),
		GroupID: nullableBridgeString(
			normalized.GroupID,
		),
		SessionID: normalized.SessionID,
		AgentName: normalized.AgentName,
		LastActivityAt: store.FormatTimestamp(
			normalized.LastActivityAt,
		),
		CreatedAt: store.FormatTimestamp(normalized.CreatedAt),
		UpdatedAt: store.FormatTimestamp(normalized.UpdatedAt),
	}); err != nil {
		return fmt.Errorf(
			"store: put bridge route %q: %w",
			normalized.RoutingKeyHash,
			mapBridgeChildConstraintError(err),
		)
	}

	return nil
}

// GetBridgeRoute loads one persisted route by routing-key hash.
func (g *BridgeRepo) GetBridgeRoute(ctx context.Context, routingKeyHash string) (bridges.BridgeRoute, error) {
	if err := g.checkReady(ctx, "get bridge route"); err != nil {
		return bridges.BridgeRoute{}, err
	}

	trimmedHash := strings.TrimSpace(routingKeyHash)
	if trimmedHash == "" {
		return bridges.BridgeRoute{}, errors.New("store: routing key hash is required")
	}

	row, err := g.queries.GetBridgeRoute(ctx, trimmedHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return bridges.BridgeRoute{}, bridges.ErrBridgeRouteNotFound
		}
		return bridges.BridgeRoute{}, err
	}
	return bridgeRouteFromGenerated(row)
}

// ResolveBridgeRoute loads a persisted route by computing the hash for the supplied routing key.
func (g *BridgeRepo) ResolveBridgeRoute(ctx context.Context, key bridges.RoutingKey) (bridges.BridgeRoute, error) {
	if err := g.checkReady(ctx, "resolve bridge route"); err != nil {
		return bridges.BridgeRoute{}, err
	}

	hash, err := key.Hash()
	if err != nil {
		return bridges.BridgeRoute{}, err
	}
	return g.GetBridgeRoute(ctx, hash)
}

// ListBridgeRoutes returns persisted routes for one bridge instance ordered by recency.
func (g *BridgeRepo) ListBridgeRoutes(
	ctx context.Context,
	bridgeInstanceID string,
) (routes []bridges.BridgeRoute, err error) {
	if err := g.checkReady(ctx, "list bridge routes"); err != nil {
		return nil, err
	}

	trimmedInstanceID := strings.TrimSpace(bridgeInstanceID)
	if trimmedInstanceID == "" {
		return nil, errors.New("store: bridge instance id is required")
	}

	rows, err := g.queries.ListBridgeRoutes(ctx, trimmedInstanceID)
	if err != nil {
		return nil, fmt.Errorf("store: query bridge routes for %q: %w", trimmedInstanceID, err)
	}
	routes = make([]bridges.BridgeRoute, 0, len(rows))
	for _, row := range rows {
		route, scanErr := bridgeRouteFromGenerated(row)
		if scanErr != nil {
			return nil, scanErr
		}
		routes = append(routes, route)
	}
	return routes, nil
}

// DeleteBridgeRoute removes one persisted route row.
func (g *BridgeRepo) DeleteBridgeRoute(ctx context.Context, routingKeyHash string) error {
	if err := g.checkReady(ctx, "delete bridge route"); err != nil {
		return err
	}

	trimmedHash := strings.TrimSpace(routingKeyHash)
	if trimmedHash == "" {
		return errors.New("store: routing key hash is required")
	}

	affected, err := g.queries.DeleteBridgeRoute(ctx, trimmedHash)
	if err != nil {
		return fmt.Errorf("store: delete bridge route %q: %w", trimmedHash, err)
	}

	if affected == 0 {
		return fmt.Errorf("store: bridge route %q: %w", trimmedHash, bridges.ErrBridgeRouteNotFound)
	}

	return nil
}

// PutBridgeIngestDedup inserts or refreshes an ingest dedup record.
