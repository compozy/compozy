package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"syscall"
)

// Run boots the daemon, blocks until signal or context cancellation, then performs graceful shutdown.
func (d *Daemon) Run(ctx context.Context) error {
	if ctx == nil {
		return errors.New("daemon: run context is required")
	}

	sigCh, stopSignals := d.signalSource()
	defer stopSignals()
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	receivedSignal := make(chan os.Signal, 1)
	signalDone := make(chan struct{})
	go func() {
		defer close(signalDone)
		select {
		case <-runCtx.Done():
			return
		case sig, ok := <-sigCh:
			if ok && sig != nil {
				select {
				case receivedSignal <- sig:
				default:
				}
				cancelRun()
			}
		}
	}()

	if err := d.boot(runCtx); err != nil {
		cancelRun()
		<-signalDone
		return err
	}
	if d.dreamRuntime != nil {
		d.dreamRuntime.Start(runCtx)
	}
	if d.memoryExtractor != nil {
		if err := d.memoryExtractor.Start(runCtx); err != nil {
			cancelRun()
			<-signalDone
			shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
			defer cancel()
			shutdownErr := d.Shutdown(shutdownCtx)
			return errors.Join(
				fmt.Errorf("daemon: start memory extractor: %w", err),
				shutdownErr,
			)
		}
	}
	if err := d.startObserverRetention(runCtx); err != nil {
		cancelRun()
		<-signalDone
		shutdownCtx, cancel := daemonShutdownContext(ctx)
		defer cancel()
		shutdownErr := d.Shutdown(shutdownCtx)
		return errors.Join(
			fmt.Errorf("daemon: start observability retention: %w", err),
			shutdownErr,
		)
	}

	select {
	case <-ctx.Done():
	case sig := <-receivedSignal:
		d.runtimeLogger().Info("daemon: received shutdown signal", "signal", sig.String())
	}
	cancelRun()
	<-signalDone

	shutdownCtx, cancel := daemonShutdownContext(ctx)
	defer cancel()

	return d.Shutdown(shutdownCtx)
}

// Shutdown gracefully tears down the daemon in the required order.
func (d *Daemon) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.TODO()
	}
	drainErr := d.Drain(context.WithoutCancel(ctx))
	return errors.Join(drainErr, d.shutdownDetached(ctx, d.detachShutdownTargets()))
}

func daemonShutdownContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.TODO()
	}
	return context.WithTimeout(context.WithoutCancel(parent), defaultShutdownTimeout)
}

func (d *Daemon) shutdownDetached(ctx context.Context, targets shutdownTargets) error {
	var errs []error
	d.shutdownRuntimeWorkers(ctx, targets, &errs)
	d.shutdownServersAndHooks(ctx, targets, &errs)
	d.shutdownPersistentResources(ctx, targets, &errs)
	return errors.Join(errs...)
}

func (d *Daemon) shutdownServersAndHooks(ctx context.Context, targets shutdownTargets, errs *[]error) {
	if targets.httpServer != nil {
		appendWrappedError(errs, "daemon: shutdown http server", targets.httpServer.Shutdown(ctx))
	}
	if targets.udsServer != nil {
		appendWrappedError(errs, "daemon: shutdown uds server", targets.udsServer.Shutdown(ctx))
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
