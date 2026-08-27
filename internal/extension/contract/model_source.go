package contract

import (
	"time"

	apicontract "github.com/compozy/compozy/internal/api/contract"
)

// ModelSourceListParams is sent by Compozy to extension model sources.
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
	ConfigOptions          []ModelSourceOptionDescriptor        `json:"config_options,omitempty"`
	TransportBindings      []ModelSourceTransportBinding        `json:"transport_bindings,omitempty"`
	Cost                   *apicontract.ModelCatalogCostPayload `json:"cost,omitempty"`
	Deprecated             *bool                                `json:"deprecated,omitempty"`
	Hidden                 *bool                                `json:"hidden,omitempty"`
	Featured               *bool                                `json:"featured,omitempty"`
	ReleaseDate            *string                              `json:"release_date,omitempty"`
	LastError              string                               `json:"last_error,omitempty"`
}

// ModelSourceOptionKind identifies the value shape of an extension-provided model option.
type ModelSourceOptionKind string

const (
	ModelSourceOptionKindSelect  ModelSourceOptionKind = "select"
	ModelSourceOptionKindBoolean ModelSourceOptionKind = "boolean"
)

// ModelSourceOptionDescriptor describes one option supported by a logical model.
type ModelSourceOptionDescriptor struct {
	ID             string                   `json:"id"`
	Label          string                   `json:"label,omitempty"`
	Description    string                   `json:"description,omitempty"`
	Category       string                   `json:"category,omitempty"`
	Kind           ModelSourceOptionKind    `json:"kind"`
	CurrentValueID string                   `json:"current_value_id,omitempty"`
	CurrentBool    *bool                    `json:"current_bool,omitempty"`
	Values         []ModelSourceOptionValue `json:"values,omitempty"`
}

// ModelSourceOptionValue is one grouped value of a select option.
type ModelSourceOptionValue struct {
	ValueID     string `json:"value_id"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
	GroupID     string `json:"group_id,omitempty"`
	GroupLabel  string `json:"group_label,omitempty"`
	Order       int    `json:"order,omitempty"`
}

// ModelSourceOptionSelection selects one typed option value for a transport binding.
type ModelSourceOptionSelection struct {
	ID        string `json:"id"`
	ValueID   string `json:"value_id,omitempty"`
	BoolValue *bool  `json:"bool_value,omitempty"`
}

// ModelSourceTransportBinding maps a logical model configuration to a provider transport identifier.
type ModelSourceTransportBinding struct {
	TransportModelID string                       `json:"transport_model_id"`
	Label            string                       `json:"label,omitempty"`
	ReasoningEffort  *apicontract.ReasoningEffort `json:"reasoning_effort,omitempty"`
	Fast             *bool                        `json:"fast,omitempty"`
	Thinking         *bool                        `json:"thinking,omitempty"`
	OptionSelections []ModelSourceOptionSelection `json:"option_selections,omitempty"`
}
