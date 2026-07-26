package observe

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"
)

// BridgeSource exposes the daemon-owned bridge runtime data needed by
// observability health and query surfaces.
type BridgeSource interface {
	ListInstances(ctx context.Context) ([]bridgepkg.BridgeInstance, error)
	CountBridgeRoutes(ctx context.Context, bridgeInstanceIDs []string) (map[string]int, error)
	DeliveryMetrics() map[string]bridgepkg.BridgeDeliveryMetrics
	DeliveryMetricsFor(bridgeInstanceIDs []string) (map[string]bridgepkg.BridgeDeliveryMetrics, error)
}

// BridgeStatusCounts captures the current effective per-status instance counts.
type BridgeStatusCounts struct {
	Disabled     int `json:"disabled"`
	Starting     int `json:"starting"`
	Ready        int `json:"ready"`
	Degraded     int `json:"degraded"`
	AuthRequired int `json:"auth_required"`
	Error        int `json:"error"`
}

// BridgeAggregateHealth captures the additive bridge summary included in the
// daemon health surface.
type BridgeAggregateHealth struct {
	TotalInstances        int                `json:"total_instances"`
	RouteCount            int                `json:"route_count"`
	DeliveryBacklog       int                `json:"delivery_backlog"`
	DeliveryDroppedTotal  int                `json:"delivery_dropped_total"`
	DeliveryFailuresTotal int                `json:"delivery_failures_total"`
	AuthFailuresTotal     int                `json:"auth_failures_total"`
	StatusCounts          BridgeStatusCounts `json:"status_counts"`
}

// BridgeInstanceHealth captures the effective health/telemetry for one
// bridge instance.
type BridgeInstanceHealth struct {
	BridgeInstanceID        string                 `json:"bridge_instance_id"`
	Status                  bridgepkg.BridgeStatus `json:"status"`
	RouteCount              int                    `json:"route_count"`
	DeliveryBacklog         int                    `json:"delivery_backlog"`
	DeliveryDroppedTotal    int                    `json:"delivery_dropped_total"`
	DeliveryDroppedByReason map[string]int         `json:"delivery_dropped_by_reason,omitempty"`
	DeliveryFailuresTotal   int                    `json:"delivery_failures_total"`
	AuthFailuresTotal       int                    `json:"auth_failures_total"`
	LastSuccessAt           time.Time              `json:"last_success_at"`
	LastError               string                 `json:"last_error,omitempty"`
	LastErrorAt             time.Time              `json:"last_error_at"`
}

type observedBridgeState struct {
	authFailuresTotal int
	runtimeStatus     bridgepkg.BridgeStatus
	runtimeMessage    string
	runtimeUpdatedAt  time.Time
}

// WithBridgeSource injects the daemon-owned bridge runtime seam used for
// bridge-specific health aggregation and query helpers.
func WithBridgeSource(source BridgeSource) Option {
	return func(observer *Observer) {
		observer.bridgeSource = source
	}
}

// QueryBridgeHealth returns the current per-instance bridge health snapshot.
func (o *Observer) QueryBridgeHealth(ctx context.Context) ([]BridgeInstanceHealth, error) {
	health, _, err := o.collectBridgeHealth(ctx)
	if err != nil {
		return nil, err
	}

	ordered := make([]BridgeInstanceHealth, 0, len(health))
	ordered = append(ordered, health...)
	slices.SortFunc(ordered, func(left, right BridgeInstanceHealth) int {
		return strings.Compare(left.BridgeInstanceID, right.BridgeInstanceID)
	})
	return ordered, nil
}

