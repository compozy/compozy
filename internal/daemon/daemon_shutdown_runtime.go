package daemon

import (
	"context"
	"fmt"
	"sync"
)

// shutdownRuntimeWorkers tears down the background workers and stops the sessions side by side, each
// under its own budget, and returns once both have finished. Worker joins that wait on a prompt end
// with the session that owns it instead of spending their budget first. No provider can be spawned
// by a worker still running: the session manager refuses every start once shutdown has begun.
func (d *Daemon) shutdownRuntimeWorkers(ctx context.Context, targets *shutdownTargets, errs *[]error) {
	workersCtx, cancel := context.WithTimeout(ctx, shutdownWorkersBudget)
	defer cancel()
	drainWorkspaceDeletions(workersCtx, targets, errs)

	// Each side collects into its own slice; they are merged after both goroutines are joined.
	var workerErrs, sessionErrs []error
	var teardown sync.WaitGroup
	teardown.Go(func() {
		d.timeConcurrentShutdownStep(ctx, "stop_workers", func() {
			d.shutdownBackgroundWorkers(workersCtx, targets, &workerErrs)
			targets.runtimeWorkers.shutdown(workersCtx, &workerErrs)
		})
	})
	teardown.Go(func() {
		d.timeConcurrentShutdownStep(ctx, "stop_sessions", func() {
			d.shutdownRuntimeSessions(ctx, targets, &sessionErrs)
		})
	})
	teardown.Wait()
	*errs = append(*errs, workerErrs...)
	*errs = append(*errs, sessionErrs...)
}

// shutdownSessionDependencies closes what an in-flight tool call or a stopping session still uses.
// It runs only after every session has stopped, so those calls are not failed by the teardown itself.
func shutdownSessionDependencies(ctx context.Context, targets *shutdownTargets, errs *[]error) {
	if targets.stdioMCP != nil {
		appendWrappedError(errs, "daemon: close stdio MCP sessions", targets.stdioMCP.Close(ctx))
	}
	if targets.terminals != nil {
		appendWrappedError(errs, "daemon: shutdown terminal runtime", targets.terminals.Shutdown(ctx))
	}
}

func (d *Daemon) shutdownBackgroundWorkers(ctx context.Context, targets *shutdownTargets, errs *[]error) {
	if targets.worktrees != nil {
		targets.worktrees.Close()
	}
	if targets.backgroundUpdates != nil {
		appendWrappedError(errs, "daemon: shutdown background updates", targets.backgroundUpdates.Shutdown(ctx))
	}
	if targets.clarify != nil {
		appendWrappedError(errs, "daemon: close clarification broker", targets.clarify.Close(ctx))
	}
	if targets.providerUsage != nil {
		appendWrappedError(errs, "daemon: shutdown provider usage", targets.providerUsage.Shutdown(ctx))
	}
	if targets.modelCatalog != nil {
		appendWrappedError(errs, "daemon: shutdown model catalog", targets.modelCatalog.Shutdown(ctx))
	}
	appendWrappedError(
		errs,
		"daemon: stop skills watcher",
		stopSkillsWatcher(ctx, targets.skillsCancel, targets.skillsDone),
	)
	appendWrappedError(
		errs,
		"daemon: stop loops watcher",
		stopLoopWatcher(ctx, targets.loopsCancel, targets.loopsDone),
	)
	appendWrappedError(
		errs,
		"daemon: stop loop effect relay",
		stopLoopEffectRelay(ctx, targets.effectRelayCancel, targets.effectRelayDone),
	)
	appendWrappedError(
		errs,
		"daemon: stop Goal session outbox relay",
		stopGoalSessionOutboxRelay(ctx, targets.goalOutboxCancel, targets.goalOutboxDone),
	)
	if targets.resourceReconcile != nil {
		appendWrappedError(errs, "daemon: close resource reconcile driver", targets.resourceReconcile.Close(ctx))
	}
	if targets.extensions != nil {
		appendWrappedError(errs, "daemon: stop extensions", targets.extensions.Stop(ctx))
	}
	if targets.automation != nil {
		appendWrappedError(errs, "daemon: shutdown automation", targets.automation.Shutdown(ctx))
	}
	if targets.retention != nil {
		appendWrappedError(errs, "daemon: shutdown observability retention", targets.retention.ShutdownRetention(ctx))
	}
	if targets.scheduler != nil {
		appendWrappedError(errs, "daemon: shutdown scheduler", targets.scheduler.stopLoop(ctx))
	}
	if targets.spawnReaper != nil {
		appendWrappedError(errs, "daemon: shutdown spawn reaper", targets.spawnReaper.shutdown(ctx))
	}
	if targets.coordinator != nil {
		appendWrappedError(errs, "daemon: shutdown coordinator runtime", targets.coordinator.shutdown(ctx))
	}
	if targets.scheduler != nil {
		appendWrappedError(errs, "daemon: shutdown scheduler wake dispatcher", targets.scheduler.shutdownWaker(ctx))
	}
	if targets.tasks != nil {
		appendWrappedError(errs, "daemon: shutdown task runtime", targets.tasks.shutdown(ctx))
	}
	if targets.sessionProcesses != nil {
		targets.sessionProcesses.Shutdown()
	}
	if targets.sessionMonitors != nil {
		appendWrappedError(errs, "daemon: shutdown session monitors", targets.sessionMonitors.Shutdown(ctx))
	}
	if targets.sessionWakeBridge != nil {
		appendWrappedError(errs, "daemon: shutdown session wake bridge", targets.sessionWakeBridge.shutdown(ctx))
	}
}

type replyWatchQuiescer interface {
	QuiesceReplyWatches(context.Context) error
}

func (d *Daemon) shutdownRuntimeSessions(ctx context.Context, targets *shutdownTargets, errs *[]error) {
	// Reply watches close first: a target that settles while sessions stop must not resume a sender
	// being stopped; its fired watch is delivered by the next boot's recovery.
	if quiescer, ok := targets.sessions.(replyWatchQuiescer); ok {
		if err := quiescer.QuiesceReplyWatches(ctx); err != nil {
			*errs = append(*errs, fmt.Errorf("daemon: quiesce reply watches: %w", err))
		}
	}
	// A target stopped by this shutdown says nothing about its work, so session monitors stop
	// evaluating first; they stay active and the next boot re-evaluates them.
	if targets.sessionMonitors != nil {
		targets.sessionMonitors.QuiesceSessionWatches()
	}
	if err := d.stopSessions(ctx, targets.sessions); err != nil {
		*errs = append(*errs, err)
	}

	if err := d.shutdownSessionManager(ctx, targets.sessions); err != nil {
		targets.sessionShutdownIncomplete = true
		*errs = append(*errs, err)
	}
}
