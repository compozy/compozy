package config

import (
	"errors"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/reasoning"
)

const expectedCodexModelsReleaseDate = "2026-06-26"

func TestBuiltinProvidersContainExpectedCommands(t *testing.T) {
	t.Parallel()

	providers := BuiltinProviders()
	tests := []struct {
		name            string
		command         string
		harness         ProviderHarness
		authMode        ProviderAuthMode
		runtimeProvider string
		defaultModel    string
		noSessionMCP    bool
		loginCommand    string
		apiKeyEnv       string
		required        bool
	}{
		{
			name:     "blackbox",
			command:  "blackbox --experimental-acp",
			harness:  ProviderHarnessACP,
			authMode: ProviderAuthModeNativeCLI,
		},
		{
			name:         "claude",
			command:      "npx -y @agentclientprotocol/claude-agent-acp@latest",
			harness:      ProviderHarnessACP,
			authMode:     ProviderAuthModeNativeCLI,
			defaultModel: modelClaudeSonnet5ID,
			loginCommand: "claude auth login",
		},
		{
			name:     "cline",
			command:  "npx -y cline@latest --acp",
			harness:  ProviderHarnessACP,
			authMode: ProviderAuthModeNativeCLI,
		},
		{
			name:         "codex",
			command:      "npx -y @agentclientprotocol/codex-acp@latest",
			harness:      ProviderHarnessACP,
			authMode:     ProviderAuthModeNativeCLI,
			defaultModel: modelGPT56SolID,
			loginCommand: "codex login",
		},
		{
			name:     "copilot",
			command:  "copilot --acp --stdio",
			harness:  ProviderHarnessACP,
			authMode: ProviderAuthModeNativeCLI,
		},
		{name: "cursor", command: "cursor-agent acp", harness: ProviderHarnessACP, authMode: ProviderAuthModeNativeCLI},
		{
			name:         "gemini",
			command:      "gemini --acp",
			harness:      ProviderHarnessACP,
			authMode:     ProviderAuthModeNativeCLI,
			defaultModel: "gemini-3.1-pro-preview",
		},
		{name: "goose", command: "goose acp", harness: ProviderHarnessACP, authMode: ProviderAuthModeNativeCLI},
		{name: "hermes", command: "hermes acp", harness: ProviderHarnessACP, authMode: ProviderAuthModeNativeCLI},
		{name: "junie", command: "junie --acp true", harness: ProviderHarnessACP, authMode: ProviderAuthModeNativeCLI},
		{
			name:     "kimi-cli",
			command:  "kimi acp",
			harness:  ProviderHarnessACP,
			authMode: ProviderAuthModeNativeCLI,
		},
		{name: "kiro", command: "kiro-cli-chat acp", harness: ProviderHarnessACP, authMode: ProviderAuthModeNativeCLI},
		{
			name:         "opencode",
			command:      "npx -y opencode-ai@latest acp",
			harness:      ProviderHarnessACP,
			authMode:     ProviderAuthModeNativeCLI,
			loginCommand: "opencode auth login",
		},
		{
			name:         "openclaw",
			command:      "openclaw acp",
			harness:      ProviderHarnessACP,
			authMode:     ProviderAuthModeNativeCLI,
			noSessionMCP: true,
		},
		{name: "openhands", command: "openhands acp", harness: ProviderHarnessACP, authMode: ProviderAuthModeNativeCLI},
		{
			name:     "qoder",
			command:  "npx -y @qoder-ai/qodercli@latest --acp",
			harness:  ProviderHarnessACP,
			authMode: ProviderAuthModeNativeCLI,
		},
		{
			name:         "qwen-code",
			command:      "npx -y @qwen-code/qwen-code@latest --acp --experimental-skills",
			harness:      ProviderHarnessACP,
			authMode:     ProviderAuthModeNativeCLI,
			defaultModel: "qwen3.6-plus",
		},
		{
			name:            "pi",
			command:         "npx -y pi-acp@latest",
			harness:         ProviderHarnessPiACP,
			authMode:        ProviderAuthModeNativeCLI,
			runtimeProvider: "anthropic",
			defaultModel:    "claude-opus-4-7",
			loginCommand:    "npx -y pi-acp@latest --terminal-login",
		},
		{
			name:            "openrouter",
			command:         "npx -y pi-acp@latest",
			harness:         ProviderHarnessPiACP,
			authMode:        ProviderAuthModeBoundSecret,
			runtimeProvider: "openrouter",
			defaultModel:    "openai/gpt-5.4",
			apiKeyEnv:       "OPENROUTER_API_KEY",
			required:        true,
		},
		{
			name:            "zai",
			command:         "npx -y pi-acp@latest",
			harness:         ProviderHarnessPiACP,
			authMode:        ProviderAuthModeBoundSecret,
			runtimeProvider: "zai",
			defaultModel:    "glm-4.6",
			apiKeyEnv:       "ZAI_API_KEY",
			required:        true,
		},
		{
			name:            "moonshot",
			command:         "npx -y pi-acp@latest",
			harness:         ProviderHarnessPiACP,
			authMode:        ProviderAuthModeBoundSecret,
			runtimeProvider: "kimi-coding",
			defaultModel:    "kimi-k2-thinking",
			apiKeyEnv:       "KIMI_API_KEY",
			required:        true,
		},
		{
			name:            "vercel-ai-gateway",
			command:         "npx -y pi-acp@latest",
			harness:         ProviderHarnessPiACP,
			authMode:        ProviderAuthModeBoundSecret,
			runtimeProvider: "vercel-ai-gateway",
			defaultModel:    "anthropic/claude-opus-4-7",
			apiKeyEnv:       "AI_GATEWAY_API_KEY",
			required:        true,
		},
		{
			name:            "xai",
			command:         "npx -y pi-acp@latest",
			harness:         ProviderHarnessPiACP,
			authMode:        ProviderAuthModeBoundSecret,
			runtimeProvider: "xai",
			defaultModel:    "grok-4-fast-non-reasoning",
			apiKeyEnv:       "XAI_API_KEY",
			required:        true,
		},
		{
			name:            "minimax",
			command:         "npx -y pi-acp@latest",
			harness:         ProviderHarnessPiACP,
			authMode:        ProviderAuthModeBoundSecret,
			runtimeProvider: "minimax",
			defaultModel:    "MiniMax-M2.1",
			apiKeyEnv:       "MINIMAX_API_KEY",
			required:        true,
		},
		{
			name:            "mistral",
			command:         "npx -y pi-acp@latest",
			harness:         ProviderHarnessPiACP,
			authMode:        ProviderAuthModeBoundSecret,
			runtimeProvider: "mistral",
			defaultModel:    "devstral-medium-latest",
			apiKeyEnv:       "MISTRAL_API_KEY",
			required:        true,
		},
		{
			name:            "groq",
			command:         "npx -y pi-acp@latest",
			harness:         ProviderHarnessPiACP,
			authMode:        ProviderAuthModeBoundSecret,
			runtimeProvider: "groq",
			defaultModel:    "openai/gpt-oss-120b",
			apiKeyEnv:       "GROQ_API_KEY",
			required:        true,
		},
	}

	for _, tc := range tests {
		t.Run("Should expose builtin provider "+tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := providers[tc.name]
			if !ok {
				t.Fatalf("BuiltinProviders() missing %q", tc.name)
			}
			if got.Command != tc.command {
				t.Fatalf("BuiltinProviders()[%q].Command = %q, want %q", tc.name, got.Command, tc.command)
			}
			if got.EffectiveHarness() != tc.harness {
				t.Fatalf("BuiltinProviders()[%q].Harness = %q, want %q", tc.name, got.EffectiveHarness(), tc.harness)
			}
			if got.EffectiveAuthMode() != tc.authMode {
				t.Fatalf("BuiltinProviders()[%q].AuthMode = %q, want %q", tc.name, got.EffectiveAuthMode(), tc.authMode)
			}
			if got.RuntimeProviderName(tc.name) != firstNonEmpty(tc.runtimeProvider, tc.name) {
				t.Fatalf(
					"BuiltinProviders()[%q].RuntimeProvider = %q, want %q",
					tc.name,
					got.RuntimeProviderName(tc.name),
					firstNonEmpty(tc.runtimeProvider, tc.name),
				)
			}
			if got.Models.Default != tc.defaultModel {
				t.Fatalf(
					"BuiltinProviders()[%q].Models.Default = %q, want %q",
					tc.name,
					got.Models.Default,
					tc.defaultModel,
				)
			}
			if tc.defaultModel != "" && !providerCuratedModelsContain(got.Models.Curated, tc.defaultModel) {
				t.Fatalf(
					"BuiltinProviders()[%q].Models.Curated = %#v, want default model %q",
					tc.name,
					got.Models.Curated,
					tc.defaultModel,
				)
			}
			if got.SessionMCPEnabled() == tc.noSessionMCP {
				t.Fatalf(
					"BuiltinProviders()[%q].SessionMCPEnabled() = %t, want %t",
					tc.name,
					got.SessionMCPEnabled(),
					!tc.noSessionMCP,
				)
			}
			if strings.TrimSpace(got.AuthLoginCmd) != tc.loginCommand {
				t.Fatalf(
					"BuiltinProviders()[%q].AuthLoginCmd = %q, want %q",
					tc.name,
					got.AuthLoginCmd,
					tc.loginCommand,
				)
			}
			slots := got.EffectiveCredentialSlots()
			if tc.apiKeyEnv == "" {
				if len(slots) != 0 {
					t.Fatalf("BuiltinProviders()[%q].CredentialSlots = %#v, want none", tc.name, slots)
				}
				return
			}
			if len(slots) != 1 {
				t.Fatalf("BuiltinProviders()[%q].CredentialSlots = %#v, want one slot", tc.name, slots)
			}
			slot := slots[0]
			if slot.Name != "api_key" || slot.Kind != "api_key" || slot.TargetEnv != tc.apiKeyEnv ||
				slot.SecretRef != "env:"+tc.apiKeyEnv || slot.Required != tc.required {
				t.Fatalf(
					"BuiltinProviders()[%q].CredentialSlots[0] = %#v, want api_key env slot required=%t",
					tc.name,
					slot,
					tc.required,
				)
			}
		})
	}

	t.Run("Should keep process args and non-secret env valid for stdio", func(t *testing.T) {
		t.Parallel()

		server := MCPServer{
			Name:      "local",
			Transport: MCPServerTransportStdio,
			Command:   "npx",
			Args:      []string{"--verbose"},
			Env:       map[string]string{"LOG_LEVEL": "debug"},
		}
		if err := server.Validate("mcp_servers[0]"); err != nil {
			t.Fatalf("Validate(stdio MCP process fields) error = %v, want nil", err)
		}
	})
}

