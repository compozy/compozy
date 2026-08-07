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

		changed, err := reopened.Transition(ctx, gateway.TransitionRequest{
			Target: gateway.TargetProvider, Tier: gateway.TierPrivate,
			Desired: gateway.DesiredDisabled, ExpectedGeneration: 1,
			Provider: gateway.ProviderIdentity{Name: "connectivity-test"},
		}, time.Date(2026, 8, 6, 12, 2, 0, 0, time.UTC))
		if err != nil || !changed {
			t.Fatalf("Transition(disable provider) = (%t, %v), want changed", changed, err)
		}
		disabledSnapshot, err := reopened.Snapshot(ctx)
		if err != nil {
			t.Fatalf("Snapshot(disabled provider) error = %v", err)
		}
		assertGatewayProviderState(
			t,
			disabledSnapshot,
			gateway.TierPrivate,
			gateway.DesiredDisabled,
			2,
		)

		_, err = reopened.Transition(ctx, gateway.TransitionRequest{
			Target: gateway.TargetProvider, Tier: gateway.TierPublic, Desired: gateway.DesiredEnabled,
			Provider: gateway.ProviderIdentity{Name: "second-provider", InstallSource: "marketplace"},
		}, time.Date(2026, 8, 6, 12, 3, 0, 0, time.UTC))
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

func assertGatewayProviderState(
	t *testing.T,
	snapshot gateway.Snapshot,
	tier gateway.Tier,
	desired gateway.DesiredState,
	generation uint64,
) {
	t.Helper()
	for _, provider := range snapshot.Providers {
		if provider.ProviderName == "connectivity-test" && provider.Tier == tier {
			if provider.Desired != desired || provider.Generation != generation {
				t.Fatalf(
					"provider %s state = (%s, %d), want (%s, %d)",
					tier,
					provider.Desired,
					provider.Generation,
					desired,
					generation,
				)
			}
			return
		}
	}
	t.Fatalf("snapshot providers = %#v, want connectivity-test on %s", snapshot.Providers, tier)
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

func TestGlobalDBGatewayDeviceLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should persist hash-only credentials and order the complete device inventory", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		database := openFreshTestGlobalDB(t)
		createdAt := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		first := gateway.StoredDeviceSession{
			Session: gateway.DeviceSession{
				ID: "device-first", Name: "First", ActorKind: gateway.ActorKindOperatorDevice,
				PairingOrigin: "local", CreatedAt: createdAt,
			},
			TokenHash: strings.Repeat("a", 64),
		}
		second := gateway.StoredDeviceSession{
			Session: gateway.DeviceSession{
				ID: "device-second", Name: "Second", ActorKind: gateway.ActorKindCLIProfile,
				PairingOrigin: "private", CreatedAt: createdAt.Add(time.Minute),
			},
			TokenHash: strings.Repeat("b", 64),
		}
		for _, record := range []gateway.StoredDeviceSession{first, second} {
			if err := database.CreateDevice(ctx, record); err != nil {
				t.Fatalf("CreateDevice(%q) error = %v", record.Session.ID, err)
			}
		}
		seenAt := createdAt.Add(2 * time.Minute)
		found, err := database.FindDeviceByTokenHash(ctx, first.TokenHash, seenAt)
		if err != nil {
			t.Fatalf("FindDeviceByTokenHash() error = %v", err)
		}
		if found.Session.ID != first.Session.ID ||
			found.TokenHash != first.TokenHash ||
			found.Session.LastSeenAt != seenAt {
			t.Fatalf("FindDeviceByTokenHash() = %#v", found)
		}
		devices, err := database.ListDevices(ctx)
		if err != nil {
			t.Fatalf("ListDevices() error = %v", err)
		}
		if len(devices) != 2 || devices[0].ID != first.Session.ID || devices[1].ID != second.Session.ID {
			t.Fatalf("ListDevices() = %#v, want last-seen ordering", devices)
		}

		duplicate := second
		duplicate.Session.ID = "device-duplicate-hash"
		duplicate.TokenHash = first.TokenHash
		if err := database.CreateDevice(ctx, duplicate); err == nil {
			t.Fatal("CreateDevice(duplicate token hash) error = nil, want uniqueness failure")
		}
	})

	t.Run("Should fence mutations with the actor epoch and revoke idempotently", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		database := openFreshTestGlobalDB(t)
		createdAt := time.Date(2026, 8, 6, 13, 0, 0, 0, time.UTC)
		actor := gateway.StoredDeviceSession{
			Session: gateway.DeviceSession{
				ID: "actor-device", Name: "Actor", ActorKind: gateway.ActorKindOperatorDevice,
				PairingOrigin: "local", CreatedAt: createdAt,
			},
			TokenHash: strings.Repeat("c", 64),
		}
		target := gateway.StoredDeviceSession{
			Session: gateway.DeviceSession{
				ID: "target-device", Name: "Target", ActorKind: gateway.ActorKindCLIProfile,
				PairingOrigin: "private", CreatedAt: createdAt.Add(time.Minute),
			},
			TokenHash: strings.Repeat("d", 64),
		}
		for _, record := range []gateway.StoredDeviceSession{actor, target} {
			if err := database.CreateDevice(ctx, record); err != nil {
				t.Fatalf("CreateDevice(%q) error = %v", record.Session.ID, err)
			}
		}
		renamed, err := database.RenameDeviceForActor(
			ctx,
			actor.Session.ID,
			actor.Session.RevokeEpoch,
			target.Session.ID,
			"Renamed",
		)
		if err != nil || renamed.Name != "Renamed" {
			t.Fatalf("RenameDeviceForActor() = (%#v, %v)", renamed, err)
		}

		revoked, changed, err := database.RevokeDevice(ctx, actor.Session.ID, createdAt.Add(2*time.Minute))
		if err != nil || !changed || revoked.RevokeEpoch != 1 || revoked.RevokedAt.IsZero() {
			t.Fatalf("RevokeDevice(first) = (%#v, %t, %v)", revoked, changed, err)
		}
		revokedAgain, changedAgain, err := database.RevokeDevice(
			ctx,
			actor.Session.ID,
			createdAt.Add(3*time.Minute),
		)
		if err != nil || changedAgain || revokedAgain.RevokeEpoch != 1 || revokedAgain.RevokedAt != revoked.RevokedAt {
			t.Fatalf("RevokeDevice(second) = (%#v, %t, %v)", revokedAgain, changedAgain, err)
		}
		if err := database.RevalidateDeviceEpoch(
			ctx,
			actor.Session.ID,
			actor.Session.RevokeEpoch,
		); !errors.Is(err, gateway.ErrDeviceRevoked) {
			t.Fatalf("RevalidateDeviceEpoch() error = %v, want ErrDeviceRevoked", err)
		}
		_, err = database.RenameDeviceForActor(
			ctx,
			actor.Session.ID,
			actor.Session.RevokeEpoch,
			target.Session.ID,
			"Must not commit",
		)
		if !errors.Is(err, gateway.ErrDeviceRevoked) {
			t.Fatalf("RenameDeviceForActor(stale actor) error = %v, want ErrDeviceRevoked", err)
		}
		unchanged, err := database.GetDevice(ctx, target.Session.ID)
		if err != nil {
			t.Fatalf("GetDevice(target) error = %v", err)
		}
		if unchanged.Name != "Renamed" {
			t.Fatalf("stale actor committed target name %q", unchanged.Name)
		}
	})
}
