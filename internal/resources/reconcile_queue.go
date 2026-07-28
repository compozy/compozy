package resources

import (
	"context"
	"errors"
	"fmt"

	"sort"

	"time"
)

func (d *reconcileDriver) Trigger(ctx context.Context, kind ResourceKind, reason ReconcileReason) error {
	if ctx == nil {
		return errors.New("resources: reconcile trigger context is required")
	}
	if d.topoErr != nil {
		return d.topoErr
	}

	normalizedKind := kind.Normalize()
	if err := normalizedKind.Validate("kind"); err != nil {
		return err
	}
	normalizedReason := reason.Normalize()
	if err := normalizedReason.Validate("reason"); err != nil {
		return err
	}

	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return errors.New("resources: reconcile driver is closed")
	}
	_, hasProjector := d.projectors[normalizedKind]
	_, hasDependents := d.dependents[normalizedKind]
	if !hasProjector && !hasDependents {
		d.mu.Unlock()
		return fmt.Errorf("%w: reconcile kind %q is not registered", ErrValidation, normalizedKind)
	}

	scheduledKinds := d.scheduleCascade(normalizedKind)
	events := make([]ReconcileEvent, 0, len(scheduledKinds)*2)
	for idx, scheduledKind := range scheduledKinds {
		scheduledReason := normalizedReason
		if idx > 0 {
			scheduledReason = ReconcileReasonDependency
		}
		events = append(events, ReconcileEvent{
			Type:   ReconcileEventRequested,
			Kind:   scheduledKind,
			Reason: scheduledReason,
		})
		if coalesced := d.enqueueLocked(scheduledKind, scheduledReason); coalesced != nil {
			events = append(events, *coalesced)
		}
	}
	d.mu.Unlock()

	for _, event := range events {
		d.emitEvent(ctx, event)
	}
	d.notify()
	return nil
}

func (d *reconcileDriver) RunBoot(ctx context.Context) error {
	if ctx == nil {
		return errors.New("resources: reconcile boot context is required")
	}
	if d.topoErr != nil {
		return d.topoErr
	}

	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return errors.New("resources: reconcile driver is closed")
	}
	order := append([]ResourceKind(nil), d.bootOrder...)
	d.mu.Unlock()

	for _, kind := range order {
		d.emitEvent(ctx, ReconcileEvent{
			Type:   ReconcileEventRequested,
			Kind:   kind,
			Reason: ReconcileReasonBoot,
		})
		result := d.runPass(ctx, kind, ReconcileReasonBoot)
		if result.err != nil {
			d.handleBootFailure(ctx, kind, result)
			return result.err
		}
		d.handleBootSuccess(ctx, kind, result)
	}
	return nil
}

func (d *reconcileDriver) Close(ctx context.Context) error {
	if ctx == nil {
		return errors.New("resources: reconcile close context is required")
	}

	d.mu.Lock()
	if !d.closed {
		d.closed = true
		d.queue = nil
		for _, state := range d.kindStates {
			state.pending = false
			state.dirty = false
			state.pendingReason = ""
			state.dirtyReason = ""
			state.readyAt = time.Time{}
		}
		d.workerCancel()
		d.notifyLocked()
	}
	doneCh := d.doneCh
	d.mu.Unlock()

	select {
	case <-doneCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (d *reconcileDriver) scheduleCascade(root ResourceKind) []ResourceKind {
	ordered := []ResourceKind{root}
	visited := map[ResourceKind]struct{}{
		root: {},
	}
	var reachable []ResourceKind
	queue := append([]ResourceKind(nil), d.dependents[root]...)
	for len(queue) > 0 {
		kind := queue[0]
		queue = queue[1:]
		if _, seen := visited[kind]; seen {
			continue
		}
		visited[kind] = struct{}{}
		reachable = append(reachable, kind)
		queue = append(queue, d.dependents[kind]...)
	}

	sort.Slice(reachable, func(i int, j int) bool {
		left := reachable[i]
		right := reachable[j]
		leftRank, leftOK := d.topoRank[left]
		rightRank, rightOK := d.topoRank[right]
		switch {
		case leftOK && rightOK:
			return leftRank < rightRank
		case leftOK:
			return true
		case rightOK:
			return false
		default:
			return string(left) < string(right)
		}
	})

	return append(ordered, reachable...)
}

func (d *reconcileDriver) enqueueLocked(kind ResourceKind, reason ReconcileReason) *ReconcileEvent {
	state := d.kindStates[kind]
	if state == nil {
		return nil
	}

	if reason == ReconcileReasonWrite && state.degradedUntil.After(d.now()) {
		state.degradedUntil = time.Time{}
		state.readyAt = time.Time{}
	}

	if state.running {
		state.dirty = true
		state.dirtyReason = reason
		return &ReconcileEvent{
			Type:   ReconcileEventCoalesced,
			Kind:   kind,
			Reason: reason,
		}
	}

	if state.pending {
		state.pendingReason = reason
		return &ReconcileEvent{
			Type:   ReconcileEventCoalesced,
			Kind:   kind,
			Reason: reason,
		}
	}

	state.pending = true
	state.pendingReason = reason
	state.readyAt = time.Time{}
	d.queue = append(d.queue, kind)
	return nil
}

func (d *reconcileDriver) run() {
	defer close(d.doneCh)

	for {
		kind, reason, waitUntil, ok := d.nextPending()
		if !ok {
			if d.shouldExit() {
				return
			}
			if !d.waitForWork(waitUntil) {
				return
			}
			continue
		}

		result := d.runPass(d.workerCtx, kind, reason)
		d.finishAsyncPass(kind, result)
	}
}

func (d *reconcileDriver) shouldExit() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.closed {
		return false
	}
	for _, state := range d.kindStates {
		if state.running {
			return false
		}
	}
	return true
}

func (d *reconcileDriver) nextPending() (ResourceKind, ReconcileReason, time.Time, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := d.now()
	var earliest time.Time
	for idx, kind := range d.queue {
		state := d.kindStates[kind]
		if state == nil || !state.pending {
			continue
		}

		notBefore := state.readyAt
		if state.degradedUntil.After(notBefore) {
			notBefore = state.degradedUntil
		}
		if notBefore.After(now) {
			if earliest.IsZero() || notBefore.Before(earliest) {
				earliest = notBefore
			}
			continue
		}

		d.queue = append(d.queue[:idx], d.queue[idx+1:]...)
		state.pending = false
		state.running = true
		reason := state.pendingReason
		state.pendingReason = ""
		return kind, reason, time.Time{}, true
	}

	return "", "", earliest, false
}

func (d *reconcileDriver) waitForWork(waitUntil time.Time) bool {
	if waitUntil.IsZero() {
		select {
		case <-d.notifyCh:
			return true
		case <-d.workerCtx.Done():
			return false
		}
	}

	delay := time.Until(waitUntil)
	if delay <= 0 {
		return true
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-d.notifyCh:
		return true
	case <-d.workerCtx.Done():
		return false
	}
}
