package gateway

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
)

const maxPersistedGatewayDiagnosticBytes = 2048

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
	inFlight  atomic.Int64
	provider  *ProviderActivation
	accepting bool
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
	if !ok || !state.accepting || !state.Advertised || !slices.Contains(state.Surfaces, surface) {
		r.mu.Unlock()
		return nil, refusalError(
			"the requested gateway surface is not reachable",
			"enable the surface and wait for gateway status to report it on",
		)
	}
	state.inFlight.Add(1)
	r.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			state.inFlight.Add(-1)
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

func (r *Reconciler) enableTier(ctx context.Context, plan TierPlan) error {
	if plan.Provider == nil {
		return refusalError(
			"an enabled gateway tier requires a connectivity provider",
			"enable a trusted provider for the tier before reconciling reachability",
		)
	}
	provider := *plan.Provider
	surfaces := surfaceNames(plan.Surfaces)
	state := r.runtimeState(plan.Tier)
	hadAdvertisement := state.Advertised
	previousRuntime := state.RuntimeTier
	previousRuntime.Addresses = append([]string(nil), state.Addresses...)
	previousRuntime.Surfaces = append([]Surface(nil), state.Surfaces...)
	previousProvider := state.provider
	previousAccepting := state.accepting
	state.accepting = false
	if hadAdvertisement {
		if err := r.withdrawTier(ctx, plan.Tier, state); err != nil {
			state.accepting = previousAccepting
			return err
		}
	}
	bound, err := r.effects.Bind(ctx, plan.Tier)
	if err != nil {
		if hadAdvertisement {
			restoreErr := r.effects.Advertise(ctx, Reachability{
				Tier: plan.Tier, Addresses: append([]string(nil), previousRuntime.Addresses...),
				Health: HealthHealthy,
			}, append([]Surface(nil), previousRuntime.Surfaces...))
			state.RuntimeTier = previousRuntime
			state.provider = previousProvider
			state.accepting = restoreErr == nil && previousAccepting
			return errors.Join(err, restoreErr)
		}
		return err
	}
	state.Bound = bound
	if err := r.assertCurrent(ctx, plan); err != nil {
		return errors.Join(err, r.unbindTier(ctx, plan.Tier, state))
	}
	providerCopy := provider
	state.provider = &providerCopy
	reachability, err := r.effects.Establish(ctx, provider, bound)
	if err != nil {
		return errors.Join(err, r.compensateTier(ctx, plan.Tier, provider, state, false))
	}
	if err := r.assertCurrent(ctx, plan); err != nil {
		return errors.Join(err, r.compensateTier(ctx, plan.Tier, provider, state, false))
	}
	if err := r.effects.Verify(ctx, reachability); err != nil {
		return errors.Join(r.markDegraded(ctx, plan, err), r.compensateTier(ctx, plan.Tier, provider, state, false))
	}
	if err := r.assertCurrent(ctx, plan); err != nil {
		return errors.Join(err, r.compensateTier(ctx, plan.Tier, provider, state, false))
	}
	state.Addresses = append([]string(nil), reachability.Addresses...)
	state.Surfaces = append([]Surface(nil), surfaces...)
	state.Advertised = true
	if err := r.effects.Advertise(ctx, reachability, surfaces); err != nil {
		return errors.Join(r.markDegraded(ctx, plan, err), r.compensateTier(ctx, plan.Tier, provider, state, true))
	}
	if err := r.assertCurrent(ctx, plan); err != nil {
		return errors.Join(err, r.compensateTier(ctx, plan.Tier, provider, state, true))
	}
	if err := r.markObserved(ctx, plan, ProviderUp, SurfaceOn, ""); err != nil {
		return errors.Join(err, r.compensateTier(ctx, plan.Tier, provider, state, true))
	}
	state.accepting = true
	return nil
}

func (r *Reconciler) disableTier(ctx context.Context, plan TierPlan) error {
	state := r.runtimeState(plan.Tier)
	state.accepting = false
	var errs []error
	errs = append(errs, r.withdrawTier(ctx, plan.Tier, state))
	provider := plan.Provider
	if provider == nil {
		provider = state.provider
	}
	if provider != nil {
		errs = append(errs, r.teardownTier(ctx, *provider, state))
	}
	errs = append(errs, r.unbindTier(ctx, plan.Tier, state))
	if cleanupErr := errors.Join(errs...); cleanupErr != nil {
		return cleanupErr
	}
	if plan.Provider == nil {
		return nil
	}
	return r.markObserved(ctx, plan, ProviderDown, SurfaceOff, "")
}

func (r *Reconciler) compensateTier(
	ctx context.Context,
	tier Tier,
	provider ProviderActivation,
	state *runtimeTierState,
	includeWithdraw bool,
) error {
	state.accepting = false
	var errs []error
	if includeWithdraw {
		errs = append(errs, r.withdrawTier(ctx, tier, state))
	}
	errs = append(errs, r.teardownTier(ctx, provider, state), r.unbindTier(ctx, tier, state))
	return errors.Join(errs...)
}

func (r *Reconciler) withdrawTier(ctx context.Context, tier Tier, state *runtimeTierState) error {
	if err := r.effects.Withdraw(ctx, tier); err != nil {
		return err
	}
	state.Advertised = false
	state.Addresses = nil
	state.Surfaces = nil
	return nil
}

func (r *Reconciler) teardownTier(
	ctx context.Context,
	provider ProviderActivation,
	state *runtimeTierState,
) error {
	if err := r.effects.Teardown(ctx, provider); err != nil {
		return err
	}
	state.provider = nil
	return nil
}

func (r *Reconciler) unbindTier(ctx context.Context, tier Tier, state *runtimeTierState) error {
	if err := r.effects.Unbind(ctx, tier); err != nil {
		return err
	}
	state.Bound = netip.AddrPort{}
	return nil
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
	safeCause := diagnostics.RedactAndBound(cause.Error(), maxPersistedGatewayDiagnosticBytes)
	markErr := r.markObserved(ctx, plan, ProviderDegraded, SurfaceOff, safeCause)
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
		state.accepting = false
		errs = append(errs, r.withdrawTier(ctx, tier, state))
		if state.provider != nil {
			errs = append(errs, r.teardownTier(ctx, *state.provider, state))
		}
		errs = append(errs, r.unbindTier(ctx, tier, state))
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
