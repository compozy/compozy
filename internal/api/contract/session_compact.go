package contract

// SessionCompactRequest requests one advertised native compaction turn.
type SessionCompactRequest struct{}

// SessionCompactResponse is the experimental compaction acceptance receipt.
type SessionCompactResponse struct {
	SessionID string `json:"session_id"`
	PromptID  string `json:"prompt_id"`
	Command   string `json:"command"`
	Status    string `json:"status"`
}
