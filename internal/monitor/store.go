package monitor

import (
	"context"
	"time"
)

// Record is the durable form of one monitor. It carries the definition and
// lifecycle state needed to restore a watch after a daemon restart; terminal
// output is never stored. Version orders writes made without holding the
// service lock, so a stale write can never overwrite a newer one.
type Record struct {
	Monitor      Monitor
	WorkspaceID  string
	ProfileID    string
	Timeout      time.Duration
	OwnerBinding string
	// OwnerCheckFailedAt is when a recovering monitor's owner check first failed transiently; zero
	// otherwise. It anchors the retry horizon across restarts.
	OwnerCheckFailedAt time.Time
	Version            int64
}

// Store persists monitors so they survive a daemon restart.
type Store interface {
	// SaveMonitor upserts the record; a write whose Version is not newer than the stored one is ignored.
	SaveMonitor(ctx context.Context, record Record) error
	// LoadMonitors returns every persisted monitor.
	LoadMonitors(ctx context.Context) ([]Record, error)
	// PruneFinishedMonitors deletes finished monitors that fired before cutoff.
	PruneFinishedMonitors(ctx context.Context, cutoff time.Time) error
}