func providerCuratedModelsContain(models []ProviderModelConfig, id string) bool {
	for _, model := range models {
		if model.ID == id {
			return true
		}
	}
	return false
}

func TestBuiltinProviderModelCatalogMatchesCurrentContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		models ProviderModelsConfig
	}{
		{
			name: "claude",
			models: ProviderModelsConfig{
				Default: modelClaudeSonnet5ID,
				Reasoning: ProviderReasoningConfig{
					Apply: ReasoningApplyACPOption,
				},
				Curated: []ProviderModelConfig{
					{
						ID:                     modelClaudeFable5ID,
						DisplayName:            "Claude Fable 5",
						ContextWindow:          new(int64(1_000_000)),
						MaxOutputTokens:        new(int64(128_000)),
						SupportsTools:          new(true),
						SupportsReasoning:      new(true),
						ReasoningEfforts:       []string{"low", "medium", "high", "xhigh", "max"},
						DefaultReasoningEffort: "high",
						CostInputPerMillion:    new(10.0),
						CostOutputPerMillion:   new(50.0),
						Featured:               new(true),
						ReleaseDate:            "2026-06-09",
					},
					{
						ID:                     modelClaudeOpus48ID,
						DisplayName:            "Claude Opus 4.8",
						ContextWindow:          new(int64(1_000_000)),
						MaxOutputTokens:        new(int64(128_000)),
						SupportsTools:          new(true),
						SupportsReasoning:      new(true),
						ReasoningEfforts:       []string{"low", "medium", "high", "xhigh", "max"},
						DefaultReasoningEffort: "high",
						CostInputPerMillion:    new(5.0),
						CostOutputPerMillion:   new(25.0),
						Featured:               new(false),
					},
					{
						ID:                     modelClaudeSonnet5ID,
						DisplayName:            "Claude Sonnet 5",
						ContextWindow:          new(int64(1_000_000)),
						MaxOutputTokens:        new(int64(128_000)),
						SupportsTools:          new(true),
						SupportsReasoning:      new(true),
						ReasoningEfforts:       []string{"low", "medium", "high", "xhigh", "max"},
						DefaultReasoningEffort: "high",
						CostInputPerMillion:    new(3.0),
						CostOutputPerMillion:   new(15.0),
						Featured:               new(false),
					},
					{
						ID:                   modelClaudeHaiku45CurrentID,
						DisplayName:          "Claude Haiku 4.5",
						ContextWindow:        new(int64(200_000)),
						MaxOutputTokens:      new(int64(64_000)),
						SupportsTools:        new(true),
						SupportsReasoning:    new(true),
						CostInputPerMillion:  new(1.0),
						CostOutputPerMillion: new(5.0),
					},
				},
			},
		},
		{
			name: "codex",
			models: ProviderModelsConfig{
				Default: modelGPT56SolID,
				Reasoning: ProviderReasoningConfig{
					Apply: ReasoningApplyACPOption,
				},
				Curated: []ProviderModelConfig{
					{
						ID:                     modelGPT56SolID,
						DisplayName:            "GPT-5.6 Sol",
						ContextWindow:          new(int64(1_050_000)),
						MaxOutputTokens:        new(int64(128_000)),
						SupportsTools:          new(true),
						SupportsReasoning:      new(true),
						ReasoningEfforts:       []string{"none", "low", "medium", "high", "xhigh", "max"},
						DefaultReasoningEffort: "medium",
						CostInputPerMillion:    new(5.0),
						CostOutputPerMillion:   new(30.0),
						Featured:               new(true),
						ReleaseDate:            expectedCodexModelsReleaseDate,
					},
					{
						ID:                     modelGPT56TerraID,
						DisplayName:            "GPT-5.6 Terra",
						ContextWindow:          new(int64(1_050_000)),
						MaxOutputTokens:        new(int64(128_000)),
						SupportsTools:          new(true),
						SupportsReasoning:      new(true),
						ReasoningEfforts:       []string{"none", "low", "medium", "high", "xhigh", "max"},
						DefaultReasoningEffort: "medium",
						CostInputPerMillion:    new(2.5),
						CostOutputPerMillion:   new(15.0),
						Featured:               new(false),
						ReleaseDate:            expectedCodexModelsReleaseDate,
					},
					{
						ID:                     modelGPT56LunaID,
						DisplayName:            "GPT-5.6 Luna",
						ContextWindow:          new(int64(1_050_000)),
						MaxOutputTokens:        new(int64(128_000)),
						SupportsTools:          new(true),
						SupportsReasoning:      new(true),
						ReasoningEfforts:       []string{"none", "low", "medium", "high", "xhigh", "max"},
						DefaultReasoningEffort: "medium",
						CostInputPerMillion:    new(1.0),
						CostOutputPerMillion:   new(6.0),
						Featured:               new(false),
						ReleaseDate:            expectedCodexModelsReleaseDate,
					},
				},
			},
		},
	}

	providers := BuiltinProviders()
	for _, tc := range tests {
		t.Run("Should expose exact current "+tc.name+" model metadata", func(t *testing.T) {
			t.Parallel()

			provider, ok := providers[tc.name]
			if !ok {
				t.Fatalf("BuiltinProviders() missing %q", tc.name)
			}
			if !reflect.DeepEqual(provider.Models, tc.models) {
				t.Fatalf("BuiltinProviders()[%q].Models = %#v, want %#v", tc.name, provider.Models, tc.models)
			}
		})
	}
}

func TestRepoRootConfigProviderDefaultsMatchBuiltinRegistry(t *testing.T) {
	t.Parallel()

	rootConfig := filepath.Join(repoRootFromConfigTest(t), "config.toml")
	overlay := Config{Providers: map[string]ProviderConfig{}}
	if err := ApplyConfigOverlayFile(rootConfig, &overlay); err != nil {
		t.Fatalf("ApplyConfigOverlayFile(repo config) error = %v", err)
	}

	builtins := BuiltinProviders()
	for name, provider := range overlay.Providers {
		if provider.Models.Default == "" {
			continue
		}
		builtin, ok := builtins[name]
		if !ok {
			t.Fatalf("repo config provider %q is not in the builtin registry", name)
		}
		if got, want := provider.Models.Default, builtin.Models.Default; got != want {
			t.Fatalf("repo config provider %q models.default = %q, want builtin %q", name, got, want)
		}
	}
}

