package cache

import (
	"context"

	"github.com/alicebob/miniredis/v2"

	"github.com/compozy/compozy/pkg/config"
)

// SnapshotManager is a lightweight stub to satisfy compilation until the
// full persistence implementation is added in a later task. Methods are
// no-ops and safe to call.
type SnapshotManager struct{}

// NewSnapshotManager creates a stub snapshot manager. The returned instance
// performs no persistence operations.
func NewSnapshotManager(
	_ context.Context,
	_ *miniredis.Miniredis,
	_ config.RedisPersistenceConfig,
) (*SnapshotManager, error) {
	return &SnapshotManager{}, nil
}

// Snapshot is a no-op in the stub implementation.
func (sm *SnapshotManager) Snapshot(_ context.Context) error { return nil }

// Restore is a no-op in the stub implementation.
func (sm *SnapshotManager) Restore(_ context.Context) error { return nil }

// StartPeriodicSnapshots is a no-op in the stub implementation.
func (sm *SnapshotManager) StartPeriodicSnapshots(_ context.Context) {}

// Stop is a no-op in the stub implementation.
func (sm *SnapshotManager) Stop() {}
