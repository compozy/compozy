package daemon

import (
	"context"
	"errors"
	"fmt"

	core "github.com/compozy/agh/internal/api/core"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/observe"
)

var (
	_ core.BridgeService         = (*bridgeRuntime)(nil)
	_ core.Observer              = (*observe.Observer)(nil)
	_ core.BridgeCatalogObserver = (*observe.Observer)(nil)
	_ observe.BridgeSource       = (*bridgeRuntime)(nil)
)

type bridgeCatalogBatchStore interface {
	CountBridgeRoutes(ctx context.Context, bridgeInstanceIDs []string) (map[string]int, error)
	ListBridgeSecretBindingsForInstances(
		ctx context.Context,
		bridgeInstanceIDs []string,
	) (map[string][]bridgepkg.BridgeSecretBinding, error)
}

func (r *bridgeRuntime) CountBridgeRoutes(
	ctx context.Context,
	bridgeInstanceIDs []string,
) (map[string]int, error) {
	if r == nil || r.store == nil {
		return nil, errors.New("daemon: bridge runtime store is required")
	}
	counts, err := r.store.CountBridgeRoutes(ctx, bridgeInstanceIDs)
	if err != nil {
		return nil, fmt.Errorf("daemon: count bridge routes: %w", err)
	}
	return counts, nil
}

func (r *bridgeRuntime) ListSecretBindingsForInstances(
	ctx context.Context,
	bridgeInstanceIDs []string,
) (map[string][]bridgepkg.BridgeSecretBinding, error) {
	if r == nil || r.store == nil {
		return nil, errors.New("daemon: bridge runtime store is required")
	}
	bindings, err := r.store.ListBridgeSecretBindingsForInstances(ctx, bridgeInstanceIDs)
	if err != nil {
		return nil, fmt.Errorf("daemon: list bridge secret bindings for instances: %w", err)
	}
	return bindings, nil
}

func (r *bridgeRuntime) DeliveryMetrics() map[string]bridgepkg.BridgeDeliveryMetrics {
	if r == nil || r.broker == nil {
		return nil
	}
	return r.broker.DeliveryMetrics()
}

func (r *bridgeRuntime) DeliveryMetricsFor(
	bridgeInstanceIDs []string,
) (map[string]bridgepkg.BridgeDeliveryMetrics, error) {
	if r == nil || r.broker == nil {
		return nil, errors.New("daemon: bridge delivery broker is required")
	}
	metrics, err := r.broker.DeliveryMetricsFor(bridgeInstanceIDs)
	if err != nil {
		return nil, fmt.Errorf("daemon: query bridge delivery metrics: %w", err)
	}
	return metrics, nil
}
