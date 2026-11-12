package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
)

// timeShutdownPhase publishes and logs one named shutdown phase with its duration.
func (d *Daemon) timeShutdownPhase(ctx context.Context, phase string, run func() error) error {
	return d.timeLifecycleStep(ctx, d.runtimeLogger(), contract.DaemonReadinessStopping, phase, run)
}

// daemonShutdownOperation is one daemon-owned teardown attempt. Callers share
// its result while retaining independent wait contexts.
type daemonShutdownOperation struct {
	done   chan struct{}
	result error
	// Published with done; discovery withdrawal also requires current singleton ownership.
	sessionShutdownIncomplete bool
	withdrawDiscovery         bool
}

// beginShutdownOperation carries terminal withdrawal intent across shared teardown attempts.
func (d *Daemon) beginShutdownOperation(
	ctx context.Context,
	withdrawDiscovery bool,
) (*daemonShutdownOperation, bool, *shutdownTargets, time.Duration, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.shutdown != nil {
		withdrawDiscovery = withdrawDiscovery || d.shutdown.withdrawDiscovery
		d.shutdown.withdrawDiscovery = withdrawDiscovery
		select {
		case <-d.shutdown.done:
			if d.shutdown.result == nil || ctx.Err() != nil {
				return d.shutdown, false, nil, 0, nil
			}
			// A failed attempt retains the published targets. A later caller
			// with a live context owns the retry.
		default:
			return d.shutdown, false, nil, 0, nil
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, false, nil, 0, fmt.Errorf("daemon: shutdown context ended: %w", err)
	}
	if d.booting {
		return nil, false, nil, 0, errDaemonBootInProgress
	}

	operation := &daemonShutdownOperation{done: make(chan struct{}), withdrawDiscovery: withdrawDiscovery}
	targets := d.snapshotShutdownTargetsLocked()
	timeout := d.gracefulShutdownTimeoutLocked()
	d.shutdown = operation
	return operation, true, &targets, timeout, nil
}

func (d *Daemon) runShutdownOperation(
	parent context.Context,
	timeout time.Duration,
	operation *daemonShutdownOperation,
	targets *shutdownTargets,
) {
	// The teardown phases share this deadline; the phases after them own detached budgets that the
	// complete budget reserves.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(parent), shutdownTeardownBudget(timeout))
	defer cancel()

	started := time.Now()
	// Before anything drains: from here on no provider is spawned and no automatic recovery starts.
	d.beginSessionShutdown(targets.sessions)
	drainErr := d.timeShutdownPhase(shutdownCtx, "close_admission", func() error {
		return d.Drain(context.WithoutCancel(shutdownCtx))
	})
	checkpointErr := d.timeShutdownPhase(shutdownCtx, "drain_and_checkpoint_sessions", func() error {
		return d.prepareShutdownSessions(shutdownCtx, targets.sessions)
	})
	shutdownErr := d.shutdownDetached(shutdownCtx, operation, targets)
	operation.sessionShutdownIncomplete = targets.sessionShutdownIncomplete
	// The logger may already be closed; the default logger still records the total.
	slog.Default().Info("daemon: shutdown complete",
		"duration_ms", time.Since(started).Milliseconds(),
		"budget_ms", timeout.Milliseconds(),
		"session_shutdown_incomplete", targets.sessionShutdownIncomplete,
	)
	d.completeShutdownOperation(operation, errors.Join(drainErr, checkpointErr, shutdownErr))
}

// shutdownIsTerminal reports that a process-terminal caller owns or joined this operation.
func (d *Daemon) shutdownIsTerminal(operation *daemonShutdownOperation) bool {
	if operation == nil {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return operation.withdrawDiscovery
}

// completeShutdownOperation withdraws stale discovery only while the singleton remains owned.
func (d *Daemon) completeShutdownOperation(operation *daemonShutdownOperation, result error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.shutdown != operation {
		return
	}
	if operation.withdrawDiscovery && operation.sessionShutdownIncomplete && d.lock != nil && !d.lock.released.Load() {
		result = errors.Join(result, RemoveInfo(d.homePaths.DaemonInfo))
	}
	operation.result = result
	if result == nil {
		d.resetRuntimeStateLocked()
	}
	close(operation.done)
}

func waitForShutdownOperation(ctx context.Context, operation *daemonShutdownOperation) error {
	select {
	case <-operation.done:
		return operation.result
	default:
	}

	select {
	case <-operation.done:
		return operation.result
	case <-ctx.Done():
		select {
		case <-operation.done:
			return operation.result
		default:
			return fmt.Errorf("daemon: wait for shutdown: %w", ctx.Err())
		}
	}
}
