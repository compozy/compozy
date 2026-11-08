package daemon

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// daemonShutdownOperation is one daemon-owned teardown attempt. Callers share
// its result while retaining independent wait contexts.
type daemonShutdownOperation struct {
	done   chan struct{}
	result error
	// Published with done; incomplete teardown retains the singleton.
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
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(parent), timeout)
	defer cancel()

	drainErr := d.Drain(context.WithoutCancel(shutdownCtx))
	checkpointErr := d.prepareShutdownSessions(shutdownCtx, targets.sessions)
	shutdownErr := d.shutdownDetached(shutdownCtx, targets)
	operation.sessionShutdownIncomplete = targets.sessionShutdownIncomplete
	d.completeShutdownOperation(operation, errors.Join(drainErr, checkpointErr, shutdownErr))
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
