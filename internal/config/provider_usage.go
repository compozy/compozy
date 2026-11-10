package config

import (
	"maps"
	"slices"
	"strings"
	"time"
)

const ProviderUsageConfigKey = "provider_usage"
const DefaultProviderUsageRefreshInterval = 5 * time.Minute

func DefaultProviderUsageConfig() ProviderUsageConfig {
	return ProviderUsageConfig{Enabled: true, RefreshInterval: DefaultProviderUsageRefreshInterval}
}

func (c ProviderUsageConfig) ProviderEnabled(id string) bool {
	provider, ok := c.Providers[id]
	return c.Enabled && (!ok || provider.Enabled)
}

func (c ProviderUsageConfig) ClaudeKeychainAccess() bool {
	return c.Providers[providerClaudeKey].KeychainAccess
}

func (c ProviderUsageConfig) Clone() ProviderUsageConfig {
	c.Providers = maps.Clone(c.Providers)
	return c
}

func (c ProviderUsageConfig) Validate(providers map[string]ProviderConfig) error {
	if c.RefreshInterval < time.Minute || c.RefreshInterval > time.Hour {
		return ValidationError{Path: "provider_usage.refresh_interval", Message: "must be between 1m and 1h"}
	}
	for _, id := range slices.Sorted(maps.Keys(c.Providers)) {
		path := "provider_usage.providers." + id
		_, builtin := builtinProviders[id]
		_, custom := providers[id]
		if CanonicalProviderName(id) != id || (!builtin && !custom) {
			return ValidationError{Path: path, Message: "unknown canonical provider ID"}
		}
		if id != providerClaudeKey && c.Providers[id].KeychainAccess {
			return ValidationError{Path: path + ".keychain_access", Message: "is only valid for claude"}
		}
	}
	return nil
}

func providerUsageToolPath(path []string) bool {
	return len(path) == 4 && path[0] == ProviderUsageConfigKey && path[1] == "providers" &&
		path[3] == RoleFieldEnabled
}

// ProviderUsageAPIKeySlot is the bound credential slot key-based usage collectors read.
const ProviderUsageAPIKeySlot = "api_key"

// ProviderUsageAuthSupported reports whether a usage collector family reads credentials for mode.
func ProviderUsageAuthSupported(family string, mode ProviderAuthMode) bool {
	switch family {
	case "codex", providerClaudeKey, "cursor", "gemini", "kimi-cli":
		return mode == ProviderAuthModeNativeCLI
	case "openrouter", "vercel-ai-gateway", "zai", "moonshot":
		return mode == ProviderAuthModeBoundSecret
	default:
		return false
	}
}

// ProviderUsageFamily picks the collector family that owns a provider's usage. A provider's own ID
// wins, so Pi-harness built-ins whose runtime_provider names a model vendor keep their collector; a
// configured alias (for example a second Claude account) collects through its runtime_provider's
// collector when that collector supports its auth mode, and a key-based alias also needs an
// `api_key` slot. It returns the own ID when neither has a collector. hasCollector reports which
// families have a collector; nil means every family ProviderUsageAuthSupported knows.
func ProviderUsageFamily(id string, provider ProviderConfig, hasCollector func(string) bool) string {
	if hasCollector == nil {
		hasCollector = func(family string) bool {
			return ProviderUsageAuthSupported(family, ProviderAuthModeNativeCLI) ||
				ProviderUsageAuthSupported(family, ProviderAuthModeBoundSecret)
		}
	}
	if hasCollector(id) {
		return id
	}
	runtime := strings.TrimSpace(provider.RuntimeProvider)
	mode := provider.EffectiveAuthMode()
	if runtime == "" || !hasCollector(runtime) || !ProviderUsageAuthSupported(runtime, mode) {
		return id
	}
	if mode == ProviderAuthModeBoundSecret &&
		!slices.ContainsFunc(provider.EffectiveCredentialSlots(), func(slot ProviderCredentialSlot) bool {
			return slot.Name == ProviderUsageAPIKeySlot
		}) {
		return id
	}
	return runtime
}

// ProviderCostReporting describes the adapter's monetary counter, not token usage.
type ProviderCostReporting string

const (
	// ProviderCostCumulative is the ACP Cost.amount contract and the default for unknown adapters.
	ProviderCostCumulative ProviderCostReporting = "cumulative"
	// ProviderCostPerTurn is reserved for adapters explicitly reporting one turn's cost.
	ProviderCostPerTurn ProviderCostReporting = "per_turn"
)

// ProviderCostReportingFor returns the adapter's declared cost semantics. No
// shipped adapter is known to override ACP's cumulative monetary counter.
// Future exceptions belong here, alongside the adapter survey in the docs.
func ProviderCostReportingFor(provider string) ProviderCostReporting {
	// Both adapters publish session-total dollars. Other builtins either omit
	// monetary usage or do not document an exception to the ACP contract.
	declarations := map[string]ProviderCostReporting{
		providerClaudeKey:   ProviderCostCumulative,
		providerOpencodeKey: ProviderCostCumulative,
	}
	if semantics, ok := declarations[CanonicalProviderName(provider)]; ok {
		return semantics
	}
	return ProviderCostCumulative
}
