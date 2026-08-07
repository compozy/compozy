package gateway

import (
	"sort"

	"github.com/compozy/compozy/internal/diagnostics"
)

func projectStatus(
	snapshot Snapshot,
	runtime []RuntimeTier,
	enabled bool,
	refusal *Refusal,
) Status {
	status := Status{
		Enabled:   enabled,
		Tiers:     make([]TierStatus, 0, 2),
		Surfaces:  make([]SurfaceStatus, 0, len(snapshot.Surfaces)),
		Providers: make([]ProviderStatus, 0, len(snapshot.Providers)),
		Addresses: []AddressStatus{},
		Devices:   append([]DeviceSession{}, snapshot.Devices...),
		Bindings:  []IngressBindingStatus{},
		Refusal:   refusal,
	}
	runtimeByTier := make(map[Tier]RuntimeTier, len(runtime))
	for _, tierRuntime := range runtime {
		runtimeByTier[tierRuntime.Tier] = tierRuntime
		if tierRuntime.Advertised {
			for _, endpoint := range tierRuntime.Endpoints {
				status.Addresses = append(status.Addresses, AddressStatus{
					Tier: tierRuntime.Tier,
					Address: diagnostics.RedactAndBound(
						endpoint.URL,
						maxPersistedGatewayDiagnosticBytes,
					),
					Live: true,
				})
			}
		}
	}
	for _, provider := range snapshot.Providers {
		status.Providers = append(status.Providers, ProviderStatus{
			Name: provider.ProviderName, Tier: provider.Tier,
			Desired: provider.Desired, Observed: provider.Observed,
			Generation: provider.Generation,
			Health:     providerHealth(provider),
			Cause: diagnostics.RedactAndBound(
				provider.LastError,
				maxPersistedGatewayDiagnosticBytes,
			),
		})
	}
	for _, surface := range snapshot.Surfaces {
		status.Surfaces = append(status.Surfaces, SurfaceStatus{
			Surface: surface.Surface, Tier: surface.Tier,
			Desired: surface.Desired, Observed: surface.Observed,
			Generation: surface.Generation,
		})
	}
	for _, tier := range []Tier{TierPrivate, TierPublic} {
		desired, observed := tierProjection(snapshot, tier)
		listenerAddress := ""
		if bound := runtimeByTier[tier].Bound; bound.IsValid() {
			listenerAddress = bound.String()
		}
		status.Tiers = append(status.Tiers, TierStatus{
			Tier: tier, Desired: desired, Observed: observed,
			ListenerAddress: listenerAddress, Advertised: runtimeByTier[tier].Advertised,
		})
	}
	sort.Slice(status.Addresses, func(i, j int) bool {
		if status.Addresses[i].Tier != status.Addresses[j].Tier {
			return status.Addresses[i].Tier < status.Addresses[j].Tier
		}
		return status.Addresses[i].Address < status.Addresses[j].Address
	})
	return status
}

func tierProjection(snapshot Snapshot, tier Tier) (DesiredState, string) {
	for _, provider := range snapshot.Providers {
		if provider.Tier != tier || provider.Desired != DesiredEnabled {
			continue
		}
		return DesiredEnabled, string(provider.Observed)
	}
	return DesiredDisabled, string(ProviderDown)
}

func providerHealth(provider ProviderActivation) ProviderHealth {
	switch provider.Observed {
	case ProviderUp:
		return HealthHealthy
	case ProviderDegraded:
		return HealthDegraded
	default:
		return HealthDown
	}
}
