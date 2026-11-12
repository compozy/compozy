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

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
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

// shutdownAfterRunTrigger retains discovery withdrawal while retrying terminal teardown.
// It is the process-terminal path, so a watchdog bounds it: a hung teardown cannot keep serving.
func (d *Daemon) shutdownAfterRunTrigger(ctx context.Context) error {
	disarm := d.armShutdownWatchdog(shutdownWatchdogBudget(d.gracefulShutdownTimeout()))
	defer disarm()
	return d.shutdownWithOperation(context.WithoutCancel(ctx), true)
}

// Shutdown gracefully tears down the daemon in the required order.
func (d *Daemon) Shutdown(ctx context.Context) error {
	return d.shutdownWithOperation(ctx, false)
}

// shutdownWithOperation shares teardown while retaining terminal discovery withdrawal across retries.
func (d *Daemon) shutdownWithOperation(ctx context.Context, withdrawDiscovery bool) error {
	if d == nil {
		return errors.New("daemon: shutdown requires a daemon")
	}
	if ctx == nil {
		return errors.New("daemon: shutdown context is required")
	}

	operation, started, targets, timeout, err := d.beginShutdownOperation(ctx, withdrawDiscovery)
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
	return GracefulShutdownTimeout(&d.config)
}

// GracefulShutdownTimeout returns the daemon's complete shutdown budget for a config.
func GracefulShutdownTimeout(cfg *compozyconfig.Config) time.Duration {
	if cfg == nil {
		return defaultShutdownTimeout
	}
	return defaultShutdownTimeout + cfg.Daemon.ShutdownGrace + session.TerminationBudget(cfg.Session.Stop)
}

func (d *Daemon) shutdownDetached(
	ctx context.Context,
	operation *daemonShutdownOperation,
	targets *shutdownTargets,
) error {
	var errs []error
	phase := func(name string, run func()) {
		// Phases collect their own errors in errs; the timer only names and measures the phase.
		if err := d.timeShutdownPhase(ctx, name, func() error {
			run()
			return nil
		}); err != nil {
			errs = append(errs, err)
		}
	}
	phase("stop_workers_and_sessions", func() { d.shutdownRuntimeWorkers(ctx, targets, &errs) })
	// In-flight tool calls and stopping sessions use these until the last session has stopped. A
	// retryable shutdown that left the session manager unfinished keeps them open for the sessions
	// still stopping; the retry that joins that cleanup, or the process-terminal path, closes them.
	phase("close_session_dependencies", func() {
		if targets.sessionShutdownIncomplete && !d.shutdownIsTerminal(operation) {
			return
		}
		dependenciesCtx, cancel := detachedShutdownBudget(ctx, shutdownSessionDependenciesBudget)
		defer cancel()
		shutdownSessionDependencies(dependenciesCtx, targets, &errs)
	})
	phase("close_servers_and_hooks", func() { d.shutdownServersAndHooks(ctx, targets, &errs) })
	// The database is checkpointed on every exit, so this phase never inherits an expired deadline.
	phase("close_persistent_resources", func() {
		persistCtx, cancel := detachedShutdownBudget(ctx, shutdownPersistBudget)
		defer cancel()
		d.shutdownPersistentResources(persistCtx, targets, d.shutdownIsTerminal(operation), &errs)
	})
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
