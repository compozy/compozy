package contract

type SettingsProviderModelsPayload struct {
	Default   string                                  `json:"default,omitempty"`
	Curated   []SettingsProviderModelPayload          `json:"curated,omitempty"`
	Discovery *SettingsProviderModelsDiscoveryPayload `json:"discovery,omitempty"`
	Reasoning *SettingsProviderReasoningPayload       `json:"reasoning,omitempty"`
}

type SettingsProviderModelsDiscoveryPayload struct {
	Enabled  *bool  `json:"enabled,omitempty"`
	Command  string `json:"command,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	Timeout  string `json:"timeout,omitempty"`
}

type SettingsProviderReasoningPayload struct {
	Apply string `json:"apply"`
}

type SettingsProviderModelPayload struct {
	ID                       string            `json:"id"`
	DisplayName              string            `json:"display_name,omitempty"`
	ContextWindow            *int64            `json:"context_window,omitempty"`
	MaxInputTokens           *int64            `json:"max_input_tokens,omitempty"`
	MaxOutputTokens          *int64            `json:"max_output_tokens,omitempty"`
	SupportsTools            *bool             `json:"supports_tools,omitempty"`
	SupportsReasoning        *bool             `json:"supports_reasoning,omitempty"`
	ReasoningEfforts         []ReasoningEffort `json:"reasoning_efforts,omitempty"`
	DefaultReasoningEffort   ReasoningEffort   `json:"default_reasoning_effort,omitempty"`
	CostInputPerMillion      *float64          `json:"cost_input_per_million,omitempty"`
	CostOutputPerMillion     *float64          `json:"cost_output_per_million,omitempty"`
	CostCacheReadPerMillion  *float64          `json:"cost_cache_read_per_million,omitempty"`
	CostCacheWritePerMillion *float64          `json:"cost_cache_write_per_million,omitempty"`
	CostReasoningPerMillion  *float64          `json:"cost_reasoning_per_million,omitempty"`
	Deprecated               *bool             `json:"deprecated,omitempty"`
	Hidden                   *bool             `json:"hidden,omitempty"`
	Featured                 *bool             `json:"featured,omitempty"`
	ReleaseDate              string            `json:"release_date,omitempty"`
}
