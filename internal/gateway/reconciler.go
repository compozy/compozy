package gateway

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"sync"
	"time"
)

// EffectDriver performs daemon-owned and provider-owned external effects.
type EffectDriver interface {
	Bind(context.Context, Tier) (netip.AddrPort, error)
	Unbind(context.Context, Tier) error
	Establish(context.Context, ProviderActivation, netip.AddrPort) (Reachability, error)
	Teardown(context.Context, ProviderActivation) error
	Verify(context.Context, Reachability) error
	Advertise(context.Context, Reachability, []Surface) error
	Withdraw(context.Context, Tier) error
}

type refusingEffects struct{}

func (refusingEffects) Bind(context.Context, Tier) (netip.AddrPort, error) {
	return netip.AddrPort{}, refusalError(
		"gateway listener effects are unavailable",
		"install the daemon gateway listener adapter before enabling reachability",
	)
}
func (refusingEffects) Unbind(context.Context, Tier) error { return nil }
func (refusingEffects) Establish(context.Context, ProviderActivation, netip.AddrPort) (Reachability, error) {
	return Reachability{}, refusalError(
		"gateway provider effects are unavailable",
		"install a connectivity provider before enabling reachability",
	)
}
func (refusingEffects) Teardown(context.Context, ProviderActivation) error { return nil }
func (refusingEffects) Verify(context.Context, Reachability) error         { return nil }
func (refusingEffects) Advertise(context.Context, Reachability, []Surface) error {
	return nil
}
func (refusingEffects) Withdraw(context.Context, Tier) error { return nil }

type runtimeTierState struct {
	RuntimeTier
	inFlight int
	provider *ProviderActivation
}

// Reconciler drives durable desired state through ordered external effects.
type Reconciler struct {
	store   Store
	effects EffectDriver
	mu      sync.Mutex
	runtime map[Tier]*runtimeTierState
}

// NewReconciler constructs a serialized, generation-fenced reconciler.
func NewReconciler(store Store, effects EffectDriver) *Reconciler {
	if effects == nil {
		effects = refusingEffects{}
	}
	return &Reconciler{
		store:   store,
		effects: effects,
		runtime: map[Tier]*runtimeTierState{},
	}
}

// PublishListener records a daemon-resolved loopback listener address.
func (r *Reconciler) PublishListener(tier Tier, addr netip.AddrPort) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.runtimeState(tier)
	state.Bound = addr
}

// Runtime returns a stable copy for status projection.
func (r *Reconciler) Runtime() []RuntimeTier {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]RuntimeTier, 0, len(r.runtime))
	for _, tier := range []Tier{TierPrivate, TierPublic} {
		state, ok := r.runtime[tier]
		if !ok {
			continue
		}
		runtimeCopy := state.RuntimeTier
		runtimeCopy.Addresses = append([]string(nil), state.Addresses...)
		runtimeCopy.Surfaces = append([]Surface(nil), state.Surfaces...)
		result = append(result, runtimeCopy)
	}
	return result
}

// Acquire admits a request only while its surface is currently advertised.
func (r *Reconciler) Acquire(tier Tier, surface Surface) (func(), error) {
	r.mu.Lock()
	state, ok := r.runtime[tier]
	if !ok || !state.Advertised || !slices.Contains(state.Surfaces, surface) {
		r.mu.Unlock()
		return nil, refusalError(
			"the requested gateway surface is not reachable",
			"enable the surface and wait for gateway status to report it on",
		)
	}
	state.inFlight++
	r.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			r.mu.Lock()
			defer r.mu.Unlock()
			if current := r.runtime[tier]; current != nil && current.inFlight > 0 {
				current.inFlight--
			}
		})
	}, nil
}

// Reconcile applies each tier independently in deterministic order.
func (r *Reconciler) Reconcile(ctx context.Context, plan ExposurePlan) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	byTier := map[Tier]TierPlan{}
	for _, tierPlan := range plan.Tiers {
		byTier[tierPlan.Tier] = tierPlan
	}
	var errs []error
	for _, tier := range []Tier{TierPrivate, TierPublic} {
		tierPlan := byTier[tier]
		tierPlan.Tier = tier
		var err error
		if tierPlan.Enabled {
			err = r.enableTier(ctx, tierPlan)
		} else {
			err = r.disableTier(ctx, tierPlan)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("gateway: reconcile %s tier: %w", tier, err))
		}
	}
	return errors.Join(errs...)
}

