package contract

import "time"

// TaskRunOperationalSummaryPayload captures aggregated runtime metrics for run detail.
type TaskRunOperationalSummaryPayload struct {
	LastActivityAt time.Time  `json:"last_activity_at"`
	LastEventType  string     `json:"last_event_type,omitempty"`
	ToolCallCount  *int64     `json:"tool_call_count,omitempty"`
	TurnCount      *int64     `json:"turn_count,omitempty"`
	InputTokens    *int64     `json:"input_tokens,omitempty"`
	OutputTokens   *int64     `json:"output_tokens,omitempty"`
	TotalTokens    *int64     `json:"total_tokens,omitempty"`
	TotalCost      *float64   `json:"total_cost,omitempty"`
	CostCurrency   *string    `json:"cost_currency,omitempty"`
	CostStatus     CostStatus `json:"cost_status,omitempty"`
	CostSource     CostSource `json:"cost_source,omitempty"`
}