func repoRootFromConfigTest(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func TestBuiltinProviderCommandsUseLatestDriverPackages(t *testing.T) {
	t.Parallel()

	packagePrefixes := []string{
		"@agentclientprotocol/claude-agent-acp@",
		"@agentclientprotocol/codex-acp@",
		"pi-acp@",
		"opencode-ai@",
		"cline@",
		"@qoder-ai/qodercli@",
		"@qwen-code/qwen-code@",
	}

	for name, provider := range BuiltinProviders() {
		t.Run("Should not pin driver package for "+name, func(t *testing.T) {
			t.Parallel()

			fields := strings.FieldsSeq(provider.Command)
			for field := range fields {
				for _, prefix := range packagePrefixes {
					if !strings.HasPrefix(field, prefix) {
						continue
					}
					version := strings.TrimPrefix(field, prefix)
					if version != "latest" {
						t.Fatalf(
							"BuiltinProviders()[%q].Command = %q pins %s%s, want @latest",
							name,
							provider.Command,
							prefix,
							version,
						)
					}
				}
			}
		})
	}
}

func TestCanonicalProviderNameResolvesNewDriverAliases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
	}{
		{name: "blackbox-ai", want: "blackbox"},
		{name: "cline-cli", want: "cline"},
		{name: "goose-cli", want: "goose"},
		{name: "hermes-agent", want: "hermes"},
		{name: "junie-cli", want: "junie"},
		{name: "kimi", want: "moonshot"},
		{name: "kimi cli", want: "kimi-cli"},
		{name: "kimi-code", want: "kimi-cli"},
		{name: "open-hands", want: "openhands"},
		{name: "openclaw-cli", want: "openclaw"},
		{name: "qoder-cli", want: "qoder"},
		{name: "qwen", want: "qwen-code"},
		{name: "qwen code", want: "qwen-code"},
	}

	for _, tc := range tests {
		t.Run("Should resolve "+tc.name, func(t *testing.T) {
			t.Parallel()

			if got := CanonicalProviderName(tc.name); got != tc.want {
				t.Fatalf("CanonicalProviderName(%q) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func TestProviderConfigOverrideMergesWithBuiltins(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Providers: map[string]ProviderConfig{
			"claude": {
				Models: ProviderModelsConfig{Default: "claude-opus-override"},
			},
		},
	}

	provider, err := cfg.ResolveProvider("claude")
	if err != nil {
		t.Fatalf("ResolveProvider() error = %v", err)
	}
	if provider.Command == "" {
		t.Fatal("ResolveProvider() Command = empty, want builtin command")
	}
	if provider.Models.Default != "claude-opus-override" {
		t.Fatalf("ResolveProvider() Models.Default = %q, want %q", provider.Models.Default, "claude-opus-override")
	}
	if provider.EffectiveAuthMode() != ProviderAuthModeNativeCLI {
		t.Fatalf("ResolveProvider() AuthMode = %q, want native_cli", provider.EffectiveAuthMode())
	}
	if slots := provider.EffectiveCredentialSlots(); len(slots) != 0 {
		t.Fatalf("ResolveProvider() CredentialSlots = %#v, want no native CLI slots", slots)
	}
	provider.Models.Curated[0].ID = "mutated"
	provider.Models.Curated[0].ReasoningEfforts[0] = "mutated"
	if provider.Models.Curated[0].Featured != nil {
		*provider.Models.Curated[0].Featured = false
	}

	second, err := cfg.ResolveProvider("claude")
	if err != nil {
		t.Fatalf("ResolveProvider(second) error = %v", err)
	}
	builtin := BuiltinProviders()["claude"]
	for _, candidate := range []ProviderConfig{second, builtin} {
		model := candidate.Models.Curated[0]
		if model.ID != "claude-fable-5" || model.ReasoningEfforts[0] != "low" ||
			model.Featured == nil || !*model.Featured {
			t.Fatalf("builtin provider changed through resolved clone: %#v", model)
		}
	}
}

func TestProviderCredentialSlotOverridePreservesSecretRefSemantics(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve existing slot required kind and secret ref", func(t *testing.T) {
		t.Parallel()

		provider := mergeProvider(
			ProviderConfig{
				Command: "custom-agent --acp",
				CredentialSlots: []ProviderCredentialSlot{
					{
						Name:      "api_key",
						TargetEnv: "OPENROUTER_API_KEY",
						SecretRef: "vault:providers/openrouter/api-key",
						Kind:      "api_key",
						Required:  false,
					},
				},
			},
			ProviderConfig{
				CredentialSlots: []ProviderCredentialSlot{
					{
						Name:      "api_key",
						TargetEnv: "OPENROUTER_RUNTIME_KEY",
						SecretRef: "vault:providers/openrouter/api-key",
						Kind:      "api_key",
						Required:  false,
					},
				},
			},
		)
		slots := provider.EffectiveCredentialSlots()
		if len(slots) != 1 {
			t.Fatalf("EffectiveCredentialSlots() = %#v, want one slot", slots)
		}
		slot := slots[0]
		if slot.TargetEnv != "OPENROUTER_RUNTIME_KEY" {
			t.Fatalf("slot.TargetEnv = %q, want override env", slot.TargetEnv)
		}
		if slot.SecretRef != "vault:providers/openrouter/api-key" {
			t.Fatalf("slot.SecretRef = %q, want preserved secret ref", slot.SecretRef)
		}
		if slot.Required {
			t.Fatalf("slot.Required = true, want preserved optional slot")
		}
		if slot.Kind != "api_key" || slot.Name != "api_key" {
			t.Fatalf("slot identity = %#v, want preserved api_key identity", slot)
		}
	})

	t.Run("Should not synthesize compatibility slots for custom providers", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			Providers: map[string]ProviderConfig{
				"custom": {
					Command: "custom-agent --acp",
				},
			},
		}

		provider, err := cfg.ResolveProvider("custom")
		if err != nil {
			t.Fatalf("ResolveProvider(custom) error = %v", err)
		}
		slots := provider.EffectiveCredentialSlots()
		if len(slots) != 0 {
			t.Fatalf("EffectiveCredentialSlots() = %#v, want no synthesized slots", slots)
		}
	})
}

func TestProviderOverlayCredentialSlotsReplaceExistingSlots(t *testing.T) {
	t.Parallel()

	t.Run("Should replace credential slots from overlay", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			Providers: map[string]ProviderConfig{
				"custom": {
					Command: "custom-agent --acp",
					CredentialSlots: []ProviderCredentialSlot{
						{
							Name:      "api_key",
							TargetEnv: "CUSTOM_API_KEY",
							SecretRef: "env:CUSTOM_API_KEY",
							Kind:      "api_key",
							Required:  false,
						},
					},
				},
			},
		}
		overlay, err := loadConfigOverlayBytes([]byte(`
	[providers.custom]
	[[providers.custom.credential_slots]]
	name = "api_key"
	target_env = "CUSTOM_RUNTIME_KEY"
	secret_ref = "env:CUSTOM_RUNTIME_KEY"
	kind = "api_key"
	required = false
	`), "inline")
		if err != nil {
			t.Fatalf("loadConfigOverlayBytes() error = %v", err)
		}
		if err := overlay.Apply(&cfg); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}

		slots := cfg.Providers["custom"].CredentialSlots
		if len(slots) != 1 {
			t.Fatalf("CredentialSlots = %#v, want one slot", slots)
		}
		if slots[0].TargetEnv != "CUSTOM_RUNTIME_KEY" || slots[0].SecretRef != "env:CUSTOM_RUNTIME_KEY" {
			t.Fatalf("CredentialSlots[0] = %#v, want overlay credential slot", slots[0])
		}
		if slots[0].Required {
			t.Fatalf("CredentialSlots[0].Required = true, want preserved false")
		}
	})
}

func TestProviderAuthModeValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should reject credential slots on native builtins without explicit bound secret auth", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			Providers: map[string]ProviderConfig{
				"claude": {
					CredentialSlots: []ProviderCredentialSlot{
						apiKeyCredentialSlot("ANTHROPIC_API_KEY"),
					},
				},
			},
		}

		_, err := cfg.ResolveProvider("claude")
		if err == nil {
			t.Fatal("ResolveProvider(native slot override) error = nil, want validation error")
		}
		if !strings.Contains(err.Error(), `auth_mode must be "bound_secret"`) {
			t.Fatalf("ResolveProvider(native slot override) error = %v, want auth_mode guidance", err)
		}
	})

	t.Run("Should allow explicit bound secret auth on native builtins", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			Providers: map[string]ProviderConfig{
				"claude": {
					AuthMode: ProviderAuthModeBoundSecret,
					CredentialSlots: []ProviderCredentialSlot{
						apiKeyCredentialSlot("ANTHROPIC_API_KEY"),
					},
				},
			},
		}

		provider, err := cfg.ResolveProvider("claude")
		if err != nil {
			t.Fatalf("ResolveProvider(bound native override) error = %v", err)
		}
		if got, want := provider.EffectiveAuthMode(), ProviderAuthModeBoundSecret; got != want {
			t.Fatalf("ResolveProvider() AuthMode = %q, want %q", got, want)
		}
		if provider.AuthLoginCmd != "" {
			t.Fatalf("ResolveProvider() AuthLoginCmd = %q, want empty", provider.AuthLoginCmd)
		}
	})

	t.Run("Should clear inherited native auth commands when a builtin switches to none", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			Providers: map[string]ProviderConfig{
				"claude": {
					AuthMode:     ProviderAuthModeNone,
					NoneSecurity: ProviderNoneSecurityLocalTransport,
				},
			},
		}

		provider, err := cfg.ResolveProvider("claude")
		if err != nil {
			t.Fatalf("ResolveProvider(none native override) error = %v", err)
		}
		if got, want := provider.EffectiveAuthMode(), ProviderAuthModeNone; got != want {
			t.Fatalf("ResolveProvider() AuthMode = %q, want %q", got, want)
		}
		if provider.AuthStatusCmd != "" || provider.AuthLoginCmd != "" {
			t.Fatalf(
				"ResolveProvider() auth commands = (%q, %q), want empty",
				provider.AuthStatusCmd,
				provider.AuthLoginCmd,
			)
		}
		if slots := provider.EffectiveCredentialSlots(); len(slots) != 0 {
			t.Fatalf("ResolveProvider() CredentialSlots = %#v, want empty", slots)
		}
	})

	t.Run("Should clear inherited bound credentials when a builtin switches to native CLI", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			Providers: map[string]ProviderConfig{
				"openrouter": {AuthMode: ProviderAuthModeNativeCLI},
			},
		}

		provider, err := cfg.ResolveProvider("openrouter")
		if err != nil {
			t.Fatalf("ResolveProvider(native bound override) error = %v", err)
		}
		if got, want := provider.EffectiveAuthMode(), ProviderAuthModeNativeCLI; got != want {
			t.Fatalf("ResolveProvider() AuthMode = %q, want %q", got, want)
		}
		if slots := provider.EffectiveCredentialSlots(); len(slots) != 0 {
			t.Fatalf("ResolveProvider() CredentialSlots = %#v, want empty", slots)
		}
	})

	t.Run("Should reject bound secret auth without credential slots", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			Providers: map[string]ProviderConfig{
				"custom": {
					Command:  "custom-agent --acp",
					AuthMode: ProviderAuthModeBoundSecret,
				},
			},
		}

		_, err := cfg.ResolveProvider("custom")
		if err == nil {
			t.Fatal("ResolveProvider(bound without slots) error = nil, want validation error")
		}
		if !strings.Contains(err.Error(), "credential_slots is required") {
			t.Fatalf("ResolveProvider(bound without slots) error = %v, want credential_slots guidance", err)
		}
	})

	t.Run("Should reject credential slots when auth is none", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			Providers: map[string]ProviderConfig{
				"custom": {
					Command:  "custom-agent --acp",
					AuthMode: ProviderAuthModeNone,
					CredentialSlots: []ProviderCredentialSlot{
						apiKeyCredentialSlot("CUSTOM_API_KEY"),
					},
				},
			},
		}

		_, err := cfg.ResolveProvider("custom")
		if err == nil {
			t.Fatal("ResolveProvider(none with slots) error = nil, want validation error")
		}
		if !strings.Contains(err.Error(), "cannot be set when auth_mode is none") {
			t.Fatalf("ResolveProvider(none with slots) error = %v, want none guidance", err)
		}
	})

	t.Run("Should reject status command when auth is none", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			Providers: map[string]ProviderConfig{
				"custom": {
					Command:       "custom-agent --acp",
					AuthMode:      ProviderAuthModeNone,
					AuthStatusCmd: "custom-agent auth status",
				},
			},
		}

		_, err := cfg.ResolveProvider("custom")
		if err == nil {
			t.Fatal("ResolveProvider(none with status command) error = nil, want validation error")
		}
		if !strings.Contains(err.Error(), "auth_status_command cannot be set when auth_mode is none") {
			t.Fatalf("ResolveProvider(none with status command) error = %v, want status command guidance", err)
		}
	})

	t.Run("Should reject login command when auth is none", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			Providers: map[string]ProviderConfig{
				"custom": {
					Command:      "custom-agent --acp",
					AuthMode:     ProviderAuthModeNone,
					AuthLoginCmd: "custom-agent auth login",
				},
			},
		}

		_, err := cfg.ResolveProvider("custom")
		if err == nil {
			t.Fatal("ResolveProvider(none with login command) error = nil, want validation error")
		}
		if !strings.Contains(err.Error(), "auth_login_command cannot be set when auth_mode is none") {
			t.Fatalf("ResolveProvider(none with login command) error = %v, want login command guidance", err)
		}
	})

	t.Run("Should reject invalid none security rationale", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			Providers: map[string]ProviderConfig{
				"custom": {
					Command:      "custom-agent --acp",
					AuthMode:     ProviderAuthModeNone,
					NoneSecurity: ProviderNoneSecurity("public-write"),
				},
			},
		}

		_, err := cfg.ResolveProvider("custom")
		if err == nil {
			t.Fatal("ResolveProvider(invalid none_security) error = nil, want validation error")
		}
		if !strings.Contains(err.Error(), "none_security") {
			t.Fatalf("ResolveProvider(invalid none_security) error = %v, want none_security guidance", err)
		}
	})
}

