package bridges

import (
	"context"

	"errors"
	"fmt"
	"strings"
)

// BuildRoutingKey canonicalizes the supplied routing identity under the owning instance policy.
func (s *Service) BuildRoutingKey(ctx context.Context, key RoutingKey) (RoutingKey, error) {
	if err := s.checkReady(ctx, "build routing key"); err != nil {
		return RoutingKey{}, err
	}

	trimmedID := strings.TrimSpace(key.BridgeInstanceID)
	instance, err := s.store.GetBridgeInstance(ctx, trimmedID)
	if err != nil {
		return RoutingKey{}, fmt.Errorf("bridges: build routing key for %q: load bridge instance: %w", trimmedID, err)
	}
	canonicalKey, err := CanonicalizeRoutingKey(instance, key)
	if err != nil {
		return RoutingKey{}, fmt.Errorf("bridges: build routing key for %q: %w", trimmedID, err)
	}
	return canonicalKey, nil
}

// ResolveRoute resolves one route by canonical routing identity.
func (s *Service) ResolveRoute(ctx context.Context, key RoutingKey) (*BridgeRoute, error) {
	if err := s.checkReady(ctx, "resolve bridge route"); err != nil {
		return nil, err
	}

	trimmedID := strings.TrimSpace(key.BridgeInstanceID)
	instance, err := s.loadRoutableInstance(ctx, trimmedID)
	if err != nil {
		return nil, fmt.Errorf("bridges: resolve bridge route for %q: load bridge instance: %w", trimmedID, err)
	}

	canonicalKey, err := CanonicalizeRoutingKey(instance, key)
	if err != nil {
		return nil, fmt.Errorf("bridges: resolve bridge route for %q: canonicalize routing key: %w", trimmedID, err)
	}

	route, err := s.store.ResolveBridgeRoute(ctx, canonicalKey)
	if err != nil {
		return nil, fmt.Errorf("bridges: resolve bridge route for %q: lookup route: %w", trimmedID, err)
	}
	return cloneBridgeRoute(route), nil
}

// ResolveOrCreateRoute reuses an existing session binding for the canonical key
// or persists the supplied route when no binding exists yet.
func (s *Service) ResolveOrCreateRoute(ctx context.Context, route BridgeRoute) (*BridgeRoute, bool, error) {
	if err := s.checkReady(ctx, "resolve or create bridge route"); err != nil {
		return nil, false, err
	}

	trimmedID := strings.TrimSpace(route.BridgeInstanceID)
	instance, err := s.loadRoutableInstance(ctx, trimmedID)
	if err != nil {
		return nil, false, fmt.Errorf(
			"bridges: resolve or create bridge route for %q: load bridge instance: %w",
			trimmedID,
			err,
		)
	}

	canonicalRoute, err := CanonicalizeRoute(instance, route)
	if err != nil {
		return nil, false, fmt.Errorf(
			"bridges: resolve or create bridge route for %q: canonicalize route: %w",
			trimmedID,
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
				trimmedID,
				err,
			)
		}
		return cloneBridgeRoute(refreshed), false, nil
	}
	if !errors.Is(err, ErrBridgeRouteNotFound) {
		return nil, false, fmt.Errorf(
			"bridges: resolve or create bridge route for %q: lookup route: %w",
			trimmedID,
			err,
		)
	}

	canonicalRoute = s.prepareRouteForWrite(canonicalRoute, nil)
	if err := s.store.PutBridgeRoute(ctx, canonicalRoute); err != nil {
		return nil, false, fmt.Errorf(
			"bridges: resolve or create bridge route for %q: create route: %w",
			trimmedID,
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

	trimmedID := strings.TrimSpace(route.BridgeInstanceID)
	instance, err := s.loadRoutableInstance(ctx, trimmedID)
	if err != nil {
		return nil, fmt.Errorf("bridges: upsert bridge route for %q: load bridge instance: %w", trimmedID, err)
	}

	canonicalRoute, err := CanonicalizeRoute(instance, route)
	if err != nil {
		return nil, fmt.Errorf("bridges: upsert bridge route for %q: canonicalize route: %w", trimmedID, err)
	}

	existing, err := s.store.ResolveBridgeRoute(ctx, canonicalRoute.RoutingKey())
	if err != nil && !errors.Is(err, ErrBridgeRouteNotFound) {
		return nil, fmt.Errorf("bridges: upsert bridge route for %q: lookup route: %w", trimmedID, err)
	}
	var existingRoute *BridgeRoute
	if err == nil {
		existingRoute = &existing
	}

	canonicalRoute = s.prepareRouteForWrite(canonicalRoute, existingRoute)
	if err := s.store.PutBridgeRoute(ctx, canonicalRoute); err != nil {
		return nil, fmt.Errorf("bridges: upsert bridge route for %q: persist route: %w", trimmedID, err)
	}

	return cloneBridgeRoute(canonicalRoute), nil
}

// ListRoutes returns the persisted routes owned by one bridge instance.
func (s *Service) ListRoutes(ctx context.Context, bridgeInstanceID string) ([]BridgeRoute, error) {
	if err := s.checkReady(ctx, "list bridge routes"); err != nil {
		return nil, err
	}

	trimmedInstanceID := strings.TrimSpace(bridgeInstanceID)
	if _, err := s.store.GetBridgeInstance(ctx, trimmedInstanceID); err != nil {
		return nil, fmt.Errorf("bridges: list bridge routes for %q: load bridge instance: %w", trimmedInstanceID, err)
	}
	routes, err := s.store.ListBridgeRoutes(ctx, trimmedInstanceID)
	if err != nil {
		return nil, fmt.Errorf("bridges: list bridge routes for %q: %w", trimmedInstanceID, err)
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
