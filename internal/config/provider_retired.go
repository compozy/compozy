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

var retiredProviderWarnings = struct {
	mu     sync.Mutex
	logger *slog.Logger
	seen   map[string]struct{}
}{seen: make(map[string]struct{})}

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
	return cloneProvider(retired), true
}

// RouteRetiredProviderWarnings sends later retired-provider warnings to logger and re-arms the
// once-per-ID dedup, so the daemon log records the retired references it resolves after boot.
func RouteRetiredProviderWarnings(logger *slog.Logger) {
	retiredProviderWarnings.mu.Lock()
	defer retiredProviderWarnings.mu.Unlock()
	retiredProviderWarnings.logger = logger
	clear(retiredProviderWarnings.seen)
}

// WarnRetiredProviderReferences resolves the configured default and declared providers so every
// retired reference in this config reports its deprecation warning.
func (c *Config) WarnRetiredProviderReferences() {
	if c == nil {
		return
	}
	names := []string{c.Defaults.Provider}
	for name := range c.Providers {
		names = append(names, name)
	}
	for _, name := range names {
		if strings.TrimSpace(name) == "" {
			continue
		}
		// Validation owns resolution errors; this pass only emits retired-provider warnings.
		if _, err := c.ResolveProvider(name); err != nil {
			continue
		}
	}
}

// warnRetiredBuiltinProvider reports the former definition only. Logging the effective provider
// would echo operator-authored values, such as commands with inline credentials, so an existing
// override is merged by instruction instead.
func warnRetiredBuiltinProvider(name string, override ProviderConfig, overridden bool) {
	retiredProviderWarnings.mu.Lock()
	if _, seen := retiredProviderWarnings.seen[name]; seen {
		retiredProviderWarnings.mu.Unlock()
		return
	}
	retiredProviderWarnings.seen[name] = struct{}{}
	logger := retiredProviderWarnings.logger
	retiredProviderWarnings.mu.Unlock()
	if logger == nil {
		logger = slog.Default()
	}
	retired := retiredProviderAdditions(retiredBuiltinProviders[name], override, overridden)
	table := fmt.Sprintf("[%s.%s]", providersConfigKey, name)
	action := "add replacement to config.toml"
	if overridden {
		action = fmt.Sprintf(
			"merge replacement into the existing %s tables in config.toml: "+
				"add each missing key or table and keep every value already set",
			table,
		)
	}
	if len(retired.CredentialSlots) > 0 {
		// Log redaction masks secret_ref lines, so the replacement omits them and this action
		// spells out each value without the redactable joined form.
		refs := make([]string, 0, len(retired.CredentialSlots))
		for _, slot := range retired.CredentialSlots {
			scheme, ref, _ := strings.Cut(slot.SecretRef, ":")
			refs = append(refs, fmt.Sprintf("%s uses scheme %q and name %q", slot.Name, scheme, ref))
		}
		action += ", then add a secret_ref line to each credential_slots table it adds, set to scheme, a colon, and name (" +
			strings.Join(
				refs,
				"; ",
			) + ")"
	}
	logger.Warn(
		"provider.retired_builtin",
		"provider", name,
		"removed_in", retiredBuiltinProviderRemovalRelease,
		"action", action,
		"replacement", withoutSecretRefLines(retiredBuiltinProviderReplacement(name, retired)),
	)
}

// retiredProviderAdditions drops the former fields an existing override's authentication choice
// owns, so adding the missing keys reproduces the effective provider instead of conflicting with it.
func retiredProviderAdditions(retired ProviderConfig, override ProviderConfig, overridden bool) ProviderConfig {
	additions := cloneProvider(retired)
	if !overridden {
		return additions
	}
	applyProviderAuthModeOverride(&additions, ProviderConfig{AuthMode: override.AuthMode})
	additions.AuthMode = ""
	if len(override.CredentialSlots) > 0 {
		additions.CredentialSlots = nil
	}
	return additions
}

func withoutSecretRefLines(replacement string) string {
	lines := strings.Split(replacement, "\n")
	kept := lines[:0]
	for _, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "secret_ref =") {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
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
