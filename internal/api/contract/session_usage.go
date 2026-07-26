package contract

// SessionUsagePayload is the aggregated per-session token-usage summary sourced
// from the daemon's authoritative token-stats aggregate. Pointer metrics remain
// absent when the runtime has no truthful value.
type SessionUsagePayload struct {
	InputTokens  *int64     `json:"input_tokens,omitempty"`
	OutputTokens *int64     `json:"output_tokens,omitempty"`
	TotalTokens  *int64     `json:"total_tokens,omitempty"`
	TotalCost    *float64   `json:"total_cost,omitempty"`
	CostCurrency string     `json:"cost_currency,omitempty"`
	CostStatus   CostStatus `json:"cost_status,omitempty"`
	CostSource   CostSource `json:"cost_source,omitempty"`
	TurnCount    int64      `json:"turn_count"`
}