func TestProviderCredentialSlotValidateRestrictsSecretRefs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ref     string
		wantErr bool
	}{
		{name: "Should accept env ref", ref: "env:OPENROUTER_API_KEY"},
		{name: "Should accept provider secret ref", ref: "vault:providers/openrouter/api-key"},
		{
			name: "Should accept provider secret ref with underscore slot",
			ref:  "vault:providers/vercel-ai-gateway/api_key",
		},
		{name: "Should reject arbitrary vault suffix", ref: "vault:anything", wantErr: true},
		{name: "Should reject provider secret ref without slot", ref: "vault:providers/openrouter", wantErr: true},
		{
			name:    "Should reject provider secret ref with uppercase provider",
			ref:     "vault:providers/OpenRouter/api-key",
			wantErr: true,
		},
		{
			name:    "Should reject provider secret ref with uppercase slot",
			ref:     "vault:providers/openrouter/API-key",
			wantErr: true,
		},
		{name: "Should reject malformed env ref", ref: "env:OPENROUTER API KEY", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			slot := ProviderCredentialSlot{
				Name:      "api_key",
				TargetEnv: "OPENROUTER_API_KEY",
				SecretRef: tc.ref,
				Kind:      "api_key",
				Required:  true,
			}
			err := slot.Validate("providers.openrouter.credential_slots[0]")
			if tc.wantErr && err == nil {
				t.Fatalf("Validate(%q) error = nil, want validation failure", tc.ref)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Validate(%q) error = %v, want nil", tc.ref, err)
			}
		})
	}
}

func TestResolveAgentModelOverridesProviderDefault(t *testing.T) {
	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	cfg := DefaultWithHome(homePaths)
	agent := AgentDef{
		Name:     "coder",
		Provider: "claude",
		Model:    "agent-model",
		Prompt:   "prompt",
	}

	resolved, err := cfg.ResolveAgent(agent)
	if err != nil {
		t.Fatalf("ResolveAgent() error = %v", err)
	}
	if resolved.Model != "agent-model" {
		t.Fatalf("ResolveAgent() Model = %q, want %q", resolved.Model, "agent-model")
	}
}

func TestResolveAgentReasoningEffort(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve the agent reasoning default for its provider", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		cfg := DefaultWithHome(homePaths)
		resolved, err := cfg.ResolveAgent(AgentDef{
			Name:            "coder",
			Provider:        "claude",
			ReasoningEffort: providerReasoningMaxKey,
			Prompt:          "prompt",
		})
		if err != nil {
			t.Fatalf("ResolveAgent() error = %v", err)
		}
		if got, want := resolved.ReasoningEffort, providerReasoningMaxKey; got != want {
			t.Fatalf("ResolveAgent() ReasoningEffort = %q, want %q", got, want)
		}
		if got, want := resolved.RuntimeSources.ReasoningEffort, RuntimeValueSourceAgent; got != want {
			t.Fatalf("ResolveAgent() ReasoningEffort source = %q, want %q", got, want)
		}
	})

	t.Run("Should resolve the selected model default when the provider changes", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		cfg := DefaultWithHome(homePaths)
		resolved, err := cfg.ResolveSessionAgentWithRuntime(AgentDef{
			Name:            "coder",
			Provider:        "claude",
			ReasoningEffort: providerReasoningMaxKey,
			Prompt:          "prompt",
		}, "codex", "gpt-5.6-terra")
		if err != nil {
			t.Fatalf("ResolveSessionAgentWithRuntime() error = %v", err)
		}
		if got, want := resolved.ReasoningEffort, providerMediumKey; got != want {
			t.Fatalf("ResolveSessionAgentWithRuntime() ReasoningEffort = %q, want %q", got, want)
		}
		if got, want := resolved.RuntimeSources.ReasoningEffort, RuntimeValueSourceModelDefault; got != want {
			t.Fatalf("ResolveSessionAgentWithRuntime() ReasoningEffort source = %q, want %q", got, want)
		}
	})

	t.Run("Should resolve the selected model default when the model changes within one provider", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		cfg := DefaultWithHome(homePaths)
		resolved, err := cfg.ResolveSessionAgentWithRuntime(AgentDef{
			Name:            "coder",
			Provider:        "codex",
			Model:           modelGPT56SolID,
			ReasoningEffort: providerMediumKey,
			Prompt:          "prompt",
		}, "codex", "gpt-5.6-terra")
		if err != nil {
			t.Fatalf("ResolveSessionAgentWithRuntime() error = %v", err)
		}
		if got, want := resolved.ReasoningEffort, providerMediumKey; got != want {
			t.Fatalf("ResolveSessionAgentWithRuntime() ReasoningEffort = %q, want %q", got, want)
		}
		if got, want := resolved.RuntimeSources.ReasoningEffort, RuntimeValueSourceModelDefault; got != want {
			t.Fatalf("ResolveSessionAgentWithRuntime() ReasoningEffort source = %q, want %q", got, want)
		}
	})

	t.Run("Should inherit project provider and provider model defaults with provenance", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		cfg := DefaultWithHome(homePaths)
		cfg.Defaults.Provider = "codex"
		resolved, err := cfg.ResolveAgent(AgentDef{Name: "coder", Prompt: "prompt"})
		if err != nil {
			t.Fatalf("ResolveAgent() error = %v", err)
		}
		if got, want := resolved.Provider, "codex"; got != want {
			t.Fatalf("ResolveAgent() Provider = %q, want %q", got, want)
		}
		if got, want := resolved.Model, modelGPT56SolID; got != want {
			t.Fatalf("ResolveAgent() Model = %q, want %q", got, want)
		}
		if got, want := resolved.ReasoningEffort, providerMediumKey; got != want {
			t.Fatalf("ResolveAgent() ReasoningEffort = %q, want %q", got, want)
		}
		wantSources := ResolvedRuntimeSources{
			Provider:        RuntimeValueSourceProjectDefault,
			Model:           RuntimeValueSourceProviderDefault,
			ReasoningEffort: RuntimeValueSourceModelDefault,
		}
		if got := resolved.RuntimeSources; got != wantSources {
			t.Fatalf("ResolveAgent() RuntimeSources = %#v, want %#v", got, wantSources)
		}
	})
}

func TestResolveAgentAllowsDirectACPProviderManagedModel(t *testing.T) {
	t.Parallel()

	tests := []string{"blackbox", "cline"}
	for _, provider := range tests {
		t.Run("Should allow "+provider+" without resolved model", func(t *testing.T) {
			t.Parallel()

			homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
			if err != nil {
				t.Fatalf("ResolveHomePathsFrom() error = %v", err)
			}

			cfg := DefaultWithHome(homePaths)
			agent := AgentDef{
				Name:     "coder",
				Provider: provider,
				Prompt:   "prompt",
			}

			resolved, err := cfg.ResolveAgent(agent)
			if err != nil {
				t.Fatalf("ResolveAgent() error = %v", err)
			}
			if resolved.Model != "" {
				t.Fatalf("ResolveAgent() Model = %q, want provider-managed empty model", resolved.Model)
			}
			if resolved.RuntimeSources.Model != "" {
				t.Fatalf("ResolveAgent() model source = %q, want empty", resolved.RuntimeSources.Model)
			}
		})
	}
}

