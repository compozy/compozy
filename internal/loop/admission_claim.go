package loop

import "time"

// AdmissionClaim is one workspace-scoped watch redelivery tombstone.
type AdmissionClaim struct {
	WorkspaceID      WorkspaceID
	LoopName         string
	SourceKey        string
	EventKey         string
	LoopRunID        RunID
	ClaimedAt        time.Time
	ExpiresAt        time.Time
	SuppressedCount  int
	LastSuppressedAt *time.Time
}
