package kinds

// ProviderCallStartedPayload describes the start of an external provider call.
type ProviderCallStartedPayload struct {
	CallID     string `json:"call_id"`
	Provider   string `json:"provider"`
	Endpoint   string `json:"endpoint,omitempty"`
	Method     string `json:"method,omitempty"`
	PR         string `json:"pr,omitempty"`
	IssueCount int    `json:"issue_count,omitempty"`
}

// ProviderCallCompletedPayload describes a successful provider call.
type ProviderCallCompletedPayload struct {
	CallID       string `json:"call_id"`
	Provider     string `json:"provider"`
	Endpoint     string `json:"endpoint,omitempty"`
	Method       string `json:"method,omitempty"`
	StatusCode   int    `json:"status_code,omitempty"`
	DurationMs   int64  `json:"duration_ms,omitempty"`
	PayloadBytes int    `json:"payload_bytes,omitempty"`
}

// ProviderCallFailedPayload describes a failed provider call.
type ProviderCallFailedPayload struct {
	CallID       string `json:"call_id"`
	Provider     string `json:"provider"`
	Endpoint     string `json:"endpoint,omitempty"`
	Method       string `json:"method,omitempty"`
	StatusCode   int    `json:"status_code,omitempty"`
	DurationMs   int64  `json:"duration_ms,omitempty"`
	PayloadBytes int    `json:"payload_bytes,omitempty"`
	Error        string `json:"error,omitempty"`
}
