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
	// shutdownWatchdogMargin is the slack between the last phase budget and the forced exit. It covers
	// the transitions between phases, so a slow shutdown that respects every budget is never killed.
	shutdownWatchdogMargin   = 2 * time.Second
	shutdownWatchdogExitCode = 1
	// shutdownReservedBudget is what the phases that own a detached budget may spend after teardown.
	shutdownReservedBudget = shutdownSessionDependenciesBudget + shutdownPersistBudget
	// shutdownMinTeardownBudget keeps a teardown deadline when the complete budget is below the reserve.
	shutdownMinTeardownBudget = time.Second
	// shutdownPreludeBudget bounds the serial steps that run before workers and sessions stop.
	shutdownPreludeBudget = 2 * time.Second
	// shutdownPreludeShare is the largest fraction of the remaining teardown the prelude may spend.
	shutdownPreludeShare = 4
	// restartCheckpointBudget is the part of the teardown the restart drain leaves for capturing the
	// restart checkpoints of the turns it did not outwait.
	restartCheckpointBudget = 2 * time.Second
)

// shutdownTeardownBudget is the part of the complete shutdown budget the drain, worker and session
// phases may spend. The detached budgets of the later phases are reserved first, so the phase
// budgets never add up to more than the complete budget.
func shutdownTeardownBudget(total time.Duration) time.Duration {
	return max(total-shutdownReservedBudget, shutdownMinTeardownBudget)
}

// shutdownWatchdogBudget is when a process-terminal shutdown is forced to exit. It is always
// shutdownWatchdogMargin past the sum of every phase budget, which is the complete budget unless
// that budget is smaller than the reserve.
func shutdownWatchdogBudget(total time.Duration) time.Duration {
	return shutdownTeardownBudget(total) + shutdownReservedBudget + shutdownWatchdogMargin
}

// remainingShutdownBudget reports the time left before ctx expires, or fallback without a deadline.
func remainingShutdownBudget(ctx context.Context, fallback time.Duration) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return fallback
	}
	return max(time.Until(deadline), 0)
}

// shutdownPreludeWait is the allocation of the serial steps before workers and sessions stop: a
// fixed cap that is also a bounded share of the remaining teardown, so they cannot starve it.
func shutdownPreludeWait(ctx context.Context) time.Duration {
	remaining := remainingShutdownBudget(ctx, shutdownPreludeBudget*shutdownPreludeShare)
	return min(shutdownPreludeBudget, remaining/shutdownPreludeShare)
}

// restartDrainBudget bounds the restart drain by what the teardown has left after the checkpoint
// reserve. It never exceeds the capped grace and is zero when only the reserve is left.
func restartDrainBudget(ctx context.Context, grace time.Duration) time.Duration {
	wait := restartDrainWait(grace)
	return max(min(wait, remainingShutdownBudget(ctx, wait+restartCheckpointBudget)-restartCheckpointBudget), 0)
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
