package contract

type SettingsProviderModelsPayload struct {
	Default   string                                  `json:"default,omitempty"`
	Curated   []SettingsProviderModelPayload          `json:"curated,omitempty"`
	Discovery *SettingsProviderModelsDiscoveryPayload `json:"discovery,omitzero"`
	Reasoning *SettingsProviderReasoningPayload       `json:"reasoning,omitzero"`
}

type SettingsProviderModelsDiscoveryPayload struct {
	Enabled  *bool  `json:"enabled,omitzero"`
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
	ContextWindow            *int64            `json:"context_window,omitzero"`
	MaxInputTokens           *int64            `json:"max_input_tokens,omitzero"`
	MaxOutputTokens          *int64            `json:"max_output_tokens,omitzero"`
	SupportsTools            *bool             `json:"supports_tools,omitzero"`
	SupportsReasoning        *bool             `json:"supports_reasoning,omitzero"`
	ReasoningEfforts         []ReasoningEffort `json:"reasoning_efforts,omitempty"`
	DefaultReasoningEffort   ReasoningEffort   `json:"default_reasoning_effort,omitempty"`
	DefaultSpeed             Speed             `json:"default_speed,omitempty"`
	CostInputPerMillion      *float64          `json:"cost_input_per_million,omitzero"`
	CostOutputPerMillion     *float64          `json:"cost_output_per_million,omitzero"`
	CostCacheReadPerMillion  *float64          `json:"cost_cache_read_per_million,omitzero"`
	CostCacheWritePerMillion *float64          `json:"cost_cache_write_per_million,omitzero"`
	CostReasoningPerMillion  *float64          `json:"cost_reasoning_per_million,omitzero"`
	Deprecated               *bool             `json:"deprecated,omitzero"`
	Hidden                   *bool             `json:"hidden,omitzero"`
	Featured                 *bool             `json:"featured,omitzero"`
	ReleaseDate              string            `json:"release_date,omitempty"`
}
