package cli

import (
	"reflect"
	"testing"

	"github.com/compozy/agh/internal/network/participation"
)

func testParticipationChannelID(channel string) *string {
	value := channel
	return &value
}

func testLiveNamedParticipationRequest(channel string) *participation.Request {
	mode := participation.ModeLive
	strategy := participation.StrategyNamed
	return &participation.Request{
		Mode:            &mode,
		ChannelStrategy: &strategy,
		ChannelID:       testParticipationChannelID(channel),
	}
}

func assertLiveNamedParticipationRequest(t testing.TB, request *participation.Request, channel string) {
	t.Helper()
	want := testLiveNamedParticipationRequest(channel)
	if !reflect.DeepEqual(request, want) {
		t.Fatalf("network_participation = %#v, want %#v", request, want)
	}
}

func assertResolvedParticipation(t testing.TB, spec *participation.Spec, want participation.Spec) {
	t.Helper()
	if spec == nil {
		t.Fatalf("resolved_network_participation = nil, want %#v", want)
		return
	}
	if err := participation.ValidateSpec(*spec); err != nil {
		t.Fatalf("resolved_network_participation = %#v, want valid snapshot: %v", spec, err)
	}
	if *spec != want {
		t.Fatalf("resolved_network_participation = %#v, want %#v", *spec, want)
	}
}

func testLiveResolvedParticipation(channel string) *participation.Spec {
	return &participation.Spec{
		Version:         participation.SpecVersion,
		Mode:            participation.ModeLive,
		WorkspaceID:     "ws-test",
		ChannelStrategy: participation.StrategyNamed,
		ChannelID:       channel,
		Source:          participation.SourceExplicitRequest,
		Bounds: participation.Bounds{
			MaxWakes:         4,
			MaxWakeWallTime:  "30s",
			MaxTotalWallTime: "2m",
			MaxInputTokens:   4096,
			MaxOutputTokens:  4096,
			MaxWakeDepth:     4,
			CoalesceWindow:   "250ms",
		},
	}
}
