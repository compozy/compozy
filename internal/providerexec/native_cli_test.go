package providerexec

import (
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestBuiltinProviderExecutionStrategies(t *testing.T) {
	t.Parallel()

	providers := compozyconfig.BuiltinProviders()
	if len(providers) != 26 {
		t.Fatalf("BuiltinProviders() count = %d, want 26", len(providers))
	}
	for providerID, provider := range providers {
		providerID, provider := providerID, provider
		t.Run("Should classify builtin provider "+providerID, func(t *testing.T) {
			t.Parallel()

			strategy := StrategyFor(provider)
			wantKind := StrategyDirectACP
			if provider.EffectiveHarness() == compozyconfig.ProviderHarnessPiACP {
				wantKind = StrategyHarnessAdapter
			}
			if providerID == "claude" || providerID == "codex" {
				wantKind = StrategyNativeCLIBridge
			}
			if strategy.Kind != wantKind {
				t.Fatalf("StrategyFor(%q).Kind = %q, want %q", providerID, strategy.Kind, wantKind)
			}
			if strategy.Kind == StrategyNativeCLIBridge && strategy.NativeCLI.Command == "" {
				t.Fatalf("StrategyFor(%q).NativeCLI.Command is empty", providerID)
			}
		})
	}
}

func TestProviderExecutionStrategyContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		provider compozyconfig.ProviderConfig
		wantKind StrategyKind
		wantCLI  NativeCLI
	}{
		{
			name:     "Should classify a direct ACP CLI",
			provider: compozyconfig.ProviderConfig{Command: "hermes acp"},
			wantKind: StrategyDirectACP,
		},
		{
			name: "Should classify a shared API harness",
			provider: compozyconfig.ProviderConfig{
				Command: "npx -y pi-acp@latest",
				Harness: compozyconfig.ProviderHarnessPiACP,
			},
			wantKind: StrategyHarnessAdapter,
		},
		{
			name:     "Should classify a versioned Claude bridge without provider identity",
			provider: compozyconfig.ProviderConfig{Command: "npx -y @agentclientprotocol/claude-agent-acp@0.70.0"},
			wantKind: StrategyNativeCLIBridge,
			wantCLI:  NativeCLI{Command: "claude", BridgeEnvKey: "CLAUDE_CODE_EXECUTABLE"},
		},
		{
			name:     "Should classify a versioned Codex bridge without provider identity",
			provider: compozyconfig.ProviderConfig{Command: "npx -y @agentclientprotocol/codex-acp@latest"},
			wantKind: StrategyNativeCLIBridge,
			wantCLI:  NativeCLI{Command: "codex", BridgeEnvKey: "CODEX_PATH"},
		},
		{
			name:     "Should leave a direct override free of stale bridge metadata",
			provider: compozyconfig.ProviderConfig{Command: "claude --acp"},
			wantKind: StrategyDirectACP,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			strategy := StrategyFor(tc.provider)
			if strategy.Kind != tc.wantKind {
				t.Fatalf("StrategyFor().Kind = %q, want %q", strategy.Kind, tc.wantKind)
			}
			if strategy.NativeCLI != tc.wantCLI {
				t.Fatalf("StrategyFor().NativeCLI = %#v, want %#v", strategy.NativeCLI, tc.wantCLI)
			}
		})
	}
}
