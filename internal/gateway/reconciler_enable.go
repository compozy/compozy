package gateway

import (
	"context"
	"errors"
	"net/netip"
)

type tierRuntimeSnapshot struct {
	advertised bool
	runtime    RuntimeTier
	provider   *ProviderActivation
	accepting  bool
}

func snapshotRuntimeTier(state *runtimeTierState) tierRuntimeSnapshot {
	runtimeState := state.RuntimeTier
	runtimeState.Endpoints = append([]AdvertisedEndpoint(nil), state.Endpoints...)
	runtimeState.Surfaces = append([]Surface(nil), state.Surfaces...)
	return tierRuntimeSnapshot{
		advertised: state.Advertised,
		runtime:    runtimeState,
		provider:   state.provider,
		accepting:  state.accepting,
	}
}

func (r *Reconciler) bindTierForEnable(
	ctx context.Context,
	tier Tier,
	state *runtimeTierState,
	previous tierRuntimeSnapshot,
) (netip.AddrPort, error) {
	bound, err := r.effects.Bind(ctx, tier)
	if err == nil {
		return bound, nil
	}
	var unbindErr error
	if bound.IsValid() {
		unbindErr = r.effects.Unbind(ctx, tier)
	}
	if !previous.advertised {
		return netip.AddrPort{}, errors.Join(err, unbindErr)
	}
	restoreErr := r.effects.Advertise(ctx, Reachability{
		Tier:      tier,
		Endpoints: append([]AdvertisedEndpoint(nil), previous.runtime.Endpoints...),
		Health:    HealthHealthy,
	}, append([]Surface(nil), previous.runtime.Surfaces...))
	state.RuntimeTier = previous.runtime
	state.provider = previous.provider
	state.accepting = restoreErr == nil && previous.accepting
	return netip.AddrPort{}, errors.Join(err, unbindErr, restoreErr)
}
