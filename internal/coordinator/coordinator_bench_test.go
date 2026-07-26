package coordinator

import (
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
)

var testBenchmarkTime = time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)

var (
	promptOverlaySink     string
	permissionPolicySink  store.SessionPermissionPolicy
	sessionLineagePtrSink *store.SessionLineage
)

func BenchmarkPromptOverlay(b *testing.B) {
	input := PromptInput{
		WorkspaceID: "workspace-123",
		TaskID:      "task-456",
		RunID:       "run-789",
		WorkflowID:  "workflow-012",
		NetworkParticipation: participation.Spec{
			Version: participation.SpecVersion, Mode: participation.ModeLive,
			WorkspaceID: "workspace-123", ChannelStrategy: participation.StrategyNamed,
			ChannelID: "channel-345", Source: participation.SourceExplicitRequest,
		},
	}

	b.ReportAllocs()
	for b.Loop() {
		promptOverlaySink = PromptOverlay(input)
	}
}

func BenchmarkPermissionPolicy(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		permissionPolicySink = PermissionPolicy(participation.LocalSpec())
	}
}

func BenchmarkLineage(b *testing.B) {
	cfg := aghconfig.DefaultResolvedCoordinatorRole()
	cfg.Enabled = true
	policy := PermissionPolicy(participation.LocalSpec())

	b.ReportAllocs()
	for b.Loop() {
		sessionLineagePtrSink = Lineage(testBenchmarkTime, cfg, policy)
	}
}
