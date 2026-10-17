package config

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	burnttoml "github.com/BurntSushi/toml"
)

const (
	retiredProviderBlackbox  = "blackbox"
	retiredProviderCline     = "cline"
	retiredProviderGoose     = "goose"
	retiredProviderJunie     = "junie"
	retiredProviderKiro      = "kiro"
	retiredProviderOpenHands = "openhands"
	retiredProviderQoder     = "qoder"
	retiredProviderHermes    = "hermes"
	retiredProviderOpenClaw  = "openclaw"
	retiredProviderMiniMax   = "minimax"
	retiredProviderMistral   = "mistral"
	retiredProviderGroq      = "groq"
)

// retiredBuiltinProviderRemovalRelease names the release that deletes this boundary shim.
const retiredBuiltinProviderRemovalRelease = "v0.5.0"

// retiredBuiltinProviders holds the former builtin definitions of providers removed from the
// registry. A reference that names one keeps resolving for one release with a deprecation
// warning carrying the equivalent [providers.<id>] entry. Remove this boundary shim in v0.5.0;
// references then resolve like any other unknown provider.
var retiredBuiltinProviders = map[string]ProviderConfig{
	retiredProviderBlackbox: {
		Command:     "blackbox --experimental-acp",
		DisplayName: "BLACKBOX AI",
		Harness:     ProviderHarnessACP,
	},
	retiredProviderCline:     {Command: "npx -y cline@latest --acp", DisplayName: "Cline", Harness: ProviderHarnessACP},
	retiredProviderGoose:     {Command: "goose acp", DisplayName: "Goose", Harness: ProviderHarnessACP},
	retiredProviderJunie:     {Command: "junie --acp true", DisplayName: "Junie", Harness: ProviderHarnessACP},
	retiredProviderKiro:      {Command: "kiro-cli-chat acp", DisplayName: "Kiro CLI", Harness: ProviderHarnessACP},
	retiredProviderOpenHands: {Command: "openhands acp", DisplayName: "OpenHands", Harness: ProviderHarnessACP},
	retiredProviderQoder: {
		Command:     "npx -y @qoder-ai/qodercli@latest --acp",
		DisplayName: "Qoder CLI",
		Harness:     ProviderHarnessACP,
	},
	retiredProviderHermes: {Command: "hermes acp", DisplayName: "Hermes", Harness: ProviderHarnessACP},
	retiredProviderOpenClaw: {
		Command:     "openclaw acp",
		DisplayName: "OpenClaw",
		Harness:     ProviderHarnessACP,
		SessionMCP:  new(false),
	},
	retiredProviderMiniMax: retiredPiACPProvider(
		"MiniMax",
		retiredProviderMiniMax,
		"MINIMAX_API_KEY",
		"MiniMax-M2.1",
		"MiniMax M2.1",
	),
	retiredProviderMistral: retiredPiACPProvider(
		"Mistral", retiredProviderMistral, "MISTRAL_API_KEY", "devstral-medium-latest", "Devstral Medium Latest",
	),
	retiredProviderGroq: retiredPiACPProvider(
		"Groq",
		retiredProviderGroq,
		"GROQ_API_KEY",
		"openai/gpt-oss-120b",
		"OpenAI GPT-OSS 120B",
	),
}

// retiredBuiltinProviderAliases keeps the aliases of retired providers canonical until v0.5.0.
var retiredBuiltinProviderAliases = map[string]string{
	"blackbox-ai":   retiredProviderBlackbox,
	"blackboxai":    retiredProviderBlackbox,
	"cline-cli":     retiredProviderCline,
	"goose-cli":     retiredProviderGoose,
	"hermes-agent":  retiredProviderHermes,
	"junie-cli":     retiredProviderJunie,
	"open-hands":    retiredProviderOpenHands,
	"openhands-cli": retiredProviderOpenHands,
	"openclaw-cli":  retiredProviderOpenClaw,
	"qoder-cli":     retiredProviderQoder,
	"minimax-ai":    retiredProviderMiniMax,
	"minimax-cn":    retiredProviderMiniMax,
	"mistralai":     retiredProviderMistral,
	"mistral-ai":    retiredProviderMistral,
}

var retiredBuiltinProviderWarnings sync.Map

func retiredPiACPProvider(displayName, runtimeProvider, credentialEnv, model, modelName string) ProviderConfig {
	return ProviderConfig{
		Command:         piACPCommand,
		DisplayName:     displayName,
		Harness:         ProviderHarnessPiACP,
		RuntimeProvider: runtimeProvider,
		CredentialSlots: []ProviderCredentialSlot{apiKeyCredentialSlot(credentialEnv)},
		Models: ProviderModelsConfig{
			Default: model,
			Curated: []ProviderModelConfig{{ID: model, DisplayName: modelName}},
		},
	}
}

// canonicalRetiredProviderName resolves a retired provider id or alias.
func canonicalRetiredProviderName(lower string) (string, bool) {
	if _, ok := retiredBuiltinProviders[lower]; ok {
		return lower, true
	}
	canonical, ok := retiredBuiltinProviderAliases[lower]
	return canonical, ok
}

// retiredProviderBase returns the former builtin definition a reference still needs. A
// [providers.<id>] entry that declares its own command (and harness, for pi_acp providers) is
// already the replacement and resolves as a plain custom provider.
func retiredProviderBase(name string, override ProviderConfig, overridden bool) (ProviderConfig, bool) {
	retired, ok := retiredBuiltinProviders[name]
	if !ok {
		return ProviderConfig{}, false
	}
	if overridden && strings.TrimSpace(override.Command) != "" &&
		(retired.Harness != ProviderHarnessPiACP || override.Harness != "") {
		return ProviderConfig{}, false
	}
	warnRetiredBuiltinProvider(name, retired)
	return cloneProvider(retired), true
}

func warnRetiredBuiltinProvider(name string, retired ProviderConfig) {
	if _, loaded := retiredBuiltinProviderWarnings.LoadOrStore(name, struct{}{}); loaded {
		return
	}
	slog.Warn(
		"provider.retired_builtin",
		"provider", name,
		"removed_in", retiredBuiltinProviderRemovalRelease,
		"replacement", retiredBuiltinProviderReplacement(name, retired),
	)
}

// retiredBuiltinProviderReplacement renders the config.toml entry that keeps a retired builtin
// provider working as a custom provider.
func retiredBuiltinProviderReplacement(name string, retired ProviderConfig) string {
	var b bytes.Buffer
	document := map[string]map[string]ProviderConfig{providersConfigKey: {name: retired}}
	encoder := burnttoml.NewEncoder(&b)
	encoder.Indent = ""
	if err := encoder.Encode(document); err != nil {
		return fmt.Sprintf("[%s.%s]\ncommand = %q", providersConfigKey, name, retired.Command)
	}
	return strings.TrimSpace(strings.TrimPrefix(b.String(), "["+providersConfigKey+"]\n"))
}
