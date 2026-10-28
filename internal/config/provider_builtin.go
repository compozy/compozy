package config

import (
	"runtime"
	"strings"
)

// agyACPServerCommand selects the official Antigravity ACP server executable
// for one GOOS. Linux needs the empty --uid= launch argument; macOS uses the
// same archive basename without it; Windows uses the .exe. Unknown targets
// keep the Linux command so they report a missing executable instead of an
// unsupported provider claim.
func agyACPServerCommand(goos string) string {
	switch goos {
	case "windows":
		return "agy_acp_server.exe"
	case "darwin":
		return "agy_acp_server.par"
	default:
		return "agy_acp_server.par --uid="
	}
}

var builtinProviders = map[string]ProviderConfig{
	providerAgyKey: {
		Command:       agyACPServerCommand(runtime.GOOS),
		DisplayName:   "Google Antigravity",
		Harness:       ProviderHarnessACP,
		AuthMode:      ProviderAuthModeNativeCLI,
		EnvPolicy:     ProviderEnvPolicyFiltered,
		HomePolicy:    ProviderHomePolicyOperator,
		AuthACPMethod: "oauth-personal",
		Models:        ProviderModelsConfig{Reasoning: ProviderReasoningConfig{Apply: ReasoningApplyACPOption}},
	},
	providerClaudeKey: {
		Command:      claudeProviderCommand,
		DisplayName:  "Claude Code",
		Harness:      ProviderHarnessACP,
		AuthLoginCmd: "claude auth login",
		Models:       builtinClaudeModelsConfig(),
	},
	providerCodexKey: {
		Command:      "npx -y @agentclientprotocol/codex-acp@latest",
		DisplayName:  "Codex",
		Harness:      ProviderHarnessACP,
		AuthLoginCmd: "codex login",
		Models:       builtinCodexModelsConfig(),
	},
	providerGeminiKey: {
		Command:     "gemini --acp",
		DisplayName: "Gemini CLI",
		Harness:     ProviderHarnessACP,
		Models: ProviderModelsConfig{
			Default: providerGemini31ProPreviewPath,
			Curated: []ProviderModelConfig{
				{ID: providerGemini31ProPreviewPath, DisplayName: "Gemini 3.1 Pro Preview"},
			},
		},
	},
	providerOpencodeKey: {
		Command:      "opencode acp",
		DisplayName:  "OpenCode",
		Harness:      ProviderHarnessACP,
		AuthLoginCmd: "opencode auth login",
		Models:       ProviderModelsConfig{Reasoning: ProviderReasoningConfig{Apply: ReasoningApplyACPOption}},
	},
	providerKimiCLIValue: {
		Command:     "kimi acp",
		DisplayName: "Kimi CLI",
		Harness:     ProviderHarnessACP,
	},
	providerQwenCodeValue: {
		Command:     "npx -y @qwen-code/qwen-code@latest --acp --experimental-skills",
		DisplayName: "Qwen Code",
		Harness:     ProviderHarnessACP,
		Models: ProviderModelsConfig{
			Default: providerQwen36PlusPath,
			Curated: []ProviderModelConfig{
				{ID: providerQwen36PlusPath, DisplayName: "Qwen3.6 Plus"},
			},
		},
	},
	"copilot": {
		Command:     "copilot --acp --stdio",
		DisplayName: "GitHub Copilot CLI",
		Harness:     ProviderHarnessACP,
	},
	"cursor": {
		Command:     "cursor-agent acp",
		DisplayName: "Cursor Agent",
		Harness:     ProviderHarnessACP,
	},
	"pi": {
		Command:         piACPCommand,
		DisplayName:     "Pi",
		Harness:         ProviderHarnessPiACP,
		RuntimeProvider: providerAnthropicKey,
		AuthLoginCmd:    piACPAuthLoginCommand,
		Models: ProviderModelsConfig{
			Default: modelClaudeOpus47ID,
			Curated: []ProviderModelConfig{
				{ID: modelClaudeOpus47ID, DisplayName: "Claude Opus 4.7"},
			},
		},
	},
	providerOpenrouterKey: {
		Command:         piACPCommand,
		DisplayName:     "OpenRouter",
		Harness:         ProviderHarnessPiACP,
		RuntimeProvider: providerOpenrouterKey,
		CredentialSlots: []ProviderCredentialSlot{apiKeyCredentialSlot("OPENROUTER_API_KEY")},
		Models: ProviderModelsConfig{
			Default: providerOpenaiGpt54Path,
			Curated: []ProviderModelConfig{
				{ID: providerOpenaiGpt54Path, DisplayName: "OpenAI GPT-5.4"},
			},
		},
	},
	providerZaiKey: {
		Command:         piACPCommand,
		DisplayName:     "z.ai",
		Harness:         ProviderHarnessPiACP,
		RuntimeProvider: providerZaiKey,
		CredentialSlots: []ProviderCredentialSlot{apiKeyCredentialSlot("ZAI_API_KEY")},
		Models: ProviderModelsConfig{
			Default: providerGlm46Path,
			Curated: []ProviderModelConfig{
				{ID: providerGlm46Path, DisplayName: "GLM-4.6"},
			},
		},
	},
	providerMoonshotKey: {
		Command:         piACPCommand,
		DisplayName:     "Moonshot / Kimi",
		Harness:         ProviderHarnessPiACP,
		RuntimeProvider: providerKimiCodingValue,
		CredentialSlots: []ProviderCredentialSlot{apiKeyCredentialSlot("KIMI_API_KEY")},
		Models: ProviderModelsConfig{
			Default: providerKimiK2ThinkingValue,
			Curated: []ProviderModelConfig{
				{ID: providerKimiK2ThinkingValue, DisplayName: "Kimi K2 Thinking"},
			},
		},
	},
	providerVercelAIGatewayValue: {
		Command:         piACPCommand,
		DisplayName:     "Vercel AI Gateway",
		Harness:         ProviderHarnessPiACP,
		RuntimeProvider: providerVercelAIGatewayValue,
		CredentialSlots: []ProviderCredentialSlot{apiKeyCredentialSlot("AI_GATEWAY_API_KEY")},
		Models: ProviderModelsConfig{
			Default: providerAnthropicClaudeOpus47Path,
			Curated: []ProviderModelConfig{
				{ID: providerAnthropicClaudeOpus47Path, DisplayName: "Anthropic Claude Opus 4.7"},
			},
		},
	},
	providerXaiKey: {
		Command:         piACPCommand,
		DisplayName:     "xAI",
		Harness:         ProviderHarnessPiACP,
		RuntimeProvider: providerXaiKey,
		CredentialSlots: []ProviderCredentialSlot{apiKeyCredentialSlot("XAI_API_KEY")},
		Models: ProviderModelsConfig{
			Default: providerGrok4FastNonReasoningValue,
			Curated: []ProviderModelConfig{
				{ID: providerGrok4FastNonReasoningValue, DisplayName: "Grok 4 Fast Non-Reasoning"},
			},
		},
	},
}

