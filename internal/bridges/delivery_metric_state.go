package bridges

import (
	"strings"
	"time"

	redactpkg "github.com/compozy/agh/internal/redact"
)

func (b *Broker) metricsLocked(bridgeInstanceID string) *instanceDeliveryMetrics {
	if b.metrics == nil {
		b.metrics = make(map[string]*instanceDeliveryMetrics)
	}
	trimmedID := strings.TrimSpace(bridgeInstanceID)
	if trimmedID == "" {
		return nil
	}
	metrics := b.metrics[trimmedID]
	if metrics == nil {
		metrics = &instanceDeliveryMetrics{
			droppedByReason: make(map[string]int),
		}
		b.metrics[trimmedID] = metrics
	}
	return metrics
}

func (b *Broker) recordDeliveryDropLocked(bridgeInstanceID string, reason string) {
	metrics := b.metricsLocked(bridgeInstanceID)
	if metrics == nil {
		return
	}
	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		trimmedReason = "unknown"
	}
	metrics.droppedByReason[trimmedReason]++
	b.markDeliveryMetricsDirtyLocked(metrics)
}

func (b *Broker) recordDeliveryIssueLocked(bridgeInstanceID string, message string) {
	metrics := b.metricsLocked(bridgeInstanceID)
	if metrics == nil {
		return
	}
	metrics.lastError = redactpkg.String(strings.TrimSpace(message))
	metrics.lastErrorAt = b.now()
	b.markDeliveryMetricsDirtyLocked(metrics)
}

func (b *Broker) recordDeliveryFailureLocked(bridgeInstanceID string, message string) {
	metrics := b.metricsLocked(bridgeInstanceID)
	if metrics == nil {
		return
	}
	metrics.deliveryFailuresTotal++
	metrics.lastError = redactpkg.String(strings.TrimSpace(message))
	metrics.lastErrorAt = b.now()
	b.markDeliveryMetricsDirtyLocked(metrics)
}

func (b *Broker) recordDeliverySuccessLocked(bridgeInstanceID string, deliveredAt time.Time) {
	metrics := b.metricsLocked(bridgeInstanceID)
	if metrics == nil {
		return
	}
	metrics.lastSuccessAt = deliveredAt.UTC()
}
