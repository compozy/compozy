package extensiontest

import (
	"context"

	bridgepkg "github.com/compozy/agh/internal/bridges"
)

func (s harnessBridgeSource) CountBridgeRoutes(
	ctx context.Context,
	bridgeInstanceIDs []string,
) (map[string]int, error) {
	counts := make(map[string]int, len(bridgeInstanceIDs))
	for _, bridgeInstanceID := range bridgeInstanceIDs {
		routes, err := s.ListRoutes(ctx, bridgeInstanceID)
		if err != nil {
			return nil, err
		}
		counts[bridgeInstanceID] = len(routes)
	}
	return counts, nil
}

func (s harnessBridgeSource) DeliveryMetrics() map[string]bridgepkg.BridgeDeliveryMetrics {
	if s.broker == nil {
		return nil
	}
	return s.broker.DeliveryMetrics()
}

func (s harnessBridgeSource) DeliveryMetricsFor(
	bridgeInstanceIDs []string,
) (map[string]bridgepkg.BridgeDeliveryMetrics, error) {
	if s.broker == nil {
		return nil, nil
	}
	return s.broker.DeliveryMetricsFor(bridgeInstanceIDs)
}
