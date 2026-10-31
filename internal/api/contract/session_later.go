package contract

import "time"

// SessionLaterPayload is the Later state of a parked session.
type SessionLaterPayload struct {
	ParkedAt time.Time  `json:"parked_at"`
	Reason   string     `json:"reason,omitempty"`
	ReturnAt *time.Time `json:"return_at,omitempty"`
	// PendingMessages counts agent messages queued while the session is parked.
	PendingMessages int `json:"pending_messages"`
}

// MoveSessionToLaterRequest parks a session with an optional one-line reason
// and an optional return time.
type MoveSessionToLaterRequest struct {
	Reason   string     `json:"reason,omitempty"`
	ReturnAt *time.Time `json:"return_at,omitempty"`
}

// ResumeSessionFromLaterRequest returns a parked session to the normal flow.
type ResumeSessionFromLaterRequest struct {
	// WithChildren also takes parked descendants out of Later.
	WithChildren bool `json:"with_children,omitempty"`
}
