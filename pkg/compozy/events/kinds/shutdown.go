package kinds

import "time"

// ShutdownBase carries shared shutdown timestamps and source metadata.
type ShutdownBase struct {
	Source      string    `json:"source,omitempty"`
	RequestedAt time.Time `json:"requested_at,omitzero"`
	DeadlineAt  time.Time `json:"deadline_at,omitzero"`
}

// ShutdownRequestedPayload describes a requested shutdown.
type ShutdownRequestedPayload struct {
	ShutdownBase
}

// ShutdownDrainingPayload describes a draining shutdown.
type ShutdownDrainingPayload struct {
	ShutdownBase
}

// ShutdownTerminatedPayload describes a terminated shutdown.
type ShutdownTerminatedPayload struct {
	ShutdownBase
	Forced bool `json:"forced,omitempty"`
}
