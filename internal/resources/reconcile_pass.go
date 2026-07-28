package resources

import (
	"context"
	"errors"
	"fmt"

	"time"
)

func (d *reconcileDriver) runPass(
	ctx context.Context,
	kind ResourceKind,
	reason ReconcileReason,
) reconcilePassResult {
	startedAt := d.now()

	timeout := d.defaultTimeout
	if override, ok := d.kindTimeouts[kind]; ok && override > 0 {
		timeout = override
	}

	passCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	input, err := d.buildProjectionInput(passCtx, kind)
	if err != nil {
		return reconcilePassResult{
			reason:   reason,
			duration: d.now().Sub(startedAt),
			err:      err,
		}
	}

	projector := d.projectors[kind]
	plan, err := projector.Build(passCtx, input)
	if err == nil {
		err = projector.Apply(passCtx, plan)
	}
	if err != nil {
		return reconcilePassResult{
			reason:   reason,
			duration: d.now().Sub(startedAt),
			err:      err,
		}
	}

	return reconcilePassResult{
		reason:     reason,
		revision:   plan.Revision(),
		operations: plan.OperationCount(),
		duration:   d.now().Sub(startedAt),
	}
}

func (d *reconcileDriver) buildProjectionInput(ctx context.Context, kind ResourceKind) (projectionInput, error) {
	if d.raw == nil {
		return projectionInput{}, errors.New("resources: raw store is required for registered projectors")
	}

	records, err := d.raw.ListRaw(ctx, d.actor, ResourceFilter{Kind: kind})
	if err != nil {
		return projectionInput{}, fmt.Errorf("resources: list reconcile records for %q: %w", kind, err)
	}

	projector := d.projectors[kind]
	dependencies := make(map[ResourceKind][]RawRecord, len(projector.DependsOn()))
	for _, dependency := range normalizeKinds(projector.DependsOn()) {
		dependencyRecords, depErr := d.raw.ListRaw(ctx, d.actor, ResourceFilter{Kind: dependency})
		if depErr != nil {
			return projectionInput{}, fmt.Errorf(
				"resources: list reconcile dependency records for %q -> %q: %w",
				kind,
				dependency,
				depErr,
			)
		}
		dependencies[dependency] = dependencyRecords
	}

	return projectionInput{
		kind:         kind,
		revision:     maxRecordVersion(records),
		records:      records,
		dependencies: dependencies,
	}, nil
}

func maxRecordVersion(records []RawRecord) int64 {
	var revision int64
	for _, record := range records {
		if record.Version > revision {
			revision = record.Version
		}
	}
	return revision
}

func (d *reconcileDriver) finishAsyncPass(kind ResourceKind, result reconcilePassResult) {
	health, emitDegraded, ok := d.updateAsyncPassState(kind, result)
	if !ok {
		return
	}

	if result.err != nil {
		d.recordAsyncFailure(kind, result, health, emitDegraded)
		d.notify()
		return
	}

	d.recordAsyncSuccess(kind, result, health)
	d.notify()
}

func (d *reconcileDriver) updateAsyncPassState(
	kind ResourceKind,
	result reconcilePassResult,
) (ReconcileHealth, bool, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	state := d.kindStates[kind]
	if state == nil {
		return ReconcileHealth{}, false, false
	}
	state.running = false

	dirty := state.dirty
	dirtyReason := state.dirtyReason
	state.dirty = false
	state.dirtyReason = ""

	if result.err != nil {
		state.consecutiveFailures++
		if state.consecutiveFailures >= d.failureThreshold {
			state.degradedUntil = d.now().Add(d.degradedBackoff)
		}
	} else {
		state.consecutiveFailures = 0
		state.degradedUntil = time.Time{}
	}

	var health ReconcileHealth
	var emitDegraded bool
	if result.err != nil {
		health = ReconcileHealth{
			Kind:                kind,
			Status:              ReconcileHealthStatusFailing,
			ConsecutiveFailures: state.consecutiveFailures,
			LastError:           result.err,
		}
		if !state.degradedUntil.IsZero() {
			health.Status = ReconcileHealthStatusDegraded
			health.DegradedUntil = state.degradedUntil
			emitDegraded = true
		}
	} else {
		health = ReconcileHealth{
			Kind:   kind,
			Status: ReconcileHealthStatusHealthy,
		}
	}

	if dirty && !d.closed {
		state.pending = true
		state.pendingReason = dirtyReason
		state.readyAt = d.now().Add(d.coalesceWindow)
		d.queue = append([]ResourceKind{kind}, d.queue...)
	}

	return health, emitDegraded, true
}

func (d *reconcileDriver) recordAsyncFailure(
	kind ResourceKind,
	result reconcilePassResult,
	health ReconcileHealth,
	emitDegraded bool,
) {
	d.logger.Error(
		"resources: reconcile pass failed",
		"resource_kind",
		kind,
		"reconcile_reason",
		result.reason,
		"consecutive_failures",
		health.ConsecutiveFailures,
		"degraded_until",
		health.DegradedUntil,
		"error",
		result.err,
	)
	d.emitEvent(context.Background(), ReconcileEvent{
		Type:                ReconcileEventFailed,
		Kind:                kind,
		Reason:              result.reason,
		Duration:            result.duration,
		ConsecutiveFailures: health.ConsecutiveFailures,
		Err:                 result.err,
	})
	if emitDegraded {
		d.emitEvent(context.Background(), ReconcileEvent{
			Type:                ReconcileEventDegraded,
			Kind:                kind,
			Reason:              result.reason,
			ConsecutiveFailures: health.ConsecutiveFailures,
			DegradedUntil:       health.DegradedUntil,
			Err:                 result.err,
		})
	}
	d.reportHealth(context.Background(), health)
}

func (d *reconcileDriver) recordAsyncSuccess(
	kind ResourceKind,
	result reconcilePassResult,
	health ReconcileHealth,
) {
	d.logger.Debug(
		"resources: reconcile pass applied",
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
	d.emitEvent(context.Background(), ReconcileEvent{
		Type:       ReconcileEventApplied,
		Kind:       kind,
		Reason:     result.reason,
		Duration:   result.duration,
		Revision:   result.revision,
		Operations: result.operations,
	})
	d.reportHealth(context.Background(), health)
}
