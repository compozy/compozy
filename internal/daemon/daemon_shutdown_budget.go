package daemon

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
)

const (
	// restartDrainCap bounds how long a restart waits for running turns to finish. An interrupted
	// turn is checkpointed and resumed by the next boot, so a longer wait only delays the restart.
	restartDrainCap = 5 * time.Second
	// shutdownWorkersBudget bounds background worker teardown, which runs beside the session stop.
	shutdownWorkersBudget = defaultShutdownTimeout / 2
	// shutdownSessionDependenciesBudget bounds closing what stopping sessions were still using.
	shutdownSessionDependenciesBudget = 3 * time.Second
	// shutdownPersistBudget bounds the final persistence phase, including the checkpointed database
	// close. It is granted even when the earlier phases spent the whole teardown budget.
	shutdownPersistBudget = 4 * time.Second
	// shutdownWatchdogMargin is how far past the complete budget a terminal shutdown may run.
	shutdownWatchdogMargin   = 2 * time.Second
	shutdownWatchdogExitCode = 1
)

// shutdownTeardownBudget is the part of the complete shutdown budget the drain, worker and session
// phases may spend. The remainder is reserved for the phases that own a detached budget, so the
// complete budget still bounds the whole shutdown.
func shutdownTeardownBudget(total time.Duration) time.Duration {
	return max(total-shutdownSessionDependenciesBudget-shutdownPersistBudget, total/2)
}

// restartDrainWait caps the configured grace: zero still disables the wait.
func restartDrainWait(grace time.Duration) time.Duration {
	return min(grace, restartDrainCap)
}

// detachedShutdownBudget grants a phase its own budget, independent of an expired teardown context.
func detachedShutdownBudget(ctx context.Context, budget time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), budget)
}

// timeConcurrentShutdownStep logs the named duration of a step that runs beside another one. It
// leaves the single published readiness step to the enclosing phase.
func (d *Daemon) timeConcurrentShutdownStep(ctx context.Context, step string, run func()) {
	started := time.Now()
	run()
	d.runtimeLogger().InfoContext(ctx, "daemon: lifecycle step",
		"phase", string(contract.DaemonReadinessStopping),
		"step", step,
		"duration_ms", time.Since(started).Milliseconds(),
	)
}

// armShutdownWatchdog guarantees that a process-terminal shutdown ends the process: once budget
// elapses it names the phase still running and exits. The returned function disarms it and must run
// when shutdown returns. Library callers of Shutdown never arm it.
func (d *Daemon) armShutdownWatchdog(budget time.Duration) (disarm func()) {
	d.mu.Lock()
	exit := d.exitProcess
	d.mu.Unlock()
	if exit == nil {
		exit = os.Exit
	}
	timer := time.AfterFunc(budget, func() {
		current := d.readiness.snapshot()
		attrs := []any{
			"budget_ms", budget.Milliseconds(),
			"phase", string(current.phase),
			"step", current.step,
			"exit_code", shutdownWatchdogExitCode,
		}
		const message = "daemon: shutdown watchdog forcing process exit"
		logger := d.runtimeLogger()
		logger.Error(message, attrs...)
		// The daemon logger may already be closed; the default logger still records the exit.
		if fallback := slog.Default(); fallback != logger {
			fallback.Error(message, attrs...)
		}
		exit(shutdownWatchdogExitCode)
	})
	return func() { timer.Stop() }
}