func TestResolveAgentRejectsPiProviderWithoutModel(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	cfg := DefaultWithHome(homePaths)
	cfg.Providers["custom-pi"] = ProviderConfig{
		Command:         "npx -y pi-acp@latest",
		Harness:         ProviderHarnessPiACP,
		RuntimeProvider: "custom",
		CredentialSlots: []ProviderCredentialSlot{
			apiKeyCredentialSlot("CUSTOM_API_KEY"),
		},
	}
	_, err = cfg.ResolveAgent(AgentDef{
		Name:     "coder",
		Provider: "custom-pi",
		Prompt:   "prompt",
	})
	if err == nil {
		t.Fatal("ResolveAgent() error = nil, want model required error")
	}
	wantErr := `agent model is required when provider "custom-pi" has no default model`
	if err.Error() != wantErr {
		t.Fatalf("ResolveAgent() error = %q, want %q", err.Error(), wantErr)
	}
}

func TestMCPServersMergeAgentAndProviderLevels(t *testing.T) {
	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	cfg := DefaultWithHome(homePaths)
	cfg.Providers["claude"] = ProviderConfig{
		MCPServers: []MCPServer{
			{Name: "github", Command: "npx"},
		},
	}

	agent := AgentDef{
		Name:     "coder",
		Provider: "claude",
		Prompt:   "prompt",
		MCPServers: []MCPServer{
			{Name: "memory", Command: "memory-server"},
		},
	}

	resolved, err := cfg.ResolveAgent(agent)
	if err != nil {
		t.Fatalf("ResolveAgent() error = %v", err)
	}

	if len(resolved.MCPServers) != 2 {
		t.Fatalf("ResolveAgent() MCPServers = %#v, want 2 entries", resolved.MCPServers)
	}
	if resolved.MCPServers[0].Name != "github" || resolved.MCPServers[1].Name != "memory" {
		t.Fatalf("ResolveAgent() MCPServers = %#v", resolved.MCPServers)
	}
}

func TestMergeMCPServersSameNameOverlaysFields(t *testing.T) {
	t.Parallel()

	merged := MergeMCPServers(
		[]MCPServer{{Name: "github", Command: "npx", SecretEnv: map[string]string{"TOKEN": "env:GITHUB_TOKEN"}}},
		[]MCPServer{{Name: "github", Args: []string{"-y"}, Env: map[string]string{"OTHER": "1"}}},
	)

	if len(merged) != 1 {
		t.Fatalf("MergeMCPServers() len = %d, want 1", len(merged))
	}
	if merged[0].Command != "npx" {
		t.Fatalf("MergeMCPServers() Command = %q, want %q", merged[0].Command, "npx")
	}
	if got, want := len(merged[0].Args), 1; got != want {
		t.Fatalf("MergeMCPServers() Args len = %d, want %d (%#v)", got, want, merged[0].Args)
	}
	if got, want := merged[0].Args[0], "-y"; got != want {
		t.Fatalf("MergeMCPServers() Args = %#v", merged[0].Args)
	}
	if merged[0].Env["OTHER"] != "1" || merged[0].SecretEnv["TOKEN"] != "env:GITHUB_TOKEN" {
		t.Fatalf("MergeMCPServers() Env = %#v SecretEnv = %#v", merged[0].Env, merged[0].SecretEnv)
	}
}

func TestMCPServerValidateSupportsRemoteOAuthPKCE(t *testing.T) {
	t.Parallel()

	server := MCPServer{
		Name:      "linear",
		Transport: MCPServerTransportSSE,
		URL:       "https://mcp.example/sse",
		Auth: MCPAuthConfig{
			Type:             MCPAuthTypeOAuth2PKCE,
			AuthorizationURL: "https://auth.example/authorize",
			TokenURL:         "https://auth.example/token",
			ClientID:         "client-1",
			Scopes:           []string{"read", "write"},
		},
	}
	if err := server.Validate("mcp_servers[0]"); err != nil {
		t.Fatalf("Validate(remote OAuth) error = %v", err)
	}

	server.Auth.TokenURL = ""
	if err := server.Validate("mcp_servers[0]"); err == nil {
		t.Fatal("Validate(missing token metadata) error = nil, want validation failure")
	}
}

func TestMCPServerValidateRejectsUnsafeStdioEnv(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		key  string
	}{
		{name: "Should reject Node options", key: "NODE_OPTIONS"},
		{name: "Should reject Python path", key: "PYTHONPATH"},
		{name: "Should reject Python home", key: "PYTHONHOME"},
		{name: "Should reject preload", key: "LD_PRELOAD"},
		{name: "Should reject Darwin dynamic loader variables", key: "DYLD_INSERT_LIBRARIES"},
		{name: "Should reject secret-like token variables", key: "GITHUB_TOKEN"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := MCPServer{
				Name:    "local",
				Command: "npx",
				Env:     map[string]string{tc.key: "value"},
			}
			err := server.Validate("mcp_servers[0]")
			if err == nil {
				t.Fatalf("Validate(%s) error = nil, want env validation failure", tc.key)
			}
		})
	}

	for _, tc := range tests[:5] {
		t.Run(strings.Replace(tc.name, "Should reject", "Should reject secret", 1), func(t *testing.T) {
			t.Parallel()

			server := MCPServer{
				Name:      "local",
				Command:   "npx",
				SecretEnv: map[string]string{tc.key: "vault:mcp/global/local/env/" + tc.key},
			}
			err := server.Validate("mcp_servers[0]")
			if err == nil || !strings.Contains(err.Error(), ".secret_env."+tc.key+" is forbidden") {
				t.Fatalf("Validate(secret_env %s) error = %v, want forbidden-key detail", tc.key, err)
			}
		})
	}
}

func TestMCPServerValidateRejectsRemoteProcessFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*MCPServer)
		field  string
	}{
		{
			name: "Should reject remote MCP args",
			mutate: func(server *MCPServer) {
				server.Args = []string{"--verbose"}
			},
			field: ".args",
		},
		{
			name: "Should reject remote MCP env",
			mutate: func(server *MCPServer) {
				server.Env = map[string]string{"LOG_LEVEL": "debug"}
			},
			field: ".env",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := MCPServer{
				Name:      "remote",
				Transport: MCPServerTransportHTTP,
				URL:       "https://mcp.example/mcp",
			}
			tc.mutate(&server)
			err := server.Validate("mcp_servers[0]")
			if err == nil || !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("Validate(remote MCP %s) error = %v, want field validation", tc.field, err)
			}
		})
	}
}

func TestRedactedMCPServerDoesNotExposeEnvSecretValues(t *testing.T) {
	t.Parallel()

	server := MCPServer{
		Name:    "github",
		Command: "npx",
		SecretEnv: map[string]string{
			"GITHUB_TOKEN": "env:GITHUB_TOKEN",
		},
	}
	redacted := RedactedMCPServer(server)
	if got := redacted.SecretEnv["GITHUB_TOKEN"]; got != RedactedValue() {
		t.Fatalf("redacted secret env = %q, want placeholder", got)
	}
	if server.SecretEnv["GITHUB_TOKEN"] != "env:GITHUB_TOKEN" {
		t.Fatalf("source secret env mutated = %#v", server.SecretEnv)
	}
}

func TestMergeMCPServersTrimmedNamesCollide(t *testing.T) {
	t.Parallel()

	merged := MergeMCPServers(
		[]MCPServer{{Name: "  github  ", Command: "npx"}},
		[]MCPServer{{Name: "github", Args: []string{"-y"}}},
	)

	if len(merged) != 1 {
		t.Fatalf("MergeMCPServers() len = %d, want 1", len(merged))
	}
	if got, want := merged[0].Command, "npx"; got != want {
		t.Fatalf("MergeMCPServers() Command = %q, want %q", got, want)
	}
	if got, want := len(merged[0].Args), 1; got != want {
		t.Fatalf("MergeMCPServers() Args len = %d, want %d (%#v)", got, want, merged[0].Args)
	}
	if got, want := merged[0].Args[0], "-y"; got != want {
		t.Fatalf("MergeMCPServers() Args[0] = %q, want %q", got, want)
	}
}

func TestOverrideMCPServersSameNameReplacesObject(t *testing.T) {
	t.Parallel()

	merged := OverrideMCPServers(
		[]MCPServer{
			{
				Name:      "github",
				Command:   "npx",
				Args:      []string{"-y"},
				SecretEnv: map[string]string{"TOKEN": "env:GITHUB_TOKEN"},
			},
		},
		[]MCPServer{{Name: "github", Command: "node"}},
	)

	if len(merged) != 1 {
		t.Fatalf("OverrideMCPServers() len = %d, want 1", len(merged))
	}
	if got, want := merged[0].Command, "node"; got != want {
		t.Fatalf("OverrideMCPServers() Command = %q, want %q", got, want)
	}
	if got := len(merged[0].Args); got != 0 {
		t.Fatalf("OverrideMCPServers() Args = %#v, want replacement semantics", merged[0].Args)
	}
	if got := len(merged[0].Env); got != 0 {
		t.Fatalf("OverrideMCPServers() Env = %#v, want replacement semantics", merged[0].Env)
	}
}

func TestOverrideMCPServersTrimmedNamesCollide(t *testing.T) {
	t.Parallel()

	merged := OverrideMCPServers(
		[]MCPServer{{Name: "  github  ", Command: "npx", Args: []string{"-y"}}},
		[]MCPServer{{Name: "github", Command: "node"}},
	)

	if len(merged) != 1 {
		t.Fatalf("OverrideMCPServers() len = %d, want 1", len(merged))
	}
	if got, want := merged[0].Command, "node"; got != want {
		t.Fatalf("OverrideMCPServers() Command = %q, want %q", got, want)
	}
	if got := len(merged[0].Args); got != 0 {
		t.Fatalf("OverrideMCPServers() Args = %#v, want replacement semantics", merged[0].Args)
	}
}

