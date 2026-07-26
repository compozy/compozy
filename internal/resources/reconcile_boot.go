package resources

import (
	"context"

	"time"
)

func (d *reconcileDriver) handleBootFailure(ctx context.Context, kind ResourceKind, result reconcilePassResult) {
	consecutiveFailures := 1
	degradedUntil := time.Time{}
	if d.failureThreshold <= 1 {
		degradedUntil = d.now().Add(d.degradedBackoff)
	}

	status := ReconcileHealthStatusFailing
	if !degradedUntil.IsZero() {
		status = ReconcileHealthStatusDegraded
	}
	health := ReconcileHealth{
		Kind:                kind,
		Status:              status,
		ConsecutiveFailures: consecutiveFailures,
		DegradedUntil:       degradedUntil,
		LastError:           result.err,
	}

	d.logger.Error(
		"resources: boot reconcile failed",
		"resource_kind",
		kind,
		"reconcile_reason",
		result.reason,
		"degraded_until",
		degradedUntil,
		"error",
		result.err,
	)
	d.emitEvent(ctx, ReconcileEvent{
		Type:                ReconcileEventFailed,
		Kind:                kind,
		Reason:              result.reason,
		Duration:            result.duration,
		ConsecutiveFailures: consecutiveFailures,
		Err:                 result.err,
	})
	if !degradedUntil.IsZero() {
		d.emitEvent(ctx, ReconcileEvent{
			Type:                ReconcileEventDegraded,
			Kind:                kind,
			Reason:              result.reason,
			ConsecutiveFailures: consecutiveFailures,
			DegradedUntil:       degradedUntil,
			Err:                 result.err,
		})
	}
	d.reportHealth(ctx, health)
}

func (d *reconcileDriver) handleBootSuccess(ctx context.Context, kind ResourceKind, result reconcilePassResult) {
	d.logger.Debug(
		"resources: boot reconcile applied",
		"resource_kind",
		kind,
		"reconcile_reason",
		result.reason,
		"revision",
		result.revision,
		"operations",
		result.operations,
		"duration",
		result.duration,
	)
	d.emitEvent(ctx, ReconcileEvent{
		Type:       ReconcileEventApplied,
		Kind:       kind,
		Reason:     result.reason,
		Duration:   result.duration,
		Revision:   result.revision,
		Operations: result.operations,
	})
	d.reportHealth(ctx, ReconcileHealth{
		Kind:   kind,
		Status: ReconcileHealthStatusHealthy,
	})
}

func (d *reconcileDriver) emitEvent(ctx context.Context, event ReconcileEvent) {
	if d.eventSink == nil {
		return
	}
	d.eventSink.ObserveReconcileEvent(ctx, event)
}

func (d *reconcileDriver) reportHealth(ctx context.Context, health ReconcileHealth) {
	if d.healthSink == nil {
		return
	}
	d.healthSink.ReportReconcileHealth(ctx, health)
}

func (d *reconcileDriver) notify() {
	select {
	case d.notifyCh <- struct{}{}:
	default:
	}
}

func (d *reconcileDriver) notifyLocked() {
	select {
	case d.notifyCh <- struct{}{}:
	default:
	}
}
