package gateway

import (
	"errors"
	"testing"
	"time"
)

func TestGatewayService(t *testing.T) {
	t.Parallel()

	t.Run("Should make provider disable idempotent while preserving exact-name not-found", func(t *testing.T) {
		t.Parallel()

		store := &testStore{snapshot: Snapshot{Providers: []ProviderActivation{{
			ProviderName: "connectivity-test", Tier: TierPrivate,
			Desired: DesiredEnabled, Observed: ProviderUp, Generation: 1,
		}}}}
		policy := newTestPolicy(t, store, newTestEffects(nil), true)
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		devices := newDeviceServiceOnly(t, &now)
		service, err := NewService(policy, devices)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		first, err := service.DisableProvider(t.Context(), TierPrivate, "connectivity-test")
		if err != nil || !first.Changed {
			t.Fatalf("DisableProvider(first) = (%#v, %v), want changed", first, err)
		}
		second, err := service.DisableProvider(t.Context(), TierPrivate, "connectivity-test")
		if err != nil {
			t.Fatalf("DisableProvider(second) error = %v", err)
		}
		if second.Changed {
			t.Fatal("DisableProvider(second).Changed = true, want false")
		}
		if _, err := service.DisableProvider(
			t.Context(),
			TierPrivate,
			"missing",
		); !errors.Is(err, ErrProviderNotFound) {
			t.Fatalf("DisableProvider(missing) error = %v, want ErrProviderNotFound", err)
		}
	})
}
