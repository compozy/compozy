package cache

import (
	"sync"
	"time"
)

// SnapshotMetrics holds internal, lock-protected counters for snapshot/restore.
// Fields are intentionally unexported to prevent racy access. Use
// GetSnapshotMetrics (via SnapshotManager) to obtain a read-only view.
type SnapshotMetrics struct {
	mu               sync.RWMutex
	snapshotsTaken   int64
	snapshotFailures int64
	restores         int64
	restoreFailures  int64
	lastDuration     time.Duration
	lastSizeBytes    int64
}

// SnapshotMetricsView is a read-only view of snapshot metrics safe for external
// observation and JSON serialization.
type SnapshotMetricsView struct {
	SnapshotsTaken   int64         `json:"snapshots_taken"`
	SnapshotFailures int64         `json:"snapshot_failures"`
	Restores         int64         `json:"restores"`
	RestoreFailures  int64         `json:"restore_failures"`
	LastDuration     time.Duration `json:"last_duration"`
	LastSizeBytes    int64         `json:"last_size_bytes"`
}

// copy returns a view with values captured under read lock.
func (m *SnapshotMetrics) copy() SnapshotMetricsView {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return SnapshotMetricsView{
		SnapshotsTaken:   m.snapshotsTaken,
		SnapshotFailures: m.snapshotFailures,
		Restores:         m.restores,
		RestoreFailures:  m.restoreFailures,
		LastDuration:     m.lastDuration,
		LastSizeBytes:    m.lastSizeBytes,
	}
}