func TestResolveAgentMergesTopLevelProviderAndAgentMCPServers(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	cfg := DefaultWithHome(homePaths)
	cfg.MCPServers = []MCPServer{
		{Name: "global", Command: "global-command"},
	}
	cfg.Providers["claude"] = ProviderConfig{
		MCPServers: []MCPServer{
			{Name: "provider", Command: "provider-command"},
		},
	}

	agent := AgentDef{
		Name:     "coder",
		Provider: "claude",
		Prompt:   "prompt",
		MCPServers: []MCPServer{
			{Name: "agent", Command: "agent-command"},
		},
	}

	resolved, err := cfg.ResolveAgent(agent)
	if err != nil {
		t.Fatalf("ResolveAgent() error = %v", err)
	}

	if got, want := len(resolved.MCPServers), 3; got != want {
		t.Fatalf("ResolveAgent() MCPServers len = %d, want %d (%#v)", got, want, resolved.MCPServers)
	}
	if got, want := resolved.MCPServers[0].Name, "global"; got != want {
		t.Fatalf("ResolveAgent() MCPServers[0].Name = %q, want %q", got, want)
	}
	if got, want := resolved.MCPServers[1].Name, "provider"; got != want {
		t.Fatalf("ResolveAgent() MCPServers[1].Name = %q, want %q", got, want)
	}
	if got, want := resolved.MCPServers[2].Name, "agent"; got != want {
		t.Fatalf("ResolveAgent() MCPServers[2].Name = %q, want %q", got, want)
	}
}

func TestResolveProviderRejectsUnknownProvider(t *testing.T) {
	t.Parallel()

	cfg := Config{}
	if _, err := cfg.ResolveProvider("unknown"); err == nil {
		t.Fatal("ResolveProvider() error = nil, want non-nil")
	} else if !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("ResolveProvider() error = %v, want ErrProviderUnavailable", err)
	}
}

func TestResolveProviderMergesRuntimeOverrideHints(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	cfg := DefaultWithHome(homePaths)
	cfg.Providers["codex"] = ProviderConfig{
		Models: ProviderModelsConfig{
			Default: "gpt-manual",
			Curated: []ProviderModelConfig{
				{ID: "gpt-custom", DisplayName: "Custom GPT"},
				{ID: "gpt-mini", DisplayName: "Mini GPT"},
			},
		},
	}

	provider, err := cfg.ResolveProvider("codex")
	if err != nil {
		t.Fatalf("ResolveProvider(codex) error = %v", err)
	}
	if got, want := provider.Models.Default, "gpt-manual"; got != want {
		t.Fatalf("ResolveProvider(codex) Models.Default = %q, want %q", got, want)
	}
	wantModels := []ProviderModelConfig{
		{ID: "gpt-custom", DisplayName: "Custom GPT"},
		{ID: "gpt-mini", DisplayName: "Mini GPT"},
	}
	if !reflect.DeepEqual(provider.Models.Curated, wantModels) {
		t.Fatalf("ResolveProvider(codex) Models.Curated = %#v, want %#v", provider.Models.Curated, wantModels)
	}
}

func TestResolveProviderPreservesExplicitEmptyCuratedModels(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	cfg := DefaultWithHome(homePaths)
	cfg.Providers["codex"] = ProviderConfig{
		Models: ProviderModelsConfig{
			Curated: []ProviderModelConfig{},
		},
	}

	provider, err := cfg.ResolveProvider("codex")
	if err != nil {
		t.Fatalf("ResolveProvider(codex) error = %v", err)
	}
	if got := len(provider.Models.Curated); got != 0 {
		t.Fatalf("ResolveProvider(codex) Models.Curated len = %d, want 0", got)
	}
}

func TestLoadProviderRuntimeOverrideHintsFromTOML(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	writeFile(t, homePaths.ConfigFile, `
[providers.codex.models]
default = "gpt-manual"

[[providers.codex.models.curated]]
id = "gpt-custom"
display_name = "Custom GPT"

[[providers.codex.models.curated]]
id = "gpt-mini"
display_name = "Mini GPT"
`)

	cfg, err := LoadForHome(homePaths, withoutDotEnv())
	if err != nil {
		t.Fatalf("LoadForHome() error = %v", err)
	}
	provider, err := cfg.ResolveProvider("codex")
	if err != nil {
		t.Fatalf("ResolveProvider(codex) error = %v", err)
	}
	if got, want := provider.Models.Default, "gpt-manual"; got != want {
		t.Fatalf("ResolveProvider(codex) Models.Default = %q, want %q", got, want)
	}
	wantModels := []ProviderModelConfig{
		{ID: "gpt-custom", DisplayName: "Custom GPT"},
		{ID: "gpt-mini", DisplayName: "Mini GPT"},
	}
	if !reflect.DeepEqual(provider.Models.Curated, wantModels) {
		t.Fatalf("ResolveProvider(codex) Models.Curated = %#v, want %#v", provider.Models.Curated, wantModels)
	}
}

func TestLoadRejectsBlankProviderCuratedModelID(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	writeFile(t, homePaths.ConfigFile, `
[[providers.codex.models.curated]]
id = "gpt-custom"

[[providers.codex.models.curated]]
id = " "
`)

	_, err = LoadForHome(homePaths, withoutDotEnv())
	if err == nil {
		t.Fatal("LoadForHome() error = nil, want blank curated id validation")
	}
	if !strings.Contains(err.Error(), `providers.codex.models.curated[1].id is required`) {
		t.Fatalf("LoadForHome() error = %v, want curated id index detail", err)
	}
}

func TestLoadRejectsInvalidProviderModelsConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  string
		wantErr string
	}{
		{
			name: "Should reject duplicate curated model IDs",
			config: `
[[providers.codex.models.curated]]
id = "gpt-5.4"

[[providers.codex.models.curated]]
id = "gpt-5.4"
`,
			wantErr: `providers.codex.models.curated[1].id duplicates "gpt-5.4"`,
		},
		{
			name: "Should reject default reasoning effort outside allowed efforts",
			config: `
[[providers.codex.models.curated]]
id = "gpt-5.4"
reasoning_efforts = ["low", "medium"]
default_reasoning_effort = "high"
`,
			wantErr: `providers.codex.models.curated[0].default_reasoning_effort must be listed in reasoning_efforts`,
		},
		{
			name: "Should reject reasoning effort IDs with surrounding whitespace",
			config: `
[[providers.codex.models.curated]]
id = "gpt-5.4"
reasoning_efforts = [" high "]
`,
			wantErr: `providers.codex.models.curated[0].reasoning_efforts[0]`,
		},
		{
			name: "Should reject default reasoning effort with surrounding whitespace",
			config: `
[[providers.codex.models.curated]]
id = "gpt-5.4"
reasoning_efforts = ["high"]
default_reasoning_effort = " high "
`,
			wantErr: `providers.codex.models.curated[0].default_reasoning_effort`,
		},
		{
			name: "Should reject a negative cache read rate",
			config: `
[[providers.codex.models.curated]]
id = "gpt-5.4"
cost_cache_read_per_million = -0.1
`,
			wantErr: `providers.codex.models.curated[0].cost_cache_read_per_million must be finite and non-negative`,
		},
		{
			name: "Should reject a non-finite reasoning rate",
			config: `
[[providers.codex.models.curated]]
id = "gpt-5.4"
cost_reasoning_per_million = nan
`,
			wantErr: `providers.codex.models.curated[0].cost_reasoning_per_million must be finite and non-negative`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
			if err != nil {
				t.Fatalf("ResolveHomePathsFrom() error = %v", err)
			}
			if err := EnsureHomeLayout(homePaths); err != nil {
				t.Fatalf("EnsureHomeLayout() error = %v", err)
			}
			writeFile(t, homePaths.ConfigFile, tc.config)

			_, err = LoadForHome(homePaths, withoutDotEnv())
			if err == nil {
				t.Fatal("LoadForHome() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("LoadForHome() error = %v, want %q", err, tc.wantErr)
			}
		})
	}

	t.Run("Should reject a non-canonical configured reasoning effort", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		writeFile(t, homePaths.ConfigFile, `
[[providers.codex.models.curated]]
id = "gpt-5.4"
reasoning_efforts = ["ultra"]
`)

		_, err = LoadForHome(homePaths, withoutDotEnv())
		var invalid *reasoning.InvalidEffortError
		if !errors.As(err, &invalid) {
			t.Fatalf("LoadForHome() error = %T %v, want *reasoning.InvalidEffortError", err, err)
		}
		if invalid.Path != "providers.codex.models.curated[0].reasoning_efforts[0]" ||
			invalid.Value != "ultra" {
			t.Fatalf("InvalidEffortError = %#v, want curated effort path and ultra value", invalid)
		}
	})
}

func TestLoadRejectsRemovedProviderKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		config          string
		removedPath     string
		messageFragment string
	}{
		{
			name: "Should reject old default_model key",
			config: `
[providers.codex]
default_model = "gpt-5.4"
`,
			removedPath:     `providers.codex.default_model`,
			messageFragment: `use "providers.codex.models.default"`,
		},
		{
			name: "Should reject old supported_models key",
			config: `
[providers.codex]
supported_models = ["gpt-5.4"]
`,
			removedPath:     `providers.codex.supported_models`,
			messageFragment: `use "providers.codex.models.curated"`,
		},
		{
			name: "Should reject old supports_reasoning_effort key",
			config: `
[providers.codex]
supports_reasoning_effort = true
`,
			removedPath:     `providers.codex.supports_reasoning_effort`,
			messageFragment: `use "providers.codex.models.curated[].reasoning_efforts"`,
		},
		{
			name: "Should reject removed provider aliases key",
			config: `
[providers.codex]
aliases = ["openai"]
`,
			removedPath:     `providers.codex.aliases`,
			messageFragment: "aliases was removed; reference providers by canonical name",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
			if err != nil {
				t.Fatalf("ResolveHomePathsFrom() error = %v", err)
			}
			if err := EnsureHomeLayout(homePaths); err != nil {
				t.Fatalf("EnsureHomeLayout() error = %v", err)
			}
			writeFile(t, homePaths.ConfigFile, tc.config)

			_, err = LoadForHome(homePaths, withoutDotEnv())
			if err == nil {
				t.Fatal("LoadForHome() error = nil, want removed key error")
			}
			message := err.Error()
			if !strings.Contains(message, `removed config key "`+tc.removedPath+`"`) ||
				!strings.Contains(message, tc.messageFragment) {
				t.Fatalf(
					"LoadForHome() error = %v, want removed path %q and message fragment %q",
					err,
					tc.removedPath,
					tc.messageFragment,
				)
			}
		})
	}
}

func TestModelCatalogModelsDevConfigValidatesDefaultsAndOverrides(t *testing.T) {
	t.Parallel()

	defaults := DefaultModelCatalogConfig().Sources.ModelsDev
	if !defaults.EffectiveEnabled() {
		t.Fatal("DefaultModelCatalogConfig().ModelsDev enabled = false, want true")
	}
	if got, want := defaults.EffectiveEndpoint(), defaultModelsDevEndpoint; got != want {
		t.Fatalf("ModelsDev EffectiveEndpoint() = %q, want %q", got, want)
	}
	if got, want := defaults.EffectiveTTL(), defaultModelsDevTTL; got != want {
		t.Fatalf("ModelsDev EffectiveTTL() = %q, want %q", got, want)
	}
	if got, want := defaults.EffectiveTimeout(), defaultModelsDevTimeout; got != want {
		t.Fatalf("ModelsDev EffectiveTimeout() = %q, want %q", got, want)
	}

	enabled := false
	override := ModelsDevSourceConfig{
		Enabled:  &enabled,
		Endpoint: "https://models.example.test/api.json",
		TTL:      "2h",
		Timeout:  "5s",
	}
	if err := override.Validate("model_catalog.sources.models_dev"); err != nil {
		t.Fatalf("ModelsDev Validate(valid override) error = %v", err)
	}
	if override.EffectiveEnabled() {
		t.Fatal("ModelsDev EffectiveEnabled() = true, want explicit false")
	}
	if got, want := override.EffectiveEndpoint(), "https://models.example.test/api.json"; got != want {
		t.Fatalf("ModelsDev EffectiveEndpoint() = %q, want %q", got, want)
	}

	tests := []struct {
		name           string
		value          ModelsDevSourceConfig
		wantErr        string
		wantErrDetails string
	}{
		{
			name:    "Should reject invalid endpoint",
			value:   ModelsDevSourceConfig{Endpoint: "file:///tmp/models.json"},
			wantErr: "model_catalog.sources.models_dev.endpoint must be an absolute HTTP(S) URL",
		},
		{
			name:           "Should reject invalid TTL",
			value:          ModelsDevSourceConfig{TTL: "soon"},
			wantErr:        "model_catalog.sources.models_dev.ttl must be a positive duration",
			wantErrDetails: `time: invalid duration "soon"`,
		},
		{
			name:    "Should reject invalid timeout",
			value:   ModelsDevSourceConfig{Timeout: "0s"},
			wantErr: "model_catalog.sources.models_dev.timeout must be a positive duration",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.value.Validate("model_catalog.sources.models_dev")
			if err == nil {
				t.Fatal("ModelsDev Validate() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("ModelsDev Validate() error = %v, want %q", err, tc.wantErr)
			}
			if tc.wantErrDetails != "" && !strings.Contains(err.Error(), tc.wantErrDetails) {
				t.Fatalf("ModelsDev Validate() error = %v, want parse details %q", err, tc.wantErrDetails)
			}
		})
	}
}

