package testutil

import (
	"context"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store/globaldb"
)

func AssertResolvedParticipationChannel(
	t testing.TB,
	spec *participation.Spec,
	wantWorkspace string,
	wantChannel string,
	wantSource participation.Source,
) {
	t.Helper()
	if spec == nil {
		t.Fatalf("resolved_network_participation = nil, want channel %q", wantChannel)
		return
	}
	if err := participation.ValidateSpec(*spec); err != nil {
		t.Fatalf("resolved_network_participation = %#v, want valid snapshot: %v", spec, err)
	}
	if got, want := spec.Version, participation.SpecVersion; got != want {
		t.Fatalf("resolved_network_participation.version = %q, want %q", got, want)
	}
	if got, want := spec.Mode, participation.ModeLive; got != want {
		t.Fatalf("resolved_network_participation.mode = %q, want %q", got, want)
	}
	if got, want := strings.TrimSpace(spec.WorkspaceID), strings.TrimSpace(wantWorkspace); got != want {
		t.Fatalf("resolved_network_participation.workspace_id = %q, want %q", got, want)
	}
	if got, want := strings.TrimSpace(spec.ChannelID), strings.TrimSpace(wantChannel); got != want {
		t.Fatalf("resolved_network_participation.channel_id = %q, want %q", got, want)
	}
	if got, want := spec.Source, wantSource; got != want {
		t.Fatalf("resolved_network_participation.source = %q, want %q", got, want)
	}
	if got, want := spec.ChannelStrategy, participation.StrategyNamed; got != want {
		t.Fatalf("resolved_network_participation.channel_strategy = %q, want %q", got, want)
	}
	if got, want := spec.Bounds, IntegrationParticipationBounds(); got != want {
		t.Fatalf("resolved_network_participation.bounds = %#v, want %#v", got, want)
	}
}

func AssertLocalResolvedParticipation(t testing.TB, spec *participation.Spec) {
	t.Helper()
	if spec == nil {
		t.Fatal("resolved_network_participation = nil, want explicit Local participation")
		return
	}
	if err := participation.ValidateSpec(*spec); err != nil {
		t.Fatalf("resolved_network_participation = %#v, want valid Local snapshot: %v", spec, err)
	}
	if spec.Mode != participation.ModeLocal {
		t.Fatalf("resolved_network_participation.mode = %q, want %q", spec.Mode, participation.ModeLocal)
	}
	if spec.Source != participation.SourceBuiltInLocal {
		t.Fatalf(
			"resolved_network_participation.source = %q, want %q",
			spec.Source,
			participation.SourceBuiltInLocal,
		)
	}
}

// IntegrationParticipationBounds returns the canonical bounds used by API integration runtimes.
func IntegrationParticipationBounds() participation.Bounds {
	return participation.Bounds{
		MaxWakes:         4,
		MaxWakeWallTime:  "30s",
		MaxTotalWallTime: "2m",
		MaxInputTokens:   4096,
		MaxOutputTokens:  4096,
		MaxWakeDepth:     4,
		CoalesceWindow:   "250ms",
	}
}

func NewIntegrationParticipationResolver(t testing.TB, db *globaldb.GlobalDB) participation.Resolver {
	t.Helper()

	resolver, err := participation.NewResolver(participation.ResolverOptions{
		Defaults: IntegrationParticipationBounds(),
		Limits: participation.Limits{
			MaxWakes:          16,
			MaxWakeWallTime:   "2m",
			MaxTotalWallTime:  "10m",
			MaxInputTokens:    65536,
			MaxOutputTokens:   65536,
			MaxWakeDepth:      16,
			MinCoalesceWindow: "100ms",
			MaxCoalesceWindow: "5s",
		},
		Availability: func(ctx context.Context) (bool, error) {
			state, readErr := db.GetNetworkAvailability(ctx)
			if readErr != nil {
				return false, readErr
			}
			return state.Enabled, nil
		},
		ChannelExists: func(context.Context, string, string) (bool, error) {
			return true, nil
		},
		LiveSupport: func(context.Context, participation.ResolveInput) (bool, error) {
			return true, nil
		},
	})
	if err != nil {
		t.Fatalf("participation.NewResolver() error = %v", err)
	}
	return resolver
}

func EnableIntegrationLiveNetwork(t testing.TB, registry *globaldb.GlobalDB) {
	t.Helper()

	if _, err := registry.SetNetworkAvailability(context.Background(), true, "test:enable"); err != nil {
		t.Fatalf("SetNetworkAvailability(true) error = %v", err)
	}
}
