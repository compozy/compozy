package loop

import "time"

// SessionWork is durable loop evidence for a session's ephemeral supervision view.
type SessionWork struct {
	RunID        RunID
	Since        time.Time
	ReconciledAt time.Time
	Waits        []NodeWait
}
