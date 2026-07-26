package bridges

import (
	"context"
	"errors"
	"fmt"
	"maps"

	redactpkg "github.com/compozy/agh/internal/redact"
)

// LoadDeliveryMetrics hydrates one exact durable scope before health surfaces become available.
func (b *Broker) LoadDeliveryMetrics(ctx context.Context, query DeliveryLedgerQuery) error {
	if b == nil {
		return errors.New("bridges: delivery broker is required")
	}
	if ctx == nil {
		return errors.New("bridges: delivery metrics context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if b.ledgerStore == nil {
		return errors.New("bridges: delivery ledger store is required")
	}
	normalized, err := query.Canonicalize()
	if err != nil {
		return err
	}
	persisted, err := b.ledgerStore.ListBridgeDeliveryMetrics(ctx, normalized)
	if err != nil {
		return fmt.Errorf("bridges: load durable delivery metrics: %w", err)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	for bridgeInstanceID, entry := range persisted {
		metrics := b.metricsLocked(bridgeInstanceID)
		if metrics == nil {
			continue
		}
		metrics.scope = normalized.Scope
		metrics.workspaceID = normalized.WorkspaceID
		metrics.droppedByReason = cloneDeliveryDropReasons(entry.DeliveryDroppedByReason)
		metrics.deliveryFailuresTotal = entry.DeliveryFailuresTotal
		metrics.lastError = entry.LastError
		metrics.lastErrorAt = entry.LastErrorAt
		metrics.lastSuccessAt = entry.LastSuccessAt
	}
	return nil
}

// DeliveryMetrics returns a point-in-time snapshot of all broker telemetry.
func (b *Broker) DeliveryMetrics() map[string]BridgeDeliveryMetrics {
	if b == nil {
		return nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	ids := make([]string, 0, len(b.metrics)+len(b.bridgeRoutes))
	seen := make(map[string]struct{}, len(b.metrics)+len(b.bridgeRoutes))
	for bridgeInstanceID := range b.metrics {
		seen[bridgeInstanceID] = struct{}{}
		ids = append(ids, bridgeInstanceID)
	}
	for bridgeInstanceID := range b.bridgeRoutes {
		if _, exists := seen[bridgeInstanceID]; exists {
			continue
		}
		ids = append(ids, bridgeInstanceID)
	}
	return b.deliveryMetricsForIDsLocked(ids)
}

// DeliveryMetricPersistenceError returns accumulated durable-metric write failures.
func (b *Broker) DeliveryMetricPersistenceError() error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.metricPersistErr
}

// DeliveryMetricsFor returns telemetry for one bounded set of bridge IDs.
func (b *Broker) DeliveryMetricsFor(
	bridgeInstanceIDs []string,
) (map[string]BridgeDeliveryMetrics, error) {
	if b == nil {
		return nil, errors.New("bridges: delivery broker is required")
	}
	ids, err := normalizeBoundedBridgeInstanceIDs(bridgeInstanceIDs)
	if err != nil {
		return nil, err
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	return b.deliveryMetricsForIDsLocked(ids), nil
}

func (b *Broker) deliveryMetricsForIDsLocked(ids []string) map[string]BridgeDeliveryMetrics {
	snapshot := make(map[string]BridgeDeliveryMetrics, len(ids))
	for _, bridgeInstanceID := range ids {
		metrics := b.metrics[bridgeInstanceID]
		if metrics != nil {
			clonedReasons := make(map[string]int, len(metrics.droppedByReason))
			totalDropped := 0
			for reason, count := range metrics.droppedByReason {
				clonedReasons[reason] = count
				totalDropped += count
			}
			snapshot[bridgeInstanceID] = BridgeDeliveryMetrics{
				BridgeInstanceID:        bridgeInstanceID,
				DeliveryDroppedTotal:    totalDropped,
				DeliveryDroppedByReason: clonedReasons,
				DeliveryFailuresTotal:   metrics.deliveryFailuresTotal,
				LastError:               metrics.lastError,
				LastErrorAt:             metrics.lastErrorAt,
				LastSuccessAt:           metrics.lastSuccessAt,
			}
		}
		for _, route := range b.bridgeRoutes[bridgeInstanceID] {
			if route == nil {
				continue
			}
			entry := snapshot[bridgeInstanceID]
			entry.BridgeInstanceID = bridgeInstanceID
			entry.DeliveryBacklog += len(route.queue)
			snapshot[bridgeInstanceID] = entry
		}
		if issue, ok := b.metricPersistIssues[bridgeInstanceID]; ok && issue.err != nil {
			entry := snapshot[bridgeInstanceID]
			entry.BridgeInstanceID = bridgeInstanceID
			persistenceError := redactpkg.String("delivery metric persistence: " + issue.err.Error())
			if entry.LastError == "" {
				entry.LastError = persistenceError
			} else {
				entry.LastError += "; " + persistenceError
			}
			if entry.LastErrorAt.Before(issue.at) {
				entry.LastErrorAt = issue.at
			}
			snapshot[bridgeInstanceID] = entry
		}
	}
	return snapshot
}

func cloneDeliveryDropReasons(reasons map[string]int) map[string]int {
	cloned := make(map[string]int, len(reasons))
	maps.Copy(cloned, reasons)
	return cloned
}
