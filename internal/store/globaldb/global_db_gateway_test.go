package globaldb

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/testutil"
)

func TestGlobalDBGatewayTierKeysSurviveReopen(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve both tiers and enforce one active provider per tier [UT-161]", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		first, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(first) error = %v", err)
		}
		provider := gateway.ProviderIdentity{
			Name: "connectivity-test", InstallSource: "bundled",
			DigestConfirmed: "sha256:test", ConfirmedAt: time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC),
		}
		for _, tier := range []gateway.Tier{gateway.TierPrivate, gateway.TierPublic} {
			changed, transitionErr := first.Transition(ctx, gateway.TransitionRequest{
				Target: gateway.TargetProvider, Tier: tier, Desired: gateway.DesiredEnabled, Provider: provider,
			}, time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC))
			if transitionErr != nil || !changed {
				t.Fatalf("Transition(provider %s) = (%t, %v), want changed", tier, changed, transitionErr)
			}
			changed, transitionErr = first.Transition(ctx, gateway.TransitionRequest{
				Target: gateway.TargetSurface, Tier: tier, Surface: gateway.SurfaceOperatorUI,
				Desired: gateway.DesiredEnabled, Consent: tier == gateway.TierPublic,
			}, time.Date(2026, 8, 6, 12, 1, 0, 0, time.UTC))
			if transitionErr != nil || !changed {
				t.Fatalf("Transition(surface %s) = (%t, %v), want changed", tier, changed, transitionErr)
			}
		}
		if err := first.Close(ctx); err != nil {
			t.Fatalf("Close(first) error = %v", err)
		}

		reopened, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopened) error = %v", err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(ctx); err != nil {
				t.Errorf("Close(reopened) error = %v", err)
			}
		})
		snapshot, err := reopened.Snapshot(ctx)
		if err != nil {
			t.Fatalf("Snapshot() error = %v", err)
		}
		if len(snapshot.Providers) != 2 || len(snapshot.Surfaces) != 2 {
			t.Fatalf("reopened snapshot = %#v, want two provider and two surface tier keys", snapshot)
		}
		for _, tier := range []gateway.Tier{gateway.TierPrivate, gateway.TierPublic} {
			assertGatewayTierKey(t, snapshot, tier)
		}

		_, err = reopened.Transition(ctx, gateway.TransitionRequest{
			Target: gateway.TargetProvider, Tier: gateway.TierPrivate, Desired: gateway.DesiredEnabled,
			Provider: gateway.ProviderIdentity{Name: "second-provider", InstallSource: "marketplace"},
		}, time.Date(2026, 8, 6, 12, 2, 0, 0, time.UTC))
		if !errors.Is(err, gateway.ErrTierProviderConflict) {
			t.Fatalf("Transition(second provider) error = %v, want ErrTierProviderConflict", err)
		}

		for _, table := range []string{
			"gateway_device_sessions",
			"gateway_providers",
			"gateway_provider_activations",
			"gateway_surface_exposure",
		} {
			columns, columnsErr := tableColumns(ctx, reopened.db, table)
			if columnsErr != nil {
				t.Fatalf("tableColumns(%q) error = %v", table, columnsErr)
			}
			if _, exists := columns["workspace_id"]; exists {
				t.Fatalf("operator-global table %q contains workspace_id", table)
			}
		}
		var indexSQL string
		if err := reopened.db.QueryRowContext(
			ctx,
			`SELECT sql FROM sqlite_master WHERE type = 'index' AND name = 'gateway_active_provider_per_tier'`,
		).Scan(&indexSQL); err != nil {
			t.Fatalf("query partial gateway provider index: %v", err)
		}
		if !strings.Contains(strings.ToLower(indexSQL), "where desired_state = 'enabled'") {
			t.Fatalf("gateway provider index = %q, want enabled-only partial predicate", indexSQL)
		}
	})
}

func assertGatewayTierKey(t *testing.T, snapshot gateway.Snapshot, tier gateway.Tier) {
	t.Helper()
	providerFound := false
	for _, provider := range snapshot.Providers {
		if provider.ProviderName == "connectivity-test" && provider.Tier == tier &&
			provider.Desired == gateway.DesiredEnabled && provider.Generation == 1 {
			providerFound = true
		}
	}
	if !providerFound {
		t.Fatalf("snapshot providers = %#v, want enabled connectivity-test on %s", snapshot.Providers, tier)
	}
	surfaceFound := false
	for _, surface := range snapshot.Surfaces {
		if surface.Surface == gateway.SurfaceOperatorUI && surface.Tier == tier &&
			surface.Desired == gateway.DesiredEnabled && surface.Generation == 1 {
			surfaceFound = true
		}
	}
	if !surfaceFound {
		t.Fatalf("snapshot surfaces = %#v, want enabled operator UI on %s", snapshot.Surfaces, tier)
	}
}
