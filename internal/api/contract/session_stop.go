package contract

import "github.com/compozy/compozy/internal/session"

// StopSessionRequest selects asynchronous acceptance or a verified stop result.
type StopSessionRequest struct {
	Wait *bool `json:"wait,omitempty"`
}

// SessionStopPayload reports acceptance or the settled termination outcome.
type SessionStopPayload struct {
	SessionID    string            `json:"session_id"`
	Status       string            `json:"status"`
	State        session.State     `json:"state"`
	Verified     bool              `json:"verified"`
	Escalated    bool              `json:"escalated"`
	StopCause    string            `json:"stop_cause,omitempty"`
	Phase        session.StopPhase `json:"phase,omitempty"`
	StoppedAfter string            `json:"stopped_after,omitempty"`
	Attention    string            `json:"attention,omitempty"`
}
