package cli

import (
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
)

// defaultRestartMigrationIdleTimeout bounds how long a migrating replacement may report no progress.
// Progress is written at least once per acknowledged migration batch, so a healthy upgrade advances
// far more often; there is deliberately no wall-clock cap while progress keeps advancing.
const defaultRestartMigrationIdleTimeout = 5 * time.Minute

// restartProgressWatch tracks replacement migration progress across restart polling windows.
type restartProgressWatch struct {
	idleTimeout time.Duration
	now         func() time.Time
	onProgress  func(contract.RestartProgress)
	last        *contract.RestartProgress
	lastAdvance time.Time
}

func newRestartProgressWatch(deps commandDeps, onProgress func(contract.RestartProgress)) *restartProgressWatch {
	now := deps.now
	if now == nil {
		now = time.Now
	}
	idle := deps.restartMigrationIdleTimeout
	if idle <= 0 {
		idle = defaultRestartMigrationIdleTimeout
	}
	return &restartProgressWatch{idleTimeout: idle, now: now, onProgress: onProgress}
}

// observe reports advancing progress and fails only a migration that stopped advancing before completion.
// Measuring idleness on the observer's clock keeps a stale journal timestamp from failing a live upgrade.
func (w *restartProgressWatch) observe(status SettingsRestartStatusRecord) error {
	if w == nil || status.Progress == nil {
		return nil
	}
	progress := *status.Progress
	now := w.now()
	if w.last == nil || progress.Stage != w.last.Stage || progress.Completed != w.last.Completed ||
		progress.Total != w.last.Total {
		w.last = &progress
		w.lastAdvance = now
		if w.onProgress != nil {
			w.onProgress(progress)
		}
		return nil
	}
	if progress.Completed >= progress.Total || now.Sub(w.lastAdvance) < w.idleTimeout {
		return nil
	}
	return fmt.Errorf(
		"cli: daemon data upgrade %q stopped progressing at %d of %d rows for %s",
		progress.Stage,
		progress.Completed,
		progress.Total,
		w.idleTimeout,
	)
}

// restartProgressPercent converts migration progress into a bounded whole percentage.
func restartProgressPercent(progress contract.RestartProgress) int {
	if progress.Total <= 0 || progress.Completed >= progress.Total {
		return 100
	}
	if progress.Completed <= 0 {
		return 0
	}
	return int(progress.Completed * 100 / progress.Total)
}
