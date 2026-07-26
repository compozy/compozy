package observe

import (
	"context"
	"errors"
	"fmt"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges"
)

// CheckBridgeCatalogReady verifies that bounded health dependencies are wired.
func (o *Observer) CheckBridgeCatalogReady() error {
	if o == nil {
		return errors.New("observe: observer is required")
	}
	o.mu.RLock()
	source := o.bridgeSource
	o.mu.RUnlock()
	if source == nil {
		return errors.New("observe: bridge source is required")
	}
	return nil
}

// QueryBridgeEffectiveStatuses merges persisted lifecycle state with live runtime overrides.
func (o *Observer) QueryBridgeEffectiveStatuses(
	ctx context.Context,
	records []bridgepkg.BridgeCatalogRecord,
) (map[string]bridgepkg.BridgeStatus, error) {
	if o == nil {
		return nil, errors.New("observe: observer is required")
	}
	if ctx == nil {
		return nil, errors.New("observe: bridge effective status context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := o.CheckBridgeCatalogReady(); err != nil {
		return nil, err
	}

	o.mu.RLock()
	stateSnapshot := cloneObservedBridgeStateForRecords(o.bridgeState, records)
	o.mu.RUnlock()

	statuses := make(map[string]bridgepkg.BridgeStatus, len(records))
	for _, record := range records {
		id := strings.TrimSpace(record.ID)
		statuses[id] = effectiveBridgeLifecycleStatus(record.Enabled, record.Status, stateSnapshot[id])
	}
	return statuses, nil
}

// QueryBridgeHealthFor returns complete health only for the supplied bridge page.
func (o *Observer) QueryBridgeHealthFor(
	ctx context.Context,
	instances []bridgepkg.BridgeInstance,
) ([]BridgeInstanceHealth, error) {
	if o == nil {
		return nil, errors.New("observe: observer is required")
	}
	if ctx == nil {
		return nil, errors.New("observe: bridge page health context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	o.mu.RLock()
	source := o.bridgeSource
	stateSnapshot := cloneObservedBridgeStateForInstances(o.bridgeState, instances)
	o.mu.RUnlock()
	if source == nil {
		return nil, errors.New("observe: bridge source is required")
	}
	if len(instances) == 0 {
		return []BridgeInstanceHealth{}, nil
	}

	routeCounts, err := bridgePageRouteCounts(ctx, source, instances)
	if err != nil {
		return nil, err
	}
	ids := bridgePageIDs(instances)
	deliveryMetrics, err := source.DeliveryMetricsFor(ids)
	if err != nil {
		return nil, fmt.Errorf("observe: query delivery metrics for page: %w", err)
	}
	health := make([]BridgeInstanceHealth, 0, len(instances))
	for _, instance := range instances {
		id := strings.TrimSpace(instance.ID)
		item := BridgeInstanceHealth{
			BridgeInstanceID: id,
			Status:           effectiveBridgeStatus(instance, stateSnapshot[id]),
			RouteCount:       routeCounts[id],
		}
		if metrics, ok := deliveryMetrics[id]; ok {
			item.DeliveryBacklog = metrics.DeliveryBacklog
			item.DeliveryDroppedTotal = metrics.DeliveryDroppedTotal
			item.DeliveryDroppedByReason = cloneDroppedReasons(metrics.DeliveryDroppedByReason)
			item.DeliveryFailuresTotal = metrics.DeliveryFailuresTotal
			item.LastSuccessAt = metrics.LastSuccessAt
			item.LastError = strings.TrimSpace(metrics.LastError)
			item.LastErrorAt = metrics.LastErrorAt
		}
		if observed, ok := stateSnapshot[id]; ok {
			item.AuthFailuresTotal = observed.authFailuresTotal
			if observed.runtimeUpdatedAt.After(item.LastErrorAt) {
				item.LastError = strings.TrimSpace(observed.runtimeMessage)
				item.LastErrorAt = observed.runtimeUpdatedAt
			}
		}
		health = append(health, item)
	}
	return health, nil
}

func bridgePageRouteCounts(
	ctx context.Context,
	source BridgeSource,
	instances []bridgepkg.BridgeInstance,
) (map[string]int, error) {
	ids := bridgePageIDs(instances)
	counts, err := source.CountBridgeRoutes(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("observe: count bridge routes for page: %w", err)
	}
	return counts, nil
}

func bridgePageIDs(instances []bridgepkg.BridgeInstance) []string {
	ids := make([]string, 0, len(instances))
	for _, instance := range instances {
		if id := strings.TrimSpace(instance.ID); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func cloneObservedBridgeStateForInstances(
	state map[string]observedBridgeState,
	instances []bridgepkg.BridgeInstance,
) map[string]observedBridgeState {
	cloned := make(map[string]observedBridgeState, len(instances))
	for _, instance := range instances {
		id := strings.TrimSpace(instance.ID)
		if observed, ok := state[id]; ok {
			cloned[id] = observed
		}
	}
	return cloned
}

func cloneObservedBridgeStateForRecords(
	state map[string]observedBridgeState,
	records []bridgepkg.BridgeCatalogRecord,
) map[string]observedBridgeState {
	cloned := make(map[string]observedBridgeState, len(records))
	for _, record := range records {
		id := strings.TrimSpace(record.ID)
		if observed, ok := state[id]; ok {
			cloned[id] = observed
		}
	}
	return cloned
}
