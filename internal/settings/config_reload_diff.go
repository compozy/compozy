package settings

import (
	"reflect"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/config/lifecycle"
)

func classifyReloadLifecycle(current *aghconfig.Config, desired *aghconfig.Config) lifecycle.Lifecycle {
	changed := reloadChangedPaths(current, desired)
	if len(changed) == 0 {
		return lifecycle.RestartRequired
	}
	configLifecycle, _, err := lifecycle.ClassifyPaths(changed)
	if err != nil {
		return lifecycle.RestartRequired
	}
	return configLifecycle
}

func reloadChangedPaths(current *aghconfig.Config, desired *aghconfig.Config) []string {
	var changed []string
	changed = append(changed, diffGeneralSettings(current, generalSettingsFromConfig(desired))...)
	changed = append(changed, diffSkillsSettings(current.Skills, desired.Skills)...)
	changed = append(changed, diffRolesSettings(&current.Roles, &desired.Roles)...)
	changed = append(changed, diffMemorySettings(&current.Memory, &desired.Memory)...)
	changed = append(changed, diffAutomationSettings(current, automationSettingsFromConfig(desired))...)
	if current.Automation.Suggestions.PendingCap != desired.Automation.Suggestions.PendingCap {
		changed = append(changed, "automation.suggestions.pending_cap")
	}
	changed = append(changed, diffNetworkSettings(current.Network, desired.Network)...)
	changed = append(changed, diffWindowManagerSettings(current.WindowManager, desired.WindowManager)...)
	changed = append(changed, diffObservabilitySettings(current.Observability, desired.Observability)...)
	changed = append(changed, diffExtensionsSettings(current.Extensions, desired.Extensions)...)
	changed = append(changed, diffMarketplaceCatalog(current.Marketplace.Catalog, desired.Marketplace.Catalog)...)
	changed = append(changed, diffProviderSettings(current.Providers, desired.Providers)...)
	if !reflect.DeepEqual(current.MCPServers, desired.MCPServers) {
		changed = append(changed, "mcp-servers.*")
	}
	if !reflect.DeepEqual(current.Sandboxes, desired.Sandboxes) {
		changed = append(changed, "sandboxes.*")
	}
	if !reflect.DeepEqual(current.Hooks.Declarations, desired.Hooks.Declarations) {
		changed = append(changed, "hooks.*")
	}
	return changed
}

func diffMarketplaceCatalog(
	current aghconfig.MarketplaceCatalogConfig,
	desired aghconfig.MarketplaceCatalogConfig,
) []string {
	changed := make([]string, 0, 3)
	if current.BaseURL != desired.BaseURL {
		changed = append(changed, "marketplace.catalog.base_url")
	}
	if current.TTL != desired.TTL {
		changed = append(changed, "marketplace.catalog.ttl")
	}
	if current.Timeout != desired.Timeout {
		changed = append(changed, "marketplace.catalog.timeout")
	}
	return changed
}
