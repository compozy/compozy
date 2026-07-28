package config

import "strings"

var builtinProviders = map[string]ProviderConfig{
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
		Command:      "npx -y opencode-ai@latest acp",
		DisplayName:  "OpenCode",
		Harness:      ProviderHarnessACP,
		AuthLoginCmd: "opencode auth login",
	},
	providerBlackboxKey: {
		Command:     "blackbox --experimental-acp",
		DisplayName: "BLACKBOX AI",
		Harness:     ProviderHarnessACP,
	},
	providerClineKey: {
		Command:     "npx -y cline@latest --acp",
		DisplayName: "Cline",
		Harness:     ProviderHarnessACP,
	},
	providerGooseKey: {
		Command:     "goose acp",
		DisplayName: "Goose",
		Harness:     ProviderHarnessACP,
	},
	providerHermesKey: {
		Command:     "hermes acp",
		DisplayName: "Hermes",
		Harness:     ProviderHarnessACP,
	},
	providerJunieKey: {
		Command:     "junie --acp true",
		DisplayName: "Junie",
		Harness:     ProviderHarnessACP,
	},
	providerKimiCLIValue: {
		Command:     "kimi acp",
		DisplayName: "Kimi CLI",
		Harness:     ProviderHarnessACP,
	},
	providerOpenclawKey: {
		Command:     "openclaw acp",
		DisplayName: "OpenClaw",
		Harness:     ProviderHarnessACP,
		SessionMCP:  new(false),
	},
	providerOpenhandsKey: {
		Command:     "openhands acp",
		DisplayName: "OpenHands",
		Harness:     ProviderHarnessACP,
	},
	providerQoderKey: {
		Command:     "npx -y @qoder-ai/qodercli@latest --acp",
		DisplayName: "Qoder CLI",
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
	"kiro": {
		Command:     "kiro-cli-chat acp",
		DisplayName: "Kiro CLI",
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
	providerMinimaxKey: {
		Command:         piACPCommand,
		DisplayName:     "MiniMax",
		Harness:         ProviderHarnessPiACP,
		RuntimeProvider: providerMinimaxKey,
		CredentialSlots: []ProviderCredentialSlot{apiKeyCredentialSlot("MINIMAX_API_KEY")},
		Models: ProviderModelsConfig{
			Default: providerMiniMaxM21Path,
			Curated: []ProviderModelConfig{
				{ID: providerMiniMaxM21Path, DisplayName: "MiniMax M2.1"},
			},
		},
	},
	providerMistralKey: {
		Command:         piACPCommand,
		DisplayName:     "Mistral",
		Harness:         ProviderHarnessPiACP,
		RuntimeProvider: providerMistralKey,
		CredentialSlots: []ProviderCredentialSlot{apiKeyCredentialSlot("MISTRAL_API_KEY")},
		Models: ProviderModelsConfig{
			Default: providerDevstralMediumLatestValue,
			Curated: []ProviderModelConfig{
				{ID: providerDevstralMediumLatestValue, DisplayName: "Devstral Medium Latest"},
			},
		},
	},
	providerGroqKey: {
		Command:         piACPCommand,
		DisplayName:     "Groq",
		Harness:         ProviderHarnessPiACP,
		RuntimeProvider: providerGroqKey,
		CredentialSlots: []ProviderCredentialSlot{apiKeyCredentialSlot("GROQ_API_KEY")},
		Models: ProviderModelsConfig{
			Default: providerOpenaiGptOss120bPath,
			Curated: []ProviderModelConfig{
				{ID: providerOpenaiGptOss120bPath, DisplayName: "OpenAI GPT-OSS 120B"},
			},
		},
	},
}

// BuiltinProviders returns a deep copy of the built-in provider registry.
func BuiltinProviders() map[string]ProviderConfig {
	return cloneProviders(builtinProviders)
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
