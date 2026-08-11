package daemon

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestCollectAgentProbeTargetsSkipsUnresolvedProviders(t *testing.T) {
	t.Run("Should skip provider probes when provider resolution fails", func(t *testing.T) {
		t.Parallel()

		cfg := &compozyconfig.Config{
			Providers: map[string]compozyconfig.ProviderConfig{
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

func TestAgentProbeTargetSourceDeduplicatesResolutionWarnings(t *testing.T) {
	t.Parallel()

	t.Run("Should log one warning per target error and config generation", func(t *testing.T) {
		t.Parallel()

		cfg := compozyconfig.Config{
			Providers: map[string]compozyconfig.ProviderConfig{"broken": {}},
		}
		configState := newAgentProbeConfigState(&cfg)
		var logs bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logs, nil))
		source := agentProbeTargetSource(configState, nil, logger)

		for range 2 {
			if _, err := source(t.Context()); err != nil {
				t.Fatalf("agentProbeTargetSource() error = %v", err)
			}
		}
		const warning = "daemon: resolve provider for probe health failed"
		if got, want := strings.Count(logs.String(), warning), 1; got != want {
			t.Fatalf("warning count = %d, want %d; logs=%s", got, want, logs.String())
		}

		configState.Update(&cfg)
		if _, err := source(t.Context()); err != nil {
			t.Fatalf("agentProbeTargetSource(after config generation) error = %v", err)
		}
		if got, want := strings.Count(logs.String(), warning), 2; got != want {
			t.Fatalf("warning count after config generation = %d, want %d; logs=%s", got, want, logs.String())
		}
	})
}
