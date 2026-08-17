package hooks

import "time"

// SessionAttentionChangedPayload observes one committed attention badge edge.
type SessionAttentionChangedPayload struct {
	PayloadBase
	SessionContext
	From  string    `json:"from"`
	To    string    `json:"to"`
	Class string    `json:"class"`
	At    time.Time `json:"at"`
}

// SessionAttentionObservationPatch is the no-op patch surface for attention observations.
type SessionAttentionObservationPatch struct{}
