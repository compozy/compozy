package kinds

// Usage tracks token consumption in public event payloads.
type Usage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
	TotalTokens  int `json:"total_tokens,omitempty"`
	CacheReads   int `json:"cache_reads,omitempty"`
	CacheWrites  int `json:"cache_writes,omitempty"`
}

// Add accumulates usage from another value into the receiver.
func (u *Usage) Add(other Usage) {
	u.InputTokens += other.InputTokens
	u.OutputTokens += other.OutputTokens
	u.TotalTokens += other.TotalTokens
	u.CacheReads += other.CacheReads
	u.CacheWrites += other.CacheWrites
}

// Total returns the stored total or derives it from inputs and outputs.
func (u Usage) Total() int {
	if u.TotalTokens != 0 {
		return u.TotalTokens
	}
	return u.InputTokens + u.OutputTokens
}

// UsageUpdatedPayload carries per-job usage deltas.
type UsageUpdatedPayload struct {
	Index int   `json:"index"`
	Usage Usage `json:"usage"`
}

// UsageAggregatedPayload carries run-level usage totals.
type UsageAggregatedPayload struct {
	Usage Usage `json:"usage"`
}
