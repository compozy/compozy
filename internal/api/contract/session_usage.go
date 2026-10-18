package contract

// SessionUsagePayload is the aggregated per-session token-usage summary sourced
// from the daemon's authoritative token-stats aggregate. Pointer metrics remain
// absent when the runtime has no truthful value.
type SessionUsagePayload struct {
	CacheReadTokens  *int64                `json:"cache_read_tokens,omitzero"`
	CacheWriteTokens *int64                `json:"cache_write_tokens,omitzero"`
	Context          SessionContextPayload `json:"context"`
	InputTokens      *int64                `json:"input_tokens,omitzero"`
	OutputTokens     *int64                `json:"output_tokens,omitzero"`
	TotalTokens      *int64                `json:"total_tokens,omitzero"`
	TotalCost        *float64              `json:"total_cost,omitzero"`
	CostCurrency     string                `json:"cost_currency,omitempty"`
	CostStatus       CostStatus            `json:"cost_status,omitempty"`
	CostSource       CostSource            `json:"cost_source,omitempty"`
	TurnCount        int64                 `json:"turn_count"`
}
