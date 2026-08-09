package gateway

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
)

func TestStatusProjection(t *testing.T) {
	t.Parallel()

	t.Run("Should project tiers surfaces addresses devices and provider health [UT-022]", func(t *testing.T) {
		t.Parallel()
		snapshot := enabledPrivateSnapshot()
		snapshot.Providers[0].Observed = ProviderUp
		status := projectStatus(snapshot, []RuntimeTier{{
			Tier: TierPrivate, Bound: netip.MustParseAddrPort("127.0.0.1:43210"),
			Endpoints: []AdvertisedEndpoint{{
				URL: "https://gateway.example.test", Scheme: "https", Stability: EndpointStable,
			}},
			Surfaces: []Surface{SurfaceOperatorUI}, Advertised: true,
		}}, true, nil)
		if len(status.Tiers) != 2 || len(status.Surfaces) != 1 || len(status.Providers) != 1 ||
			len(status.Addresses) != 1 || len(status.Devices) != 1 {
			t.Fatalf("status projection incomplete: %#v", status)
		}
		if status.Providers[0].Health != HealthHealthy || !status.Addresses[0].Live {
			t.Fatalf("provider/address status = %#v/%#v", status.Providers[0], status.Addresses[0])
		}
		if status.Tiers[0].ListenerAddress != "127.0.0.1:43210" {
			t.Fatalf("private listener = %q, want resolved loopback address", status.Tiers[0].ListenerAddress)
		}
	})

	t.Run("Should publish the resolved listener before provider advertisement [IT-016]", func(t *testing.T) {
		t.Parallel()
		status := projectStatus(enabledPrivateSnapshot(), []RuntimeTier{{
			Tier: TierPrivate, Bound: netip.MustParseAddrPort("127.0.0.1:43123"), Advertised: false,
		}}, true, nil)
		if status.Tiers[0].ListenerAddress != "127.0.0.1:43123" || status.Tiers[0].Advertised {
			t.Fatalf("private tier = %#v, want bound listener before advertisement", status.Tiers[0])
		}
		if len(status.Addresses) != 0 {
			t.Fatalf("provider addresses = %#v, want none before advertisement", status.Addresses)
		}
	})

	t.Run("Should distinguish desired from observed while reconciliation is pending [UT-023]", func(t *testing.T) {
		t.Parallel()
		status := projectStatus(enabledPrivateSnapshot(), nil, true, nil)
		if status.Surfaces[0].Desired != DesiredEnabled || status.Surfaces[0].Observed != SurfaceOff {
			t.Fatalf("surface = %#v, want enabled/off", status.Surfaces[0])
		}
		if status.Providers[0].Desired != DesiredEnabled || status.Providers[0].Observed != ProviderDown {
			t.Fatalf("provider = %#v, want enabled/down", status.Providers[0])
		}
	})

	t.Run("Should return explicit empty collections for a fresh gateway [UT-024]", func(t *testing.T) {
		t.Parallel()
		status := projectStatus(Snapshot{}, nil, false, nil)
		if status.Devices == nil || status.Providers == nil || status.Bindings == nil || status.Addresses == nil {
			t.Fatalf("fresh status contains nil collections: %#v", status)
		}
		if len(status.Devices)+len(status.Providers)+len(status.Bindings)+len(status.Addresses) != 0 {
			t.Fatalf("fresh status collections are not empty: %#v", status)
		}
	})

	t.Run("Should report degraded cause without a stale live address [UT-025]", func(t *testing.T) {
		t.Parallel()
		snapshot := enabledPrivateSnapshot()
		snapshot.Providers[0].Observed = ProviderDegraded
		snapshot.Providers[0].LastError = "verification failed"
		status := projectStatus(snapshot, []RuntimeTier{{
			Tier: TierPrivate,
			Endpoints: []AdvertisedEndpoint{{
				URL: "https://stale.example.test", Scheme: "https", Stability: EndpointStable,
			}},
			Advertised: false,
		}}, true, nil)
		if status.Providers[0].Health != HealthDegraded || status.Providers[0].Cause != "verification failed" {
			t.Fatalf("provider = %#v, want degraded cause", status.Providers[0])
		}
		if len(status.Addresses) != 0 {
			t.Fatalf("addresses = %#v, want no stale live address", status.Addresses)
		}
	})

	t.Run("Should redact sensitive material at the status projection boundary", func(t *testing.T) {
		t.Parallel()
		const secret = "sk-gateway-status-secret"
		snapshot := enabledPrivateSnapshot()
		snapshot.Providers[0].LastError = "api_key=" + secret
		status := projectStatus(snapshot, []RuntimeTier{{
			Tier: TierPrivate,
			Endpoints: []AdvertisedEndpoint{{
				URL:    "https://gateway.example.test/api_key=" + secret,
				Scheme: "https", Stability: EndpointStable,
			}},
			Advertised: true,
		}}, true, nil)
		if strings.Contains(status.Addresses[0].Address, secret) ||
			strings.Contains(status.Providers[0].Cause, secret) {
			t.Fatalf("status projection leaked provider credential: %#v", status)
		}
	})

	t.Run("Should project private and public tiers independently [UT-026]", func(t *testing.T) {
		t.Parallel()
		snapshot := enabledPrivateSnapshot()
		snapshot.Providers = append(snapshot.Providers, ProviderActivation{
			ProviderName: "connectivity-test", Tier: TierPublic,
			Desired: DesiredDisabled, Observed: ProviderDown, Generation: 2,
		})
		status := projectStatus(snapshot, []RuntimeTier{{Tier: TierPrivate, Advertised: true}}, true, nil)
		if status.Tiers[0].Tier != TierPrivate || !status.Tiers[0].Advertised {
			t.Fatalf("private tier = %#v, want advertised", status.Tiers[0])
		}
		if status.Tiers[1].Tier != TierPublic || status.Tiers[1].Advertised ||
			status.Tiers[1].Desired != DesiredDisabled {
			t.Fatalf("public tier = %#v, want independent disabled state", status.Tiers[1])
		}
	})

	t.Run("Should prefer the enabled provider after a provider switch", func(t *testing.T) {
		t.Parallel()
		status := projectStatus(Snapshot{Providers: []ProviderActivation{
			{
				ProviderName: "connectivity-a", Tier: TierPrivate,
				Desired: DesiredDisabled, Observed: ProviderDown, Generation: 2,
			},
			{
				ProviderName: "connectivity-b", Tier: TierPrivate,
				Desired: DesiredEnabled, Observed: ProviderUp, Generation: 1,
			},
		}}, nil, true, nil)
		if status.Tiers[0].Desired != DesiredEnabled || status.Tiers[0].Observed != string(ProviderUp) {
			t.Fatalf("private tier = %#v, want enabled/up from the active provider", status.Tiers[0])
		}
	})

	t.Run("Should leave no surface or consent after an abandoned public UI confirmation [UT-027]", func(t *testing.T) {
		t.Parallel()
		store := &testStore{snapshot: Snapshot{
			Providers: []ProviderActivation{{
				ProviderName: "connectivity-test", Tier: TierPublic,
				Desired: DesiredEnabled, Observed: ProviderDown, Generation: 1,
			}},
			Devices: []DeviceSession{{ID: "dev_test"}},
		}}
		policy := newTestPolicy(t, store, newTestEffects(nil), true)
		_, err := policy.Transition(context.Background(), TransitionRequest{
			Target: TargetSurface, Tier: TierPublic, Surface: SurfaceOperatorUI,
			Desired: DesiredEnabled, Consent: false,
		})
		if !errors.Is(err, ErrConsentRequired) {
			t.Fatalf("Transition() error = %v, want ErrConsentRequired", err)
		}
		snapshot, snapshotErr := store.Snapshot(context.Background())
		if snapshotErr != nil {
			t.Fatalf("Snapshot() error = %v", snapshotErr)
		}
		if len(snapshot.Surfaces) != 0 {
			t.Fatalf("surfaces = %#v, want no partial consent row", snapshot.Surfaces)
		}
	})
}
