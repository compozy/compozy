package settings

import (
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
)

func providerModelsConfigFromSettings(models aghconfig.ProviderModelsConfig) aghconfig.ProviderModelsConfig {
	return aghconfig.ProviderModelsConfig{
		Default:   strings.TrimSpace(models.Default),
		Curated:   providerModelConfigsFromSettings(models.Curated),
		Discovery: providerModelsDiscoveryConfigFromSettings(models.Discovery),
		Reasoning: aghconfig.ProviderReasoningConfig{Apply: models.Reasoning.Apply},
	}
}

func providerModelConfigsFromSettings(
	models []aghconfig.ProviderModelConfig,
) []aghconfig.ProviderModelConfig {
	if models == nil {
		return nil
	}
	values := make([]aghconfig.ProviderModelConfig, 0, len(models))
	for _, model := range models {
		id := strings.TrimSpace(model.ID)
		if id == "" {
			continue
		}
		values = append(values, aghconfig.ProviderModelConfig{
			ID:                       id,
			DisplayName:              strings.TrimSpace(model.DisplayName),
			ContextWindow:            cloneInt64Ptr(model.ContextWindow),
			MaxInputTokens:           cloneInt64Ptr(model.MaxInputTokens),
			MaxOutputTokens:          cloneInt64Ptr(model.MaxOutputTokens),
			SupportsTools:            cloneBoolPtr(model.SupportsTools),
			SupportsReasoning:        cloneBoolPtr(model.SupportsReasoning),
			ReasoningEfforts:         cloneStringSlicePreserveNil(model.ReasoningEfforts),
			DefaultReasoningEffort:   strings.TrimSpace(model.DefaultReasoningEffort),
			CostInputPerMillion:      cloneFloat64Ptr(model.CostInputPerMillion),
			CostOutputPerMillion:     cloneFloat64Ptr(model.CostOutputPerMillion),
			CostCacheReadPerMillion:  cloneFloat64Ptr(model.CostCacheReadPerMillion),
			CostCacheWritePerMillion: cloneFloat64Ptr(model.CostCacheWritePerMillion),
			CostReasoningPerMillion:  cloneFloat64Ptr(model.CostReasoningPerMillion),
			Deprecated:               cloneBoolPtr(model.Deprecated),
			Hidden:                   cloneBoolPtr(model.Hidden),
			Featured:                 cloneBoolPtr(model.Featured),
			ReleaseDate:              strings.TrimSpace(model.ReleaseDate),
		})
	}
	return values
}

func providerModelsDiscoveryConfigFromSettings(
	discovery aghconfig.ProviderModelsDiscoveryConfig,
) aghconfig.ProviderModelsDiscoveryConfig {
	return aghconfig.ProviderModelsDiscoveryConfig{
		Enabled:  cloneBoolPtr(discovery.Enabled),
		Command:  strings.TrimSpace(discovery.Command),
		Endpoint: strings.TrimSpace(discovery.Endpoint),
		Timeout:  strings.TrimSpace(discovery.Timeout),
	}
}

func providerModelsSettingsMap(models aghconfig.ProviderModelsConfig) map[string]any {
	values := make(map[string]any)
	if strings.TrimSpace(models.Default) != "" {
		values["default"] = strings.TrimSpace(models.Default)
	}
	if models.Curated != nil {
		values["curated"] = providerModelConfigMaps(models.Curated)
	}
	if discovery := providerModelsDiscoveryMap(models.Discovery); len(discovery) > 0 {
		values["discovery"] = discovery
	}
	if models.Reasoning.Apply != "" {
		values["reasoning"] = map[string]any{"apply": string(models.Reasoning.Apply)}
	}
	return values
}

func providerModelConfigMaps(models []aghconfig.ProviderModelConfig) []map[string]any {
	values := make([]map[string]any, 0, len(models))
	for _, model := range models {
		id := strings.TrimSpace(model.ID)
		if id == "" {
			continue
		}
		entry := make(map[string]any)
		entry["id"] = id
		if strings.TrimSpace(model.DisplayName) != "" {
			entry["display_name"] = strings.TrimSpace(model.DisplayName)
		}
		if model.ContextWindow != nil {
			entry["context_window"] = *model.ContextWindow
		}
		if model.MaxInputTokens != nil {
			entry["max_input_tokens"] = *model.MaxInputTokens
		}
		if model.MaxOutputTokens != nil {
			entry["max_output_tokens"] = *model.MaxOutputTokens
		}
		if model.SupportsTools != nil {
			entry["supports_tools"] = *model.SupportsTools
		}
		if model.SupportsReasoning != nil {
			entry["supports_reasoning"] = *model.SupportsReasoning
		}
		if model.ReasoningEfforts != nil {
			entry["reasoning_efforts"] = cloneStringSlicePreserveNil(model.ReasoningEfforts)
		}
		if strings.TrimSpace(model.DefaultReasoningEffort) != "" {
			entry["default_reasoning_effort"] = strings.TrimSpace(model.DefaultReasoningEffort)
		}
		if model.CostInputPerMillion != nil {
			entry["cost_input_per_million"] = *model.CostInputPerMillion
		}
		if model.CostOutputPerMillion != nil {
			entry["cost_output_per_million"] = *model.CostOutputPerMillion
		}
		if model.CostCacheReadPerMillion != nil {
			entry["cost_cache_read_per_million"] = *model.CostCacheReadPerMillion
		}
		if model.CostCacheWritePerMillion != nil {
			entry["cost_cache_write_per_million"] = *model.CostCacheWritePerMillion
		}
		if model.CostReasoningPerMillion != nil {
			entry["cost_reasoning_per_million"] = *model.CostReasoningPerMillion
		}
		if model.Deprecated != nil {
			entry["deprecated"] = *model.Deprecated
		}
		if model.Hidden != nil {
			entry["hidden"] = *model.Hidden
		}
		if model.Featured != nil {
			entry["featured"] = *model.Featured
		}
		if strings.TrimSpace(model.ReleaseDate) != "" {
			entry["release_date"] = strings.TrimSpace(model.ReleaseDate)
		}
		values = append(values, entry)
	}
	return values
}

func providerModelsDiscoveryMap(discovery aghconfig.ProviderModelsDiscoveryConfig) map[string]any {
	values := make(map[string]any)
	if discovery.Enabled != nil {
		values["enabled"] = *discovery.Enabled
	}
	if strings.TrimSpace(discovery.Command) != "" {
		values["command"] = strings.TrimSpace(discovery.Command)
	}
	if strings.TrimSpace(discovery.Endpoint) != "" {
		values["endpoint"] = strings.TrimSpace(discovery.Endpoint)
	}
	if strings.TrimSpace(discovery.Timeout) != "" {
		values["timeout"] = strings.TrimSpace(discovery.Timeout)
	}
	return values
}
