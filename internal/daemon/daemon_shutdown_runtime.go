package daemon

import (
	"context"
	"fmt"
)

func (d *Daemon) shutdownRuntimeWorkers(ctx context.Context, targets *shutdownTargets, errs *[]error) {
	workersCtx, cancel := context.WithTimeout(ctx, defaultShutdownTimeout/2)
	defer cancel()
	d.shutdownBackgroundWorkers(workersCtx, targets, errs)
	targets.runtimeWorkers.shutdown(workersCtx, errs)
	d.shutdownRuntimeSessions(ctx, targets, errs)
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
	if targets.mcpStdioPool != nil {
		appendWrappedError(errs, "daemon: close stdio MCP sessions", targets.mcpStdioPool.Close(ctx))
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
	shutdownWorkspaceAndTerminals(ctx, targets, errs)
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
	if err := d.stopSessions(ctx, targets.sessions); err != nil {
		*errs = append(*errs, err)
	}

	if err := d.shutdownSessionManager(ctx, targets.sessions); err != nil {
		*errs = append(*errs, err)
	}
}