func TestMarketplaceCatalogConfigValidatesDefaultsAndOverrides(t *testing.T) {
	t.Parallel()

	t.Run("Should expose the curated catalog defaults", func(t *testing.T) {
		t.Parallel()

		defaults := DefaultMarketplaceRuntimeConfig().Catalog
		if got, want := defaults.EffectiveBaseURL(), DefaultMarketplaceCatalogBaseURL; got != want {
			t.Fatalf("MarketplaceCatalog EffectiveBaseURL() = %q, want %q", got, want)
		}
		if got, want := defaults.EffectiveTTL(), DefaultMarketplaceCatalogTTL; got != want {
			t.Fatalf("MarketplaceCatalog EffectiveTTL() = %q, want %q", got, want)
		}
		if got, want := defaults.EffectiveTimeout(), DefaultMarketplaceCatalogTimeout; got != want {
			t.Fatalf("MarketplaceCatalog EffectiveTimeout() = %q, want %q", got, want)
		}
	})

	t.Run("Should accept valid curated catalog overrides", func(t *testing.T) {
		t.Parallel()

		valid := MarketplaceCatalogConfig{
			BaseURL: "https://catalog.example.test/v1",
			TTL:     "30m",
			Timeout: "5s",
		}
		if err := valid.Validate("marketplace.catalog"); err != nil {
			t.Fatalf("MarketplaceCatalog Validate(valid) error = %v", err)
		}
	})

	tests := []struct {
		name    string
		value   MarketplaceCatalogConfig
		wantErr string
	}{
		{
			name:    "Should reject a non-HTTP base URL",
			value:   MarketplaceCatalogConfig{BaseURL: "file:///tmp/catalog"},
			wantErr: "marketplace.catalog.base_url must be an absolute HTTP(S) URL",
		},
		{
			name:    "Should reject a non-positive TTL",
			value:   MarketplaceCatalogConfig{TTL: "0s"},
			wantErr: "marketplace.catalog.ttl must be a positive duration",
		},
		{
			name:    "Should reject a non-positive timeout",
			value:   MarketplaceCatalogConfig{Timeout: "-1s"},
			wantErr: "marketplace.catalog.timeout must be a positive duration",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.value.Validate("marketplace.catalog")
			if err == nil {
				t.Fatal("MarketplaceCatalog Validate() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("MarketplaceCatalog Validate() error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestProviderModelsDiscoveryConfigRejectsUnsafeConfiguration(t *testing.T) {
	t.Parallel()

	enabled := true
	tests := []struct {
		name    string
		value   ProviderModelsDiscoveryConfig
		wantErr string
	}{
		{
			name:    "Should reject multiline command",
			value:   ProviderModelsDiscoveryConfig{Command: "models\nlist"},
			wantErr: "providers.codex.models.discovery.command must be a single-line command",
		},
		{
			name: "Should reject ambiguous command and endpoint",
			value: ProviderModelsDiscoveryConfig{
				Command:  "models list",
				Endpoint: "https://models.example.test",
			},
			wantErr: "providers.codex.models.discovery.command and providers.codex.models.discovery.endpoint are mutually exclusive",
		},
		{
			name:    "Should reject invalid endpoint",
			value:   ProviderModelsDiscoveryConfig{Endpoint: "ftp://models.example.test"},
			wantErr: "providers.codex.models.discovery.endpoint must be an absolute HTTP(S) URL",
		},
		{
			name:    "Should reject enabled discovery without source",
			value:   ProviderModelsDiscoveryConfig{Enabled: &enabled},
			wantErr: "providers.codex.models.discovery requires command or endpoint when enabled",
		},
		{
			name:    "Should reject invalid timeout",
			value:   ProviderModelsDiscoveryConfig{Command: "models list", Timeout: "-1s"},
			wantErr: "providers.codex.models.discovery.timeout must be a positive duration",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.value.Validate("providers.codex.models.discovery")
			if err == nil {
				t.Fatal("Discovery Validate() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Discovery Validate() error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestResolveAgentDefaultsToolsAndPermissions(t *testing.T) {
	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	cfg := DefaultWithHome(homePaths)
	agent := AgentDef{
		Name:     "coder",
		Provider: "claude",
		Prompt:   "prompt",
	}

	resolved, err := cfg.ResolveAgent(agent)
	if err != nil {
		t.Fatalf("ResolveAgent() error = %v", err)
	}
	if len(resolved.Tools) != 0 {
		t.Fatalf("ResolveAgent() Tools = %#v, want empty default", resolved.Tools)
	}
	if resolved.Permissions != string(PermissionModeApproveAll) {
		t.Fatalf("ResolveAgent() Permissions = %q, want %q", resolved.Permissions, PermissionModeApproveAll)
	}
}

func TestResolveAgentFallsBackToDefaultsProvider(t *testing.T) {
	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	cfg := DefaultWithHome(homePaths)
	cfg.Defaults.Provider = "claude"
	agent := AgentDef{
		Name:   DefaultAgentName,
		Prompt: "prompt",
	}

	resolved, err := cfg.ResolveAgent(agent)
	if err != nil {
		t.Fatalf("ResolveAgent() error = %v", err)
	}
	if resolved.Provider != "claude" {
		t.Fatalf("ResolveAgent() Provider = %q, want %q", resolved.Provider, "claude")
	}
}

func TestResolveSessionAgent(t *testing.T) {
	t.Parallel()

	t.Run("Should match ResolveAgent when provider override is empty", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}

		cfg := DefaultWithHome(homePaths)
		cfg.MCPServers = []MCPServer{
			{Name: "global", Command: "global-command"},
		}

		agent := AgentDef{
			Name:        "coder",
			Provider:    "claude",
			Command:     "agent-command",
			Model:       "agent-model",
			Permissions: string(PermissionModeApproveReads),
			Prompt:      "prompt",
			Tools:       []string{"agh__skill_view"},
			Toolsets:    []string{"agh__catalog"},
			DenyTools:   []string{"agh__task_*"},
			MCPServers: []MCPServer{
				{Name: "agent", Command: "agent-command"},
			},
		}

		got, err := cfg.ResolveSessionAgent(agent, "")
		if err != nil {
			t.Fatalf("ResolveSessionAgent() error = %v", err)
		}

		want, err := cfg.ResolveAgent(agent)
		if err != nil {
			t.Fatalf("ResolveAgent() error = %v", err)
		}

		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ResolveSessionAgent() = %#v, want %#v", got, want)
		}
	})

	t.Run("Should preserve agent runtime fields when override matches agent provider", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}

		cfg := DefaultWithHome(homePaths)
		cfg.Providers["claude"] = ProviderConfig{
			Command: "provider-claude-command",
			Models:  ProviderModelsConfig{Default: "provider-claude-model"},
		}

		agent := AgentDef{
			Name:     "coder",
			Provider: "claude",
			Command:  "agent-command",
			Model:    "agent-model",
			Prompt:   "prompt",
		}

		resolved, err := cfg.ResolveSessionAgent(agent, "claude")
		if err != nil {
			t.Fatalf("ResolveSessionAgent() error = %v", err)
		}
		if got, want := resolved.Command, "agent-command"; got != want {
			t.Fatalf("ResolveSessionAgent() Command = %q, want %q", got, want)
		}
		if got, want := resolved.Model, "agent-model"; got != want {
			t.Fatalf("ResolveSessionAgent() Model = %q, want %q", got, want)
		}
	})

	t.Run("Should preserve agent runtime fields when override matches default provider", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}

		cfg := DefaultWithHome(homePaths)
		cfg.Defaults.Provider = "claude"
		cfg.Providers["claude"] = ProviderConfig{
			Command: "provider-claude-command",
			Models:  ProviderModelsConfig{Default: "provider-claude-model"},
		}

		agent := AgentDef{
			Name:    "coder",
			Command: "agent-command",
			Model:   "agent-model",
			Prompt:  "prompt",
		}

		resolved, err := cfg.ResolveSessionAgent(agent, "claude")
		if err != nil {
			t.Fatalf("ResolveSessionAgent() error = %v", err)
		}
		if got, want := resolved.Command, "agent-command"; got != want {
			t.Fatalf("ResolveSessionAgent() Command = %q, want %q", got, want)
		}
		if got, want := resolved.Model, "agent-model"; got != want {
			t.Fatalf("ResolveSessionAgent() Model = %q, want %q", got, want)
		}
	})

	t.Run("Should use workspace-merged runtime fields from the override provider", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}

		cfg := DefaultWithHome(homePaths)
		cfg.MCPServers = []MCPServer{
			{Name: "global", Command: "global-command"},
		}
		cfg.Providers["claude"] = ProviderConfig{
			Command: "workspace-claude-command",
			Models:  ProviderModelsConfig{Default: "workspace-claude-model"},
			MCPServers: []MCPServer{
				{Name: "provider-claude", Command: "provider-claude-command"},
			},
		}
		cfg.Providers["codex"] = ProviderConfig{
			Command: "workspace-codex-command",
			Models:  ProviderModelsConfig{Default: "workspace-codex-model"},
			MCPServers: []MCPServer{
				{Name: "provider-codex", Command: "provider-codex-command"},
				{Name: "shared-provider", Command: "shared-provider-codex", Args: []string{"--codex"}},
			},
		}

		agent := AgentDef{
			Name:     "coder",
			Provider: "claude",
			Command:  "agent-command",
			Model:    "agent-model",
			Prompt:   "prompt",
			MCPServers: []MCPServer{
				{Name: "agent", Command: "agent-command"},
			},
		}

		resolved, err := cfg.ResolveSessionAgent(agent, "codex")
		if err != nil {
			t.Fatalf("ResolveSessionAgent() error = %v", err)
		}

		if got, want := resolved.Provider, "codex"; got != want {
			t.Fatalf("ResolveSessionAgent() Provider = %q, want %q", got, want)
		}
		if got, want := resolved.Command, "workspace-codex-command"; got != want {
			t.Fatalf("ResolveSessionAgent() Command = %q, want %q", got, want)
		}
		if got, want := resolved.Model, "workspace-codex-model"; got != want {
			t.Fatalf("ResolveSessionAgent() Model = %q, want %q", got, want)
		}
		if resolved.Command == agent.Command {
			t.Fatalf(
				"ResolveSessionAgent() Command = %q, want provider-owned command instead of agent override",
				resolved.Command,
			)
		}
		if resolved.Model == agent.Model {
			t.Fatalf(
				"ResolveSessionAgent() Model = %q, want provider-owned default instead of agent override",
				resolved.Model,
			)
		}
		if got, want := resolved.AuthMode, ProviderAuthModeNativeCLI; got != want {
			t.Fatalf("ResolveSessionAgent() AuthMode = %q, want %q", got, want)
		}
		if len(resolved.CredentialSlots) != 0 {
			t.Fatalf("ResolveSessionAgent() CredentialSlots = %#v, want no native CLI slots", resolved.CredentialSlots)
		}

		if got, want := len(resolved.MCPServers), 4; got != want {
			t.Fatalf("ResolveSessionAgent() MCPServers len = %d, want %d (%#v)", got, want, resolved.MCPServers)
		}
		if got, want := resolved.MCPServers[0].Name, "global"; got != want {
			t.Fatalf("ResolveSessionAgent() MCPServers[0].Name = %q, want %q", got, want)
		}
		if got, want := mcpServerByName(
			t,
			resolved.MCPServers,
			"provider-codex",
		).Command, "provider-codex-command"; got != want {
			t.Fatalf("ResolveSessionAgent() provider-codex Command = %q, want %q", got, want)
		}
		if got, want := mcpServerByName(
			t,
			resolved.MCPServers,
			"shared-provider",
		).Command, "shared-provider-codex"; got != want {
			t.Fatalf("ResolveSessionAgent() shared-provider Command = %q, want %q", got, want)
		}
		if hasMCPServer(resolved.MCPServers, "provider-claude") {
			t.Fatalf(
				"ResolveSessionAgent() MCPServers = %#v, want provider-owned layer from selected provider only",
				resolved.MCPServers,
			)
		}
		if !hasMCPServer(resolved.MCPServers, "agent") {
			t.Fatalf("ResolveSessionAgent() MCPServers = %#v, want agent-local layer preserved", resolved.MCPServers)
		}
	})

	t.Run("Should use a runtime model override after selecting the override provider", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}

		cfg := DefaultWithHome(homePaths)
		cfg.Providers["codex"] = ProviderConfig{
			Command: "workspace-codex-command",
			Models:  ProviderModelsConfig{Default: "workspace-codex-model"},
		}

		agent := AgentDef{
			Name:     "coder",
			Provider: "claude",
			Command:  "agent-command",
			Model:    "agent-model",
			Prompt:   "prompt",
		}

		resolved, err := cfg.ResolveSessionAgentWithRuntime(agent, "codex", "profile-model")
		if err != nil {
			t.Fatalf("ResolveSessionAgentWithRuntime() error = %v", err)
		}
		if got, want := resolved.Provider, "codex"; got != want {
			t.Fatalf("ResolveSessionAgentWithRuntime() Provider = %q, want %q", got, want)
		}
		if got, want := resolved.Command, "workspace-codex-command"; got != want {
			t.Fatalf("ResolveSessionAgentWithRuntime() Command = %q, want %q", got, want)
		}
		if got, want := resolved.Model, "profile-model"; got != want {
			t.Fatalf("ResolveSessionAgentWithRuntime() Model = %q, want %q", got, want)
		}
	})

	t.Run("Should reject an unknown override provider with the wrapped provider error", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}

		cfg := DefaultWithHome(homePaths)
		agent := AgentDef{
			Name:     "coder",
			Provider: "claude",
			Prompt:   "prompt",
		}

		_, err = cfg.ResolveSessionAgent(agent, "missing")
		if err == nil {
			t.Fatal("ResolveSessionAgent() error = nil, want unknown provider failure")
		}
		if !errors.Is(err, ErrProviderUnavailable) {
			t.Fatalf("ResolveSessionAgent() error = %v, want ErrProviderUnavailable", err)
		}
		if !strings.Contains(err.Error(), `resolve session agent with provider "missing"`) {
			t.Fatalf("ResolveSessionAgent() error = %q, want session override context", err.Error())
		}
		if !strings.Contains(err.Error(), `unknown provider "missing"`) {
			t.Fatalf("ResolveSessionAgent() error = %q, want unknown provider detail", err.Error())
		}
	})
}

func TestMCPServerValidateRejectsMissingFields(t *testing.T) {
	t.Parallel()

	if err := (MCPServer{}).Validate("mcp"); err == nil {
		t.Fatal("MCPServer.Validate() error = nil, want non-nil")
	}
}

func mcpServerByName(t *testing.T, servers []MCPServer, name string) MCPServer {
	t.Helper()

	for _, server := range servers {
		if server.Name == name {
			return server
		}
	}

	t.Fatalf("MCP server %q not found in %#v", name, servers)

	return MCPServer{}
}

func hasMCPServer(servers []MCPServer, name string) bool {
	for _, server := range servers {
		if server.Name == name {
			return true
		}
	}

	return false
}
