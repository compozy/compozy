package kinds

import "time"

// ShutdownRequestedPayload describes a requested shutdown.
type ShutdownRequestedPayload struct {
	Source      string    `json:"source,omitempty"`
	RequestedAt time.Time `json:"requested_at,omitempty"`
	DeadlineAt  time.Time `json:"deadline_at,omitempty"`
}

// ShutdownDrainingPayload describes a draining shutdown.
type ShutdownDrainingPayload struct {
	Source      string    `json:"source,omitempty"`
	RequestedAt time.Time `json:"requested_at,omitempty"`
	DeadlineAt  time.Time `json:"deadline_at,omitempty"`
}

// ShutdownTerminatedPayload describes a terminated shutdown.
type ShutdownTerminatedPayload struct {
	Source      string    `json:"source,omitempty"`
	RequestedAt time.Time `json:"requested_at,omitempty"`
	DeadlineAt  time.Time `json:"deadline_at,omitempty"`
	Forced      bool      `json:"forced,omitempty"`
}
