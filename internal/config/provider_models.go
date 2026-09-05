package config

import speedpkg "github.com/compozy/compozy/internal/speed"

// ProviderModelsConfig describes provider-scoped model defaults and metadata.
type ProviderModelsConfig struct {
	Default   string                        `toml:"default,omitempty"`
	Curated   []ProviderModelConfig         `toml:"curated,omitempty"`
	Discovery ProviderModelsDiscoveryConfig `toml:"discovery,omitempty"`
	Reasoning ProviderReasoningConfig       `toml:"reasoning,omitempty"`
}

// ProviderModelsDiscoveryConfig describes optional provider model discovery.
type ProviderModelsDiscoveryConfig struct {
	Enabled  *bool  `toml:"enabled,omitempty"`
	Command  string `toml:"command,omitempty"`
	Endpoint string `toml:"endpoint,omitempty"`
	Timeout  string `toml:"timeout,omitempty"`
}

// ProviderModelConfig describes one curated provider model entry.
type ProviderModelConfig struct {
	ID                       string         `toml:"id"`
	DisplayName              string         `toml:"display_name,omitempty"`
	ContextWindow            *int64         `toml:"context_window,omitempty"`
	MaxInputTokens           *int64         `toml:"max_input_tokens,omitempty"`
	MaxOutputTokens          *int64         `toml:"max_output_tokens,omitempty"`
	SupportsTools            *bool          `toml:"supports_tools,omitempty"`
	SupportsReasoning        *bool          `toml:"supports_reasoning,omitempty"`
	ReasoningEfforts         []string       `toml:"reasoning_efforts,omitempty"`
	DefaultReasoningEffort   string         `toml:"default_reasoning_effort,omitempty"`
	DefaultSpeed             speedpkg.Speed `toml:"default_speed,omitempty"`
	CostInputPerMillion      *float64       `toml:"cost_input_per_million,omitempty"`
	CostOutputPerMillion     *float64       `toml:"cost_output_per_million,omitempty"`
	CostCacheReadPerMillion  *float64       `toml:"cost_cache_read_per_million,omitempty"`
	CostCacheWritePerMillion *float64       `toml:"cost_cache_write_per_million,omitempty"`
	CostReasoningPerMillion  *float64       `toml:"cost_reasoning_per_million,omitempty"`
	Deprecated               *bool          `toml:"deprecated,omitempty"`
	Hidden                   *bool          `toml:"hidden,omitempty"`
	Featured                 *bool          `toml:"featured,omitempty"`
	ReleaseDate              string         `toml:"release_date,omitempty"`
}

func cloneProviderModelsConfig(src ProviderModelsConfig) ProviderModelsConfig {
	return ProviderModelsConfig{
		Default:   src.Default,
		Curated:   cloneProviderModelConfigs(src.Curated),
		Discovery: cloneProviderModelsDiscoveryConfig(src.Discovery),
		Reasoning: src.Reasoning,
	}
}

func cloneProviderModelsDiscoveryConfig(
	src ProviderModelsDiscoveryConfig,
) ProviderModelsDiscoveryConfig {
	return ProviderModelsDiscoveryConfig{
		Enabled:  cloneBoolRef(src.Enabled),
		Command:  src.Command,
		Endpoint: src.Endpoint,
		Timeout:  src.Timeout,
	}
}

func cloneProviderModelConfigs(src []ProviderModelConfig) []ProviderModelConfig {
	if src == nil {
		return nil
	}
	cloned := make([]ProviderModelConfig, len(src))
	for idx, model := range src {
		cloned[idx] = cloneProviderModelConfig(model)
	}
	return cloned
}

func cloneProviderModelConfig(src ProviderModelConfig) ProviderModelConfig {
	return ProviderModelConfig{
		ID:                       src.ID,
		DisplayName:              src.DisplayName,
		ContextWindow:            cloneProviderModelPtr(src.ContextWindow),
		MaxInputTokens:           cloneProviderModelPtr(src.MaxInputTokens),
		MaxOutputTokens:          cloneProviderModelPtr(src.MaxOutputTokens),
		SupportsTools:            cloneProviderModelPtr(src.SupportsTools),
		SupportsReasoning:        cloneProviderModelPtr(src.SupportsReasoning),
		ReasoningEfforts:         cloneStrings(src.ReasoningEfforts),
		DefaultReasoningEffort:   src.DefaultReasoningEffort,
		DefaultSpeed:             src.DefaultSpeed,
		CostInputPerMillion:      cloneProviderModelPtr(src.CostInputPerMillion),
		CostOutputPerMillion:     cloneProviderModelPtr(src.CostOutputPerMillion),
		CostCacheReadPerMillion:  cloneProviderModelPtr(src.CostCacheReadPerMillion),
		CostCacheWritePerMillion: cloneProviderModelPtr(src.CostCacheWritePerMillion),
		CostReasoningPerMillion:  cloneProviderModelPtr(src.CostReasoningPerMillion),
		Deprecated:               cloneProviderModelPtr(src.Deprecated),
		Hidden:                   cloneProviderModelPtr(src.Hidden),
		Featured:                 cloneProviderModelPtr(src.Featured),
		ReleaseDate:              src.ReleaseDate,
	}
}

func cloneProviderModelPtr[T any](src *T) *T {
	if src == nil {
		return nil
	}
	value := *src
	return &value
}
