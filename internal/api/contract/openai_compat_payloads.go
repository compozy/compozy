package contract

// OpenAIModelListResponse is the OpenAI-compatible model list projection.
type OpenAIModelListResponse struct {
	Object string               `json:"object"`
	Data   []OpenAIModelPayload `json:"data"`
}

// OpenAIModelPayload is one OpenAI-compatible model object with AGH metadata.
type OpenAIModelPayload struct {
	ID      string                `json:"id"`
	Object  string                `json:"object"`
	Created int64                 `json:"created"`
	OwnedBy string                `json:"owned_by"`
	AGH     OpenAIModelAGHPayload `json:"agh"`
}

// OpenAIModelAGHPayload carries AGH-specific model metadata under the `agh` key.
type OpenAIModelAGHPayload struct {
	ProviderID             string                   `json:"provider_id"`
	ModelID                string                   `json:"model_id"`
	DisplayName            string                   `json:"display_name,omitempty"`
	Sources                []string                 `json:"sources"`
	Available              *bool                    `json:"available"`
	AvailabilityState      string                   `json:"availability_state"`
	Stale                  bool                     `json:"stale"`
	RefreshedAt            string                   `json:"refreshed_at,omitempty"`
	ContextWindow          *int64                   `json:"context_window,omitempty"`
	MaxInputTokens         *int64                   `json:"max_input_tokens,omitempty"`
	MaxOutputTokens        *int64                   `json:"max_output_tokens,omitempty"`
	SupportsTools          *bool                    `json:"supports_tools,omitempty"`
	SupportsReasoning      *bool                    `json:"supports_reasoning,omitempty"`
	ReasoningEfforts       []ReasoningEffort        `json:"reasoning_efforts,omitempty"`
	DefaultReasoningEffort *ReasoningEffort         `json:"default_reasoning_effort,omitempty"`
	Cost                   *ModelCatalogCostPayload `json:"cost,omitempty"`
	LastError              string                   `json:"last_error,omitempty"`
}

// OpenAIErrorResponse is the OpenAI-compatible error envelope.
type OpenAIErrorResponse struct {
	Error OpenAIErrorPayload `json:"error"`
}

// OpenAIErrorPayload carries OpenAI-style error details.
type OpenAIErrorPayload struct {
	Message string  `json:"message"`
	Type    string  `json:"type"`
	Param   *string `json:"param"`
	Code    string  `json:"code"`
}
