package store

import "time"

// ObservabilityRetentionSweepResult reports how many global observability rows
// were deleted by one retention sweep.
type ObservabilityRetentionSweepResult struct {
	CutoffAt               time.Time
	DeletedEventSummaries  int64
	DeletedTokenStats      int64
	DeletedTokenUsageDaily int64
	DeletedPermissionLogs  int64
}
