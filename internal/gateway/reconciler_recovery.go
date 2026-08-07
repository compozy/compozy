package gateway

import (
	"context"
	"time"
)

const (
	defaultProviderRecoveryMinimum = time.Second
	defaultProviderRecoveryMaximum = 30 * time.Second
)

// ReconcilerOption customizes provider recovery behavior.
type ReconcilerOption func(*Reconciler)

// WithProviderRecoveryBackoff installs bounded restart intervals.
func WithProviderRecoveryBackoff(minimum time.Duration, maximum time.Duration) ReconcilerOption {
	return func(reconciler *Reconciler) {
		if minimum > 0 {
			reconciler.recoveryMin = minimum
		}
		if maximum >= minimum && maximum > 0 {
			reconciler.recoveryMax = maximum
		}
	}
}

type providerRecovery struct {
	activation ProviderActivation
	ctx        context.Context
	cancel     context.CancelFunc
}

func (r *Reconciler) scheduleRecoveryLocked(activation ProviderActivation) {
	if r.closed {
		return
	}
	r.cancelRecoveryLocked(activation.Tier)
	ctx, cancel := context.WithCancel(r.recoveryCtx)
	recovery := &providerRecovery{activation: activation, ctx: ctx, cancel: cancel}
	r.recoveries[activation.Tier] = recovery
	go r.runProviderRecovery(recovery)
}

func (r *Reconciler) cancelRecoveryLocked(tier Tier) {
	recovery := r.recoveries[tier]
	if recovery == nil {
		return
	}
	delete(r.recoveries, tier)
	recovery.cancel()
}

func (r *Reconciler) runProviderRecovery(recovery *providerRecovery) {
	delay := r.recoveryMin
	for {
		timer := time.NewTimer(delay)
		select {
		case <-recovery.ctx.Done():
			stopRecoveryTimer(timer)
			return
		case <-timer.C:
		}
		if !r.tryProviderRecovery(recovery) {
			return
		}
		if delay < r.recoveryMax {
			delay = min(delay*2, r.recoveryMax)
		}
	}
}

func (r *Reconciler) tryProviderRecovery(recovery *providerRecovery) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.recoveries[recovery.activation.Tier] != recovery || recovery.ctx.Err() != nil {
		return false
	}
	snapshot, err := r.store.Snapshot(recovery.ctx)
	if err != nil {
		return true
	}
	plan := planForTier(planSnapshot(snapshot), recovery.activation.Tier)
	if !plan.Enabled || plan.Provider == nil ||
		plan.Provider.ProviderName != recovery.activation.ProviderName ||
		plan.Provider.Generation != recovery.activation.Generation {
		delete(r.recoveries, recovery.activation.Tier)
		return false
	}
	if err := r.enableTier(recovery.ctx, plan); err != nil {
		return true
	}
	delete(r.recoveries, recovery.activation.Tier)
	return false
}

func stopRecoveryTimer(timer *time.Timer) {
	if timer == nil || !timer.Stop() {
		return
	}
}
