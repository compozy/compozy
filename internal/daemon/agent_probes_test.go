package daemon

import (
	"context"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
)

func TestCollectAgentProbeTargetsSkipsUnresolvedProviders(t *testing.T) {
	t.Run("Should skip provider probes when provider resolution fails", func(t *testing.T) {
		t.Parallel()

		cfg := &aghconfig.Config{
			Providers: map[string]aghconfig.ProviderConfig{
				"broken": {},
				"valid":  {Command: "valid-agent --acp"},
			},
		}
		targets, err := collectAgentProbeTargets(context.Background(), cfg, nil, nil)
		if err != nil {
			t.Fatalf("collectAgentProbeTargets() error = %v", err)
		}
		if len(targets) != 1 {
			t.Fatalf("len(targets) = %d, want 1; targets=%#v", len(targets), targets)
		}
		if targets[0].Provider != "valid" || targets[0].Command != "valid-agent --acp" {
			t.Fatalf("targets[0] = %#v, want valid provider command only", targets[0])
		}
	})
}