func (r *Reconciler) enableTier(ctx context.Context, plan TierPlan) (err error) {
	provider := *plan.Provider
	surfaces := surfaceNames(plan.Surfaces)
	bound, err := r.effects.Bind(ctx, plan.Tier)
	if err != nil {
		return errors.Join(err, r.effects.Unbind(ctx, plan.Tier))
	}
	compensate := func(includeWithdraw bool) error {
		var errs []error
		if includeWithdraw {
			errs = append(errs, r.effects.Withdraw(ctx, plan.Tier))
		}
		errs = append(errs, r.effects.Teardown(ctx, provider), r.effects.Unbind(ctx, plan.Tier))
		return errors.Join(errs...)
	}
	if err := r.assertCurrent(ctx, plan); err != nil {
		return errors.Join(err, r.effects.Unbind(ctx, plan.Tier))
	}
	reachability, err := r.effects.Establish(ctx, provider, bound)
	if err != nil {
		return errors.Join(err, compensate(false))
	}
	if err := r.assertCurrent(ctx, plan); err != nil {
		return errors.Join(err, compensate(false))
	}
	if err := r.effects.Verify(ctx, reachability); err != nil {
		return errors.Join(r.markDegraded(ctx, plan, err), compensate(false))
	}
	if err := r.assertCurrent(ctx, plan); err != nil {
		return errors.Join(err, compensate(false))
	}
	if err := r.effects.Advertise(ctx, reachability, surfaces); err != nil {
		return errors.Join(r.markDegraded(ctx, plan, err), compensate(true))
	}
	if err := r.assertCurrent(ctx, plan); err != nil {
		return errors.Join(err, compensate(true))
	}
	if err := r.markObserved(ctx, plan, ProviderUp, SurfaceOn, ""); err != nil {
		return errors.Join(err, compensate(true))
	}
	state := r.runtimeState(plan.Tier)
	state.Bound = bound
	state.Addresses = append([]string(nil), reachability.Addresses...)
	state.Surfaces = append([]Surface(nil), surfaces...)
	state.Advertised = true
	providerCopy := provider
	state.provider = &providerCopy
	return nil
}

func (r *Reconciler) disableTier(ctx context.Context, plan TierPlan) error {
	state := r.runtimeState(plan.Tier)
	state.Advertised = false
	state.Addresses = nil
	state.Surfaces = nil
	var errs []error
	errs = append(errs, r.effects.Withdraw(ctx, plan.Tier))
	provider := plan.Provider
	if provider == nil {
		provider = state.provider
	}
	if provider != nil {
		errs = append(errs, r.effects.Teardown(ctx, *provider))
	}
	errs = append(errs, r.effects.Unbind(ctx, plan.Tier))
	state.Bound = netip.AddrPort{}
	state.provider = nil
	if cleanupErr := errors.Join(errs...); cleanupErr != nil {
		return cleanupErr
	}
	if plan.Provider == nil {
		return nil
	}
	return r.markObserved(ctx, plan, ProviderDown, SurfaceOff, "")
}

func (r *Reconciler) assertCurrent(ctx context.Context, plan TierPlan) error {
	snapshot, err := r.store.Snapshot(ctx)
	if err != nil {
		return err
	}
	if plan.Provider == nil || !providerGenerationCurrent(snapshot, *plan.Provider) {
		return ErrStaleGeneration
	}
	for _, surface := range plan.Surfaces {
		if !surfaceGenerationCurrent(snapshot, surface) {
			return ErrStaleGeneration
		}
	}
	return nil
}

func (r *Reconciler) markObserved(
	ctx context.Context,
	plan TierPlan,
	providerState ProviderObservedState,
	surfaceState SurfaceObservedState,
	cause string,
) error {
	now := time.Now().UTC()
	updated, err := r.store.SetObserved(ctx, plan, providerState, surfaceState, now, cause)
	if err != nil {
		return err
	}
	if !updated {
		return ErrStaleGeneration
	}
	return nil
}

func (r *Reconciler) markDegraded(ctx context.Context, plan TierPlan, cause error) error {
	markErr := r.markObserved(ctx, plan, ProviderDegraded, SurfaceOff, cause.Error())
	return errors.Join(cause, markErr)
}

func (r *Reconciler) runtimeState(tier Tier) *runtimeTierState {
	state := r.runtime[tier]
	if state == nil {
		state = &runtimeTierState{RuntimeTier: RuntimeTier{Tier: tier}}
		r.runtime[tier] = state
	}
	return state
}

// Close removes reachability in reverse effect order for both tiers.
func (r *Reconciler) Close(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var errs []error
	for _, tier := range []Tier{TierPublic, TierPrivate} {
		state := r.runtimeState(tier)
		state.Advertised = false
		state.Addresses = nil
		state.Surfaces = nil
		errs = append(errs, r.effects.Withdraw(ctx, tier))
		if state.provider != nil {
			errs = append(errs, r.effects.Teardown(ctx, *state.provider))
		}
		errs = append(errs, r.effects.Unbind(ctx, tier))
		state.Bound = netip.AddrPort{}
		state.provider = nil
	}
	return errors.Join(errs...)
}

func providerGenerationCurrent(snapshot Snapshot, expected ProviderActivation) bool {
	for _, provider := range snapshot.Providers {
		if provider.ProviderName == expected.ProviderName && provider.Tier == expected.Tier {
			return provider.Generation == expected.Generation && provider.Desired == DesiredEnabled
		}
	}
	return false
}

func surfaceGenerationCurrent(snapshot Snapshot, expected SurfaceExposure) bool {
	for _, surface := range snapshot.Surfaces {
		if surface.Surface == expected.Surface && surface.Tier == expected.Tier {
			return surface.Generation == expected.Generation && surface.Desired == DesiredEnabled
		}
	}
	return false
}

func surfaceNames(exposures []SurfaceExposure) []Surface {
	result := make([]Surface, 0, len(exposures))
	for _, exposure := range exposures {
		result = append(result, exposure.Surface)
	}
	return result
}
