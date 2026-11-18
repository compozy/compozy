package daemon

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
)

// readinessSnapshot is the daemon's current lifecycle phase and the step it is running.
type readinessSnapshot struct {
	phase contract.DaemonReadinessPhase
	step  string
	since time.Time
}

// readinessTracker publishes the current boot, recovery and shutdown step for status readers.
// It is observability only: admission and `status` keep their own owners.
type readinessTracker struct {
	current atomic.Pointer[readinessSnapshot]
}

func (t *readinessTracker) set(phase contract.DaemonReadinessPhase, step string, now time.Time) {
	t.current.Store(&readinessSnapshot{phase: phase, step: step, since: now})
}

func (t *readinessTracker) snapshot() readinessSnapshot {
	if current := t.current.Load(); current != nil {
		return *current
	}
	return readinessSnapshot{}
}

// DaemonReadiness reports the lifecycle phase and current step for the status payload.
func (d *Daemon) DaemonReadiness() *contract.DaemonReadinessPayload {
	if d == nil {
		return nil
	}
	current := d.readiness.snapshot()
	if current.phase == "" {
		return nil
	}
	return &contract.DaemonReadinessPayload{Phase: current.phase, Step: current.step, Since: current.since}
}

func (d *Daemon) lifecycleNow() time.Time {
	if d.now != nil {
		return d.now()
	}
	return time.Now().UTC()
}

// timeLifecycleStep publishes the step, runs it and logs its named duration. The log line is the
// before/after evidence for boot and shutdown cost; it is emitted for failed steps too.
func (d *Daemon) timeLifecycleStep(
	ctx context.Context,
	logger *slog.Logger,
	phase contract.DaemonReadinessPhase,
	step string,
	run func() error,
) error {
	d.readiness.set(phase, step, d.lifecycleNow())
	started := time.Now()
	err := run()
	if logger == nil {
		logger = slog.Default()
	}
	attrs := []any{"phase", string(phase), "step", step, "duration_ms", time.Since(started).Milliseconds()}
	if err != nil {
		attrs = append(attrs, "error", err)
	}
	logger.InfoContext(ctx, "daemon: lifecycle step", attrs...)
	return err
}
