package cache

import (
	"sync"
	"time"
)

// SnapshotMetrics tracks snapshot/restore operations for the embedded Redis
// persistence layer. Callers must copy via GetSnapshotMetrics to avoid races.
type SnapshotMetrics struct {
	mu               sync.RWMutex
	SnapshotsTaken   int64         `json:"snapshots_taken"`
	SnapshotFailures int64         `json:"snapshot_failures"`
	Restores         int64         `json:"restores"`
	RestoreFailures  int64         `json:"restore_failures"`
	LastDuration     time.Duration `json:"last_duration"`
	LastSizeBytes    int64         `json:"last_size_bytes"`
}

// copy returns a shallow copy safe for external observation.
func (m *SnapshotMetrics) copy() SnapshotMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return SnapshotMetrics{
		SnapshotsTaken:   m.SnapshotsTaken,
		SnapshotFailures: m.SnapshotFailures,
		Restores:         m.Restores,
		RestoreFailures:  m.RestoreFailures,
		LastDuration:     m.LastDuration,
		LastSizeBytes:    m.LastSizeBytes,
	}
}