// RecordBridgeAuthFailure increments the per-instance auth failure counter.
func (o *Observer) RecordBridgeAuthFailure(bridgeInstanceID string) {
	if o == nil {
		return
	}

	trimmedID := strings.TrimSpace(bridgeInstanceID)
	if trimmedID == "" {
		return
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	state := o.bridgeState[trimmedID]
	state.authFailuresTotal++
	o.bridgeState[trimmedID] = state
}

// RecordBridgeRuntimeIssue records a live runtime degradation/error signal for
// one instance without mutating the persisted bridge configuration.
func (o *Observer) RecordBridgeRuntimeIssue(bridgeInstanceID string, status bridgepkg.BridgeStatus, message string) {
	if o == nil {
		return
	}

	trimmedID := strings.TrimSpace(bridgeInstanceID)
	if trimmedID == "" {
		return
	}

	normalizedStatus := status.Normalize()
	if normalizedStatus != bridgepkg.BridgeStatusDegraded && normalizedStatus != bridgepkg.BridgeStatusError {
		return
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	state := o.bridgeState[trimmedID]
	state.runtimeStatus = normalizedStatus
	state.runtimeMessage = strings.TrimSpace(message)
	state.runtimeUpdatedAt = o.now()
	o.bridgeState[trimmedID] = state
}

// ClearBridgeRuntimeIssue removes the live runtime override for one instance
// after the adapter recovers.
func (o *Observer) ClearBridgeRuntimeIssue(bridgeInstanceID string) {
	if o == nil {
		return
	}

	trimmedID := strings.TrimSpace(bridgeInstanceID)
	if trimmedID == "" {
		return
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	state := o.bridgeState[trimmedID]
	state.runtimeStatus = ""
	state.runtimeMessage = ""
	state.runtimeUpdatedAt = time.Time{}
	o.bridgeState[trimmedID] = state
}

func (o *Observer) collectBridgeHealth(ctx context.Context) ([]BridgeInstanceHealth, BridgeAggregateHealth, error) {
	if ctx == nil {
		return nil, BridgeAggregateHealth{}, errors.New("observe: bridge health context is required")
	}

	o.mu.RLock()
	source := o.bridgeSource
	stateSnapshot := cloneObservedBridgeStateMap(o.bridgeState)
	o.mu.RUnlock()
	if source == nil {
		return nil, BridgeAggregateHealth{}, nil
	}

	instances, err := source.ListInstances(ctx)
	if err != nil {
		return nil, BridgeAggregateHealth{}, fmt.Errorf("observe: list bridge instances for health: %w", err)
	}

	deliveryMetrics := source.DeliveryMetrics()
	routeCounts, err := bridgePageRouteCounts(ctx, source, instances)
	if err != nil {
		return nil, BridgeAggregateHealth{}, err
	}
	health := make([]BridgeInstanceHealth, 0, len(instances))
	summary := BridgeAggregateHealth{TotalInstances: len(instances)}

	for _, instance := range instances {
		instanceID := strings.TrimSpace(instance.ID)
		item := BridgeInstanceHealth{
			BridgeInstanceID: instanceID,
			Status:           instance.Status.Normalize(),
			RouteCount:       routeCounts[instanceID],
		}

		if metrics, ok := deliveryMetrics[instanceID]; ok {
			item.DeliveryBacklog = metrics.DeliveryBacklog
			item.DeliveryDroppedTotal = metrics.DeliveryDroppedTotal
			item.DeliveryDroppedByReason = cloneDroppedReasons(metrics.DeliveryDroppedByReason)
			item.DeliveryFailuresTotal = metrics.DeliveryFailuresTotal
			item.LastSuccessAt = metrics.LastSuccessAt
			item.LastError = strings.TrimSpace(metrics.LastError)
			item.LastErrorAt = metrics.LastErrorAt
		}

		if observed, ok := stateSnapshot[instanceID]; ok {
			item.AuthFailuresTotal = observed.authFailuresTotal
			item.Status = effectiveBridgeStatus(instance, observed)
			if observed.runtimeUpdatedAt.After(item.LastErrorAt) {
				item.LastError = strings.TrimSpace(observed.runtimeMessage)
				item.LastErrorAt = observed.runtimeUpdatedAt
			}
		}

		summary.RouteCount += item.RouteCount
		summary.DeliveryBacklog += item.DeliveryBacklog
		summary.DeliveryDroppedTotal += item.DeliveryDroppedTotal
		summary.DeliveryFailuresTotal += item.DeliveryFailuresTotal
		summary.AuthFailuresTotal += item.AuthFailuresTotal
		incrementBridgeStatusCount(&summary.StatusCounts, item.Status)
		health = append(health, item)
	}

	return health, summary, nil
}

func effectiveBridgeStatus(instance bridgepkg.BridgeInstance, observed observedBridgeState) bridgepkg.BridgeStatus {
	return effectiveBridgeLifecycleStatus(instance.Enabled, instance.Status, observed)
}

func effectiveBridgeLifecycleStatus(
	enabled bool,
	status bridgepkg.BridgeStatus,
	observed observedBridgeState,
) bridgepkg.BridgeStatus {
	persisted := status.Normalize()
	if !enabled || persisted == bridgepkg.BridgeStatusDisabled {
		return bridgepkg.BridgeStatusDisabled
	}
	if persisted == bridgepkg.BridgeStatusAuthRequired {
		return bridgepkg.BridgeStatusAuthRequired
	}
	if observed.runtimeStatus.Normalize() == bridgepkg.BridgeStatusError {
		return bridgepkg.BridgeStatusError
	}
	if observed.runtimeStatus.Normalize() == bridgepkg.BridgeStatusDegraded {
		return bridgepkg.BridgeStatusDegraded
	}
	return persisted
}

func incrementBridgeStatusCount(counts *BridgeStatusCounts, status bridgepkg.BridgeStatus) {
	if counts == nil {
		return
	}

	switch status.Normalize() {
	case bridgepkg.BridgeStatusDisabled:
		counts.Disabled++
	case bridgepkg.BridgeStatusStarting:
		counts.Starting++
	case bridgepkg.BridgeStatusReady:
		counts.Ready++
	case bridgepkg.BridgeStatusDegraded:
		counts.Degraded++
	case bridgepkg.BridgeStatusAuthRequired:
		counts.AuthRequired++
	case bridgepkg.BridgeStatusError:
		counts.Error++
	}
}

func cloneDroppedReasons(input map[string]int) map[string]int {
	if len(input) == 0 {
		return nil
	}
	cloned := make(map[string]int, len(input))
	maps.Copy(cloned, input)
	return cloned
}

func cloneObservedBridgeStateMap(input map[string]observedBridgeState) map[string]observedBridgeState {
	if len(input) == 0 {
		return nil
	}
	cloned := make(map[string]observedBridgeState, len(input))
	maps.Copy(cloned, input)
	return cloned
}
