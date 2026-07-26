package contract

import (
	"time"

	apicontract "github.com/compozy/agh/internal/api/contract"
)

// ModelSourceListParams is sent by AGH to extension model sources.
type ModelSourceListParams struct {
	ProviderID   string `json:"provider_id,omitempty"`
	Refresh      bool   `json:"refresh,omitempty"`
	IncludeStale bool   `json:"include_stale,omitempty"`
}

// ModelSourceListResponse is returned by extension model sources.
type ModelSourceListResponse struct {
	Rows []ModelSourceRow `json:"rows"`
}

// ModelSourceRow is one extension-provided model catalog source row.
type ModelSourceRow struct {
	SourceID               string                               `json:"source_id"`
	ProviderID             string                               `json:"provider_id"`
	ModelID                string                               `json:"model_id"`
	DisplayName            string                               `json:"display_name,omitempty"`
	Priority               int                                  `json:"priority,omitempty"`
	Available              *bool                                `json:"available,omitempty"`
	Stale                  bool                                 `json:"stale,omitempty"`
	RefreshedAt            time.Time                            `json:"refreshed_at"`
	ExpiresAt              time.Time                            `json:"expires_at"`
	ContextWindow          *int64                               `json:"context_window,omitempty"`
	MaxInputTokens         *int64                               `json:"max_input_tokens,omitempty"`
	MaxOutputTokens        *int64                               `json:"max_output_tokens,omitempty"`
	SupportsTools          *bool                                `json:"supports_tools,omitempty"`
	SupportsReasoning      *bool                                `json:"supports_reasoning,omitempty"`
	ReasoningEfforts       []apicontract.ReasoningEffort        `json:"reasoning_efforts,omitempty"`
	DefaultReasoningEffort *apicontract.ReasoningEffort         `json:"default_reasoning_effort,omitempty"`
	Cost                   *apicontract.ModelCatalogCostPayload `json:"cost,omitempty"`
	Deprecated             *bool                                `json:"deprecated,omitempty"`
	Hidden                 *bool                                `json:"hidden,omitempty"`
	Featured               *bool                                `json:"featured,omitempty"`
	ReleaseDate            *string                              `json:"release_date,omitempty"`
	LastError              string                               `json:"last_error,omitempty"`
}
