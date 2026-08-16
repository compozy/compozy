package contract

// SessionPromptCancelResponse reports the idempotent prompt cancellation result.
type SessionPromptCancelResponse struct {
	SessionID string `json:"session_id"`
	Outcome   string `json:"outcome"`
	TurnID    string `json:"turn_id,omitempty"`
}
