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
}

func (d *Daemon) beginShutdownOperation(
	ctx context.Context,
) (*daemonShutdownOperation, bool, *shutdownTargets, time.Duration, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.shutdown != nil {
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

	operation := &daemonShutdownOperation{done: make(chan struct{})}
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
	shutdownErr := d.shutdownDetached(shutdownCtx, targets)
	d.completeShutdownOperation(operation, errors.Join(drainErr, shutdownErr))
}

func (d *Daemon) completeShutdownOperation(operation *daemonShutdownOperation, result error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.shutdown != operation {
		return
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
