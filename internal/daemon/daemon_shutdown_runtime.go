package daemon

import "context"

func (d *Daemon) shutdownRuntimeWorkers(ctx context.Context, targets shutdownTargets, errs *[]error) {
	if targets.clarify != nil {
		appendWrappedError(errs, "daemon: close clarification broker", targets.clarify.Close(ctx))
	}
	if targets.networkWakeRunner != nil {
		appendWrappedError(errs, "daemon: shutdown network wake runner", targets.networkWakeRunner.Shutdown(ctx))
	}
	if targets.dreamRuntime != nil {
		targets.dreamRuntime.Shutdown()
	}
	if targets.memoryExtractor != nil {
		appendWrappedError(errs, "daemon: shutdown memory extractor", targets.memoryExtractor.Close(ctx))
	}
	if targets.memoryStore != nil {
		appendWrappedError(
			errs,
			"daemon: shutdown recall signal recorders",
			targets.memoryStore.CloseRecallSignalRecorders(ctx),
		)
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
	targets.runtimeWorkers.shutdown(ctx, errs)
	if err := d.stopSessions(ctx, targets.sessions); err != nil {
		*errs = append(*errs, err)
	}
	if targets.localMemoryProvider != nil {
		appendWrappedError(errs, "daemon: shutdown local memory provider", targets.localMemoryProvider.Shutdown(ctx))
	}
}
