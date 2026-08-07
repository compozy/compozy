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

// ProviderPreflight validates live provider authority before existing reachability is disturbed.
type ProviderPreflight interface {
	PreflightProvider(context.Context, ProviderActivation) error
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
	store          Store
	effects        EffectDriver
	mu             sync.Mutex
	runtime        map[Tier]*runtimeTierState
	recoveryCtx    context.Context
	recoveryCancel context.CancelFunc
	recoveries     map[Tier]*providerRecovery
	recoveryMin    time.Duration
	recoveryMax    time.Duration
	providerStates ProviderStateReporter
	closed         bool
}

// NewReconciler constructs a serialized, generation-fenced reconciler.
func NewReconciler(store Store, effects EffectDriver, options ...ReconcilerOption) *Reconciler {
	if effects == nil {
		effects = refusingEffects{}
	}
	recoveryCtx, recoveryCancel := context.WithCancel(context.Background())
	reconciler := &Reconciler{
		store: store, effects: effects, runtime: map[Tier]*runtimeTierState{},
		recoveryCtx: recoveryCtx, recoveryCancel: recoveryCancel,
		recoveries:  make(map[Tier]*providerRecovery),
		recoveryMin: defaultProviderRecoveryMinimum, recoveryMax: defaultProviderRecoveryMaximum,
	}
	for _, option := range options {
		if option != nil {
			option(reconciler)
		}
	}
	if source, ok := effects.(ProviderDegradationSource); ok {
		source.SetProviderDegradationHandler(reconciler.handleProviderDegraded)
	}
	if reporter, ok := effects.(ProviderStateReporter); ok {
		reconciler.providerStates = reporter
	}
	return reconciler
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
		runtimeCopy.Endpoints = append([]AdvertisedEndpoint(nil), state.Endpoints...)
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
		r.cancelRecoveryLocked(tier)
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
	if preflight, ok := r.effects.(ProviderPreflight); ok {
		if err := preflight.PreflightProvider(ctx, provider); err != nil {
			return err
		}
	}
	state := r.runtimeState(plan.Tier)
	hadAdvertisement := state.Advertised
	previousRuntime := state.RuntimeTier
	previousRuntime.Endpoints = append([]AdvertisedEndpoint(nil), state.Endpoints...)
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
				Tier: plan.Tier, Endpoints: append([]AdvertisedEndpoint(nil), previousRuntime.Endpoints...),
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
	if err := r.markProviderEstablishing(ctx, plan, state, hadAdvertisement); err != nil {
		return err
	}
	providerCopy := provider
	state.provider = &providerCopy
	reachability, err := r.effects.Establish(ctx, provider, bound)
	if err != nil {
		return errors.Join(
			r.markDegraded(ctx, plan, err),
			r.compensateTier(ctx, plan.Tier, provider, state, false),
		)
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
	if err := r.effects.Advertise(ctx, reachability, surfaces); err != nil {
		return errors.Join(r.markDegraded(ctx, plan, err), r.compensateTier(ctx, plan.Tier, provider, state, true))
	}
	state.Endpoints = append([]AdvertisedEndpoint(nil), reachability.Endpoints...)
	state.Surfaces = append([]Surface(nil), surfaces...)
	state.Advertised = true
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
		if provider == nil || r.providerStates == nil {
			return nil
		}
		current, err := r.currentProviderActivation(ctx, *provider)
		if err != nil {
			return err
		}
		return r.recordProviderState(ctx, current, ProviderDown, "")
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
	state.Endpoints = nil
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

func (r *Reconciler) handleProviderDegraded(
	ctx context.Context,
	activation ProviderActivation,
	cause error,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.runtimeState(activation.Tier)
	if state.provider == nil || state.provider.ProviderName != activation.ProviderName ||
		state.provider.Generation != activation.Generation {
		return ErrStaleGeneration
	}
	state.accepting = false
	cleanupErr := errors.Join(
		r.withdrawTier(ctx, activation.Tier, state),
		r.unbindTier(ctx, activation.Tier, state),
	)
	state.provider = nil
	snapshot, err := r.store.Snapshot(ctx)
	if err != nil {
		return errors.Join(cause, cleanupErr, err)
	}
	plan := planForTier(planSnapshot(snapshot), activation.Tier)
	if plan.Provider == nil || plan.Provider.ProviderName != activation.ProviderName ||
		plan.Provider.Generation != activation.Generation {
		return errors.Join(cause, cleanupErr)
	}
	safeCause := diagnostics.RedactAndBound(cause.Error(), maxPersistedGatewayDiagnosticBytes)
	if err := r.markObserved(ctx, plan, ProviderDegraded, SurfaceOff, safeCause); err != nil {
		return errors.Join(cause, cleanupErr, err)
	}
	r.scheduleRecoveryLocked(activation)
	return cleanupErr
}

func planForTier(plan ExposurePlan, tier Tier) TierPlan {
	for _, candidate := range plan.Tiers {
		if candidate.Tier == tier {
			return candidate
		}
	}
	return TierPlan{Tier: tier}
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
	r.recoveryCancel()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	for tier := range r.recoveries {
		r.cancelRecoveryLocked(tier)
	}
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
