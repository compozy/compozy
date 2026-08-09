package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"syscall"
	"time"
)

// Run boots the daemon, blocks until signal or context cancellation, then performs graceful shutdown.
func (d *Daemon) Run(ctx context.Context) error {
	if ctx == nil {
		return errors.New("daemon: run context is required")
	}

	sigCh, stopSignals := d.signalSource()
	defer stopSignals()
	runCtx, cancelRun := context.WithCancel(context.WithoutCancel(ctx))
	defer cancelRun()
	bootErr := make(chan error, 1)
	go func() {
		bootErr <- d.boot(runCtx)
	}()

	select {
	case err := <-bootErr:
		if err != nil {
			return err
		}
	case <-ctx.Done():
		d.cancelBootBeforeReady(cancelRun)
		if err := <-bootErr; err != nil {
			return err
		}
		return d.shutdownAfterRunTrigger(ctx)
	case sig, ok := <-sigCh:
		d.cancelBootBeforeReady(cancelRun)
		if err := <-bootErr; err != nil {
			return err
		}
		if ok && sig != nil {
			d.runtimeLogger().Info("daemon: received shutdown signal", "signal", sig.String())
		}
		return d.shutdownAfterRunTrigger(ctx)
	}
	if d.dreamRuntime != nil {
		d.dreamRuntime.Start(runCtx)
	}
	if d.memoryExtractor != nil {
		if err := d.memoryExtractor.Start(runCtx); err != nil {
			shutdownErr := d.shutdownAfterRunTrigger(ctx)
			return errors.Join(
				fmt.Errorf("daemon: start memory extractor: %w", err),
				shutdownErr,
			)
		}
	}
	if err := d.startObserverRetention(runCtx); err != nil {
		shutdownErr := d.shutdownAfterRunTrigger(ctx)
		return errors.Join(
			fmt.Errorf("daemon: start observability retention: %w", err),
			shutdownErr,
		)
	}

	select {
	case <-ctx.Done():
	case sig, ok := <-sigCh:
		if ok && sig != nil {
			d.runtimeLogger().Info("daemon: received shutdown signal", "signal", sig.String())
		}
	}
	return d.shutdownAfterRunTrigger(ctx)
}

func (d *Daemon) cancelBootBeforeReady(cancel context.CancelFunc) {
	if cancel == nil {
		return
	}
	d.mu.Lock()
	readyCh := d.readyCh
	d.mu.Unlock()
	select {
	case <-readyCh:
		return
	default:
		cancel()
	}
}

func (d *Daemon) shutdownAfterRunTrigger(ctx context.Context) error {
	return d.Shutdown(context.WithoutCancel(ctx))
}

// Shutdown gracefully tears down the daemon in the required order.
func (d *Daemon) Shutdown(ctx context.Context) error {
	if d == nil {
		return errors.New("daemon: shutdown requires a daemon")
	}
	if ctx == nil {
		return errors.New("daemon: shutdown context is required")
	}

	operation, started, targets, timeout, err := d.beginShutdownOperation(ctx)
	if err != nil {
		return err
	}
	if started {
		go d.runShutdownOperation(ctx, timeout, operation, targets)
	}
	return waitForShutdownOperation(ctx, operation)
}

func (d *Daemon) gracefulShutdownTimeout() time.Duration {
	if d == nil {
		return defaultShutdownTimeout
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.gracefulShutdownTimeoutLocked()
}

func (d *Daemon) gracefulShutdownTimeoutLocked() time.Duration {
	timeout := defaultShutdownTimeout
	memoryEnabled := d.config.Memory.Enabled
	checkpointDeadline := d.config.Memory.Extractor.Deadline
	if memoryEnabled && checkpointDeadline > 0 {
		timeout += checkpointDeadline + checkpointSummaryStopTimeout
	}
	return timeout
}

func (d *Daemon) shutdownDetached(ctx context.Context, targets *shutdownTargets) error {
	var errs []error
	d.shutdownRuntimeWorkers(ctx, targets, &errs)
	d.shutdownServersAndHooks(ctx, targets, &errs)
	d.shutdownPersistentResources(ctx, targets, &errs)
	return errors.Join(errs...)
}

func (d *Daemon) shutdownServersAndHooks(ctx context.Context, targets *shutdownTargets, errs *[]error) {
	// Withdraw gateway reachability before stopping the servers it advertises.
	if targets.gateway != nil {
		appendWrappedError(errs, "daemon: shutdown gateway", targets.gateway.Close(ctx))
	}
	if targets.httpServer != nil {
		appendWrappedError(errs, "daemon: shutdown http server", targets.httpServer.Shutdown(ctx))
	}
	if targets.udsServer != nil {
		appendWrappedError(errs, "daemon: shutdown uds server", targets.udsServer.Shutdown(ctx))
	}
	if targets.supportBundles != nil {
		appendWrappedError(errs, "daemon: shutdown support bundles", targets.supportBundles.Shutdown(ctx))
	}
	if targets.bridges != nil {
		targets.bridges.Close()
	}
	if targets.network != nil {
		appendWrappedError(errs, "daemon: shutdown network runtime", targets.network.Shutdown(ctx))
	}
	if targets.hooks != nil {
		targets.hooks.Close()
	}
}

func (d *Daemon) runtimeLogger() *slog.Logger {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.logger != nil {
		return d.logger
	}
	return slog.Default()
}

func (d *Daemon) signalSource() (<-chan os.Signal, func()) {
	if d.signalCh != nil {
		return d.signalCh, func() {}
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	return ch, func() {
		signal.Stop(ch)
	}
}
