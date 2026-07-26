package config

import "testing"

func TestProviderAliasResolution(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve explicit provider aliases to canonical provider ids", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			want string
		}{
			{name: providerClaudeCodeAlias, want: providerClaudeKey},
			{name: "CLAUDE", want: providerClaudeKey},
			{name: "AI-Gateway", want: providerVercelAIGatewayValue},
			{name: "aigateway", want: providerVercelAIGatewayValue},
			{name: "open-code", want: providerOpencodeKey},
			{name: providerXaiDotAlias, want: providerXaiKey},
		}

		for _, tc := range tests {
			t.Run("Should resolve "+tc.name, func(t *testing.T) {
				t.Parallel()

				if got := CanonicalProviderName(tc.name); got != tc.want {
					t.Fatalf("CanonicalProviderName(%q) = %q, want %q", tc.name, got, tc.want)
				}
			})
		}
	})

	t.Run("Should preserve exact model ids while resolving provider aliases", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultWithHome(HomePaths{})
		cfg.Defaults.Provider = "ai-gateway"
		agent := AgentDef{Name: "coder", Model: "opus", Prompt: "Help with the release."}

		resolved, err := cfg.ResolveAgent(agent)
		if err != nil {
			t.Fatalf("ResolveAgent() error = %v", err)
		}
		if got, want := resolved.Provider, providerVercelAIGatewayValue; got != want {
			t.Fatalf("ResolveAgent() Provider = %q, want %q", got, want)
		}
		if got, want := resolved.Model, "opus"; got != want {
			t.Fatalf("ResolveAgent() Model = %q, want exact id %q", got, want)
		}
	})

	t.Run("Should preserve an exact configured default instead of aliasing it", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultWithHome(HomePaths{})
		cfg.Providers[providerClaudeKey] = ProviderConfig{
			Models: ProviderModelsConfig{Default: "sonnet"},
		}

		resolved, err := cfg.ResolveProvider(providerClaudeCodeAlias)
		if err != nil {
			t.Fatalf("ResolveProvider() error = %v", err)
		}
		if got, want := resolved.Models.Default, "sonnet"; got != want {
			t.Fatalf("ResolveProvider() Models.Default = %q, want exact id %q", got, want)
		}
	})
}
