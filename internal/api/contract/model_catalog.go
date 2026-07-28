package contract

// ProviderModelListResponse is the native provider model catalog list payload.
type ProviderModelListResponse struct {
	Models []ProviderModelPayload `json:"models"`
}

// ProviderModelRefreshRequest captures one provider model catalog refresh request.
type ProviderModelRefreshRequest struct {
	SourceID  string `json:"source_id,omitempty"`
	Force     bool   `json:"force,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// ProviderModelRefreshResponse reports provider model catalog refresh source status.
type ProviderModelRefreshResponse struct {
	Sources []ModelCatalogSourceStatusPayload `json:"sources"`
	Error   string                            `json:"error,omitempty"`
}

// ProviderModelCurationRequest captures one model-only provider config mutation.
type ProviderModelCurationRequest struct {
	ModelID                string           `json:"model_id"`
	Hidden                 *bool            `json:"hidden,omitempty"`
	Featured               *bool            `json:"featured,omitempty"`
	Deprecated             *bool            `json:"deprecated,omitempty"`
	DefaultReasoningEffort *ReasoningEffort `json:"default_effort,omitempty"`
}

// ProviderModelCurationResponse reports the effective model and live config-apply result.
type ProviderModelCurationResponse struct {
	Model ProviderModelPayload  `json:"model"`
	Apply SettingsApplyResponse `json:"apply"`
}

// ProviderModelStatusResponse reports provider model catalog source status.
type ProviderModelStatusResponse struct {
	Sources []ModelCatalogSourceStatusPayload `json:"sources"`
}

// ProviderModelPayload is one merged provider model catalog projection.
type ProviderModelPayload struct {
	ProviderID             string                         `json:"provider_id"`
	ModelID                string                         `json:"model_id"`
	DisplayName            string                         `json:"display_name,omitempty"`
	Sources                []ModelCatalogSourceRefPayload `json:"sources"`
	Available              *bool                          `json:"available"`
	AvailabilityState      string                         `json:"availability_state"`
	Stale                  bool                           `json:"stale"`
	RefreshedAt            string                         `json:"refreshed_at,omitempty"`
	ContextWindow          *int64                         `json:"context_window,omitempty"`
	MaxInputTokens         *int64                         `json:"max_input_tokens,omitempty"`
	MaxOutputTokens        *int64                         `json:"max_output_tokens,omitempty"`
	SupportsTools          *bool                          `json:"supports_tools,omitempty"`
	SupportsReasoning      *bool                          `json:"supports_reasoning,omitempty"`
	ReasoningEfforts       []ReasoningEffort              `json:"reasoning_efforts,omitempty"`
	DefaultReasoningEffort *ReasoningEffort               `json:"default_reasoning_effort,omitempty"`
	Cost                   *ModelCatalogCostPayload       `json:"cost,omitempty"`
	Curated                bool                           `json:"curated"`
	Deprecated             bool                           `json:"deprecated"`
	Hidden                 bool                           `json:"hidden"`
	Featured               bool                           `json:"featured"`
	ReleaseDate            string                         `json:"release_date,omitempty"`
	ReasoningSource        ReasoningSource                `json:"reasoning_source,omitempty"`
	LastError              string                         `json:"last_error,omitempty"`
}

// ModelCatalogSourceRefPayload identifies one source used by a merged model.
type ModelCatalogSourceRefPayload struct {
	SourceID    string `json:"source_id"`
	SourceKind  string `json:"source_kind"`
	Priority    int    `json:"priority"`
	RefreshedAt string `json:"refreshed_at,omitempty"`
	Stale       bool   `json:"stale"`
	LastError   string `json:"last_error,omitempty"`
}

// ModelCatalogSourceStatusPayload reports provider-scoped catalog source health.
type ModelCatalogSourceStatusPayload struct {
	SourceID     string `json:"source_id"`
	SourceKind   string `json:"source_kind"`
	ProviderID   string `json:"provider_id"`
	Priority     int    `json:"priority"`
	LastRefresh  string `json:"last_refresh,omitempty"`
	NextRefresh  string `json:"next_refresh,omitempty"`
	LastSuccess  string `json:"last_success,omitempty"`
	LastError    string `json:"last_error,omitempty"`
	RefreshState string `json:"refresh_state"`
	RowCount     int    `json:"row_count"`
	Stale        bool   `json:"stale"`
}

// ModelCatalogCostPayload reports normalized model price hints.
type ModelCatalogCostPayload struct {
	InputPerMillion      *float64 `json:"input_per_million,omitempty"`
	OutputPerMillion     *float64 `json:"output_per_million,omitempty"`
	CacheReadPerMillion  *float64 `json:"cache_read_per_million,omitempty"`
	CacheWritePerMillion *float64 `json:"cache_write_per_million,omitempty"`
	ReasoningPerMillion  *float64 `json:"reasoning_per_million,omitempty"`
}