// BuiltinProviders returns a deep copy of the built-in provider registry.
func BuiltinProviders() map[string]ProviderConfig {
	cloned := cloneProviders(builtinProviders)
	for name, provider := range cloned {
		provider.CommandIsBuiltin = true
		cloned[name] = provider
	}
	return cloned
}

// CanonicalProviderName resolves known builtin aliases to the stable provider id.
func CanonicalProviderName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	if _, ok := builtinProviders[trimmed]; ok {
		return trimmed
	}
	lower := strings.ToLower(trimmed)
	if _, ok := builtinProviders[lower]; ok {
		return lower
	}
	if canonical, ok := builtinProviderAliases[lower]; ok {
		return canonical
	}
	if canonical, ok := canonicalRetiredProviderName(lower); ok {
		return canonical
	}
	return trimmed
}

func apiKeyCredentialSlot(targetEnv string) ProviderCredentialSlot {
	return apiKeyCredentialSlotWithRequired(targetEnv, true)
}

func apiKeyCredentialSlotWithRequired(targetEnv string, required bool) ProviderCredentialSlot {
	return ProviderCredentialSlot{
		Name:      providerAPIKeyCredential,
		TargetEnv: targetEnv,
		SecretRef: "env:" + targetEnv,
		Kind:      providerAPIKeyCredential,
		Required:  required,
	}
}
