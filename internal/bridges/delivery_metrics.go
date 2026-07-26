package bridges

import "time"

// BridgeDeliveryMetrics captures the per-instance delivery telemetry exposed
// by the broker for observability surfaces.
type BridgeDeliveryMetrics struct {
	BridgeInstanceID        string         `json:"bridge_instance_id"`
	DeliveryBacklog         int            `json:"delivery_backlog"`
	DeliveryDroppedTotal    int            `json:"delivery_dropped_total"`
	DeliveryDroppedByReason map[string]int `json:"delivery_dropped_by_reason,omitempty"`
	DeliveryFailuresTotal   int            `json:"delivery_failures_total"`
	LastError               string         `json:"last_error,omitempty"`
	LastErrorAt             time.Time      `json:"last_error_at"`
	LastSuccessAt           time.Time      `json:"last_success_at"`
}
