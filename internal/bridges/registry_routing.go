package bridges

import (
	"context"

	"errors"
	"fmt"
)

// BuildRoutingKey canonicalizes the supplied routing identity under the owning instance policy.
func (s *Service) BuildRoutingKey(ctx context.Context, key RoutingKey) (RoutingKey, error) {
	if err := s.checkReady(ctx, "build routing key"); err != nil {
		return RoutingKey{}, err
	}

	bridgeID := key.BridgeInstanceID
	instance, err := s.store.GetBridgeInstance(ctx, bridgeID)
	if err != nil {
		return RoutingKey{}, fmt.Errorf("bridges: build routing key for %q: load bridge instance: %w", bridgeID, err)
	}
	canonicalKey, err := CanonicalizeRoutingKey(instance, key)
	if err != nil {
		return RoutingKey{}, fmt.Errorf("bridges: build routing key for %q: %w", bridgeID, err)
	}
	return canonicalKey, nil
}

// ResolveRoute resolves one route by canonical routing identity.
func (s *Service) ResolveRoute(ctx context.Context, key RoutingKey) (*BridgeRoute, error) {
	if err := s.checkReady(ctx, "resolve bridge route"); err != nil {
		return nil, err
	}

	bridgeID := key.BridgeInstanceID
	instance, err := s.loadRoutableInstance(ctx, bridgeID)
	if err != nil {
		return nil, fmt.Errorf("bridges: resolve bridge route for %q: load bridge instance: %w", bridgeID, err)
	}

	canonicalKey, err := CanonicalizeRoutingKey(instance, key)
	if err != nil {
		return nil, fmt.Errorf("bridges: resolve bridge route for %q: canonicalize routing key: %w", bridgeID, err)
	}

	route, err := s.store.ResolveBridgeRoute(ctx, canonicalKey)
	if err != nil {
		return nil, fmt.Errorf("bridges: resolve bridge route for %q: lookup route: %w", bridgeID, err)
	}
	return cloneBridgeRoute(route), nil
}

// ResolveOrCreateRoute reuses an existing session binding for the canonical key
// or persists the supplied route when no binding exists yet.
func (s *Service) ResolveOrCreateRoute(ctx context.Context, route BridgeRoute) (*BridgeRoute, bool, error) {
	if err := s.checkReady(ctx, "resolve or create bridge route"); err != nil {
		return nil, false, err
	}

	bridgeID := route.BridgeInstanceID
	instance, err := s.loadRoutableInstance(ctx, bridgeID)
	if err != nil {
		return nil, false, fmt.Errorf(
			"bridges: resolve or create bridge route for %q: load bridge instance: %w",
			bridgeID,
			err,
		)
	}

	canonicalRoute, err := CanonicalizeRoute(instance, route)
	if err != nil {
		return nil, false, fmt.Errorf(
			"bridges: resolve or create bridge route for %q: canonicalize route: %w",
			bridgeID,
			err,
		)
	}

	existing, err := s.store.ResolveBridgeRoute(ctx, canonicalRoute.RoutingKey())
	if err == nil {
		refreshed := existing
		refreshed.LastActivityAt = canonicalRoute.LastActivityAt
		refreshed.UpdatedAt = canonicalRoute.UpdatedAt
		refreshed = s.prepareRouteForWrite(refreshed, &existing)
		if err := s.store.PutBridgeRoute(ctx, refreshed); err != nil {
			return nil, false, fmt.Errorf(
				"bridges: resolve or create bridge route for %q: refresh route: %w",
				bridgeID,
				err,
			)
		}
		return cloneBridgeRoute(refreshed), false, nil
	}
	if !errors.Is(err, ErrBridgeRouteNotFound) {
		return nil, false, fmt.Errorf(
			"bridges: resolve or create bridge route for %q: lookup route: %w",
			bridgeID,
			err,
		)
	}

	canonicalRoute = s.prepareRouteForWrite(canonicalRoute, nil)
	if err := s.store.PutBridgeRoute(ctx, canonicalRoute); err != nil {
		return nil, false, fmt.Errorf(
			"bridges: resolve or create bridge route for %q: create route: %w",
			bridgeID,
			err,
		)
	}

	return cloneBridgeRoute(canonicalRoute), true, nil
}

// UpsertRoute writes a route using the canonical key derived from the owning instance policy.
func (s *Service) UpsertRoute(ctx context.Context, route BridgeRoute) (*BridgeRoute, error) {
	if err := s.checkReady(ctx, "upsert bridge route"); err != nil {
		return nil, err
	}

	bridgeID := route.BridgeInstanceID
	instance, err := s.loadRoutableInstance(ctx, bridgeID)
	if err != nil {
		return nil, fmt.Errorf("bridges: upsert bridge route for %q: load bridge instance: %w", bridgeID, err)
	}

	canonicalRoute, err := CanonicalizeRoute(instance, route)
	if err != nil {
		return nil, fmt.Errorf("bridges: upsert bridge route for %q: canonicalize route: %w", bridgeID, err)
	}

	existing, err := s.store.ResolveBridgeRoute(ctx, canonicalRoute.RoutingKey())
	if err != nil && !errors.Is(err, ErrBridgeRouteNotFound) {
		return nil, fmt.Errorf("bridges: upsert bridge route for %q: lookup route: %w", bridgeID, err)
	}
	var existingRoute *BridgeRoute
	if err == nil {
		existingRoute = &existing
	}

	canonicalRoute = s.prepareRouteForWrite(canonicalRoute, existingRoute)
	if err := s.store.PutBridgeRoute(ctx, canonicalRoute); err != nil {
		return nil, fmt.Errorf("bridges: upsert bridge route for %q: persist route: %w", bridgeID, err)
	}

	return cloneBridgeRoute(canonicalRoute), nil
}

// ListRoutes returns the persisted routes owned by one bridge instance.
func (s *Service) ListRoutes(ctx context.Context, bridgeInstanceID string) ([]BridgeRoute, error) {
	if err := s.checkReady(ctx, "list bridge routes"); err != nil {
		return nil, err
	}

	if _, err := s.store.GetBridgeInstance(ctx, bridgeInstanceID); err != nil {
		return nil, fmt.Errorf("bridges: list bridge routes for %q: load bridge instance: %w", bridgeInstanceID, err)
	}
	routes, err := s.store.ListBridgeRoutes(ctx, bridgeInstanceID)
	if err != nil {
		return nil, fmt.Errorf("bridges: list bridge routes for %q: %w", bridgeInstanceID, err)
	}
	if len(routes) == 0 {
		return routes, nil
	}

	cloned := make([]BridgeRoute, 0, len(routes))
	for _, route := range routes {
		cloned = append(cloned, *cloneBridgeRoute(route))
	}
	return cloned, nil
}
