package daemon

import (
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/store/globaldb"
)

// restartProgressWriteInterval bounds restart-journal writes while a long migration advances.
const restartProgressWriteInterval = time.Second

// restartProgressRecorder mirrors boot migration progress into the restart journal so restart
// observers can tell a migrating replacement from a stuck one before the daemon serves status.
type restartProgressRecorder struct {
	mu          sync.Mutex
	store       *restartStore
	operationID string
	now         func() time.Time
	lastWrite   time.Time
	disabled    bool
}

// migrationProgressReporter returns nil unless this daemon boots as a coordinated-restart replacement.
func (d *Daemon) migrationProgressReporter() globaldb.MigrationProgressFunc {
	operationID := strings.TrimSpace(restartOperationIDFromEnv(d.getenv))
	if operationID == "" {
		return nil
	}
	now := d.now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	recorder := &restartProgressRecorder{
		store:       newRestartStore(d.homePaths, now),
		operationID: operationID,
		now:         now,
	}
	return recorder.record
}

// record never fails the migration: the journal is an observation channel, not migration state.
func (r *restartProgressRecorder) record(progress globaldb.MigrationProgress) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.disabled {
		return
	}
	now := r.now()
	finished := progress.Completed >= progress.Total
	if !r.lastWrite.IsZero() && !finished && now.Sub(r.lastWrite) < restartProgressWriteInterval {
		return
	}
	recorded, err := r.store.RecordProgress(r.operationID, RestartProgress{
		Stage:     progress.Stage,
		Completed: progress.Completed,
		Total:     progress.Total,
	})
	if err != nil {
		slog.Warn(
			"daemon: record restart migration progress",
			"operation_id", r.operationID,
			"stage", progress.Stage,
			"error", err,
		)
		return
	}
	if !recorded {
		// The operation is terminal or superseded; nobody is observing this replacement anymore.
		r.disabled = true
		return
	}
	r.lastWrite = now
}
