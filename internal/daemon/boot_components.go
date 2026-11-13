package daemon

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
)

// bootStep names one serial boot step so its duration and the current step are observable.
type bootStep struct {
	name string
	run  func() error
}

func (d *Daemon) bootComponents(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	bootStarted := time.Now()
	d.serving.Store(false)
	steps := []bootStep{
		{name: "bootConfig", run: func() error { return d.bootConfig(state, cleanup) }},
		{name: "bootPromptProviders", run: func() error { return d.bootPromptProviders(ctx, state) }},
		{name: "bootRuntimeFoundation", run: func() error { return d.bootRuntimeFoundation(ctx, state, cleanup) }},
		{name: "bootSessionAttachments", run: func() error { return d.bootSessionAttachments(ctx, state, cleanup) }},
		{name: "bootRuntime", run: func() error { return d.bootRuntime(ctx, state, cleanup) }},
		{name: "bootSessionRepair", run: func() error { return d.bootSessionRepair(ctx, state) }},
		{name: "bootGoalSessionOutboxRelay",
			run: func() error { return d.bootGoalSessionOutboxRelay(ctx, state, cleanup) },
		},
		{name: "bootTasks", run: func() error { return d.bootTasks(ctx, state, cleanup) }},
		{name: "bootSpawnReaper", run: func() error { return d.bootSpawnReaper(ctx, state, cleanup) }},
		{name: "bootHooks", run: func() error { return d.bootHooks(ctx, state, cleanup) }},
		{name: "bootWorktrees", run: func() error { return d.bootWorktrees(ctx, state, cleanup) }},
		{name: "recoverBootProfileMoves", run: func() error { return recoverBootProfileMoves(ctx, state) }},
		{name: "bootSubagents", run: func() error { return d.bootSubagents(ctx, state) }},
		{name: "bootReplyWatches", run: func() error { return d.bootReplyWatches(ctx, state) }},
		{name: "bootToolRegistry", run: func() error { return d.bootToolRegistry(ctx, state, cleanup) }},
		{name: "bootCmdPalette", run: func() error { return d.bootCmdPalette(ctx, state, cleanup) }},
		{name: "bootLoopEffectRelay", run: func() error { return d.bootLoopEffectRelay(ctx, state, cleanup) }},
		{name: "bootCoordinator", run: func() error { return d.bootCoordinator(ctx, state, cleanup) }},
		{name: "bootAutomation", run: func() error { return d.bootAutomation(ctx, state, cleanup) }},
		{name: "bootResourceReconcile", run: func() error { return d.bootResourceReconcile(ctx, state, cleanup) }},
		{name: "bootExtensions", run: func() error { return d.bootExtensions(ctx, state, cleanup) }},
		{name: "bootResourceWatchers", run: func() error { return d.bootResourceWatchers(ctx, state, cleanup) }},
		{name: "bootSettings", run: func() error { return d.bootSettings(ctx, state) }},
		{name: "bootManagedDeliveries", run: func() error { return d.bootManagedDeliveries(ctx, state, cleanup) }},
		{name: "bootSupportBundles", run: func() error { return d.bootSupportBundles(state, cleanup) }},
		{name: "bootAgentInstances", run: func() error { return d.bootAgentInstances(ctx, state) }},
		{name: "bootProactiveChecks", run: func() error { return d.bootProactiveChecks(ctx, state, cleanup) }},
		{name: "bootMCPDeclarationWatcher", run: func() error {
			d.bootMCPDeclarationWatcher(ctx, state, cleanup)
			return nil
		}},
		{name: "prepareServerDependencies", run: func() error { return d.prepareServerDependencies(ctx, state) }},
		{name: "bootGateway", run: func() error { return d.bootGateway(ctx, state, cleanup) }},
		{name: "bootServers", run: func() error { return d.bootServers(ctx, state, cleanup) }},
		{name: "bootProviderAuthStartup",
			run: func() error { return d.bootProviderAuthStartup(ctx, state, cleanup) },
		},
		// Everything below runs in the serving phase: the read model is reachable, and only now may
		// recovery start providers, whose hosted MCP proxies need the listeners.
		{name: "enterServingPhase", run: func() error { return d.enterServingPhase(ctx, state, bootStarted) }},
		{name: "reconcileBootInstances", run: func() error { return reconcileBootInstances(ctx, state) }},
		{name: "recoverBootDelegatedSessions", run: func() error { return recoverBootDelegatedSessions(ctx, state) }},
		{name: "resumeBootInterruptedSessions",
			run: func() error { return resumeBootInterruptedSessions(ctx, state.sessions) },
		},
		{name: "recoverBootMonitors", run: func() error { return recoverBootMonitors(ctx, state) }},
		{name: "bootFinalize", run: func() error { return d.bootFinalize(ctx, state) }},
		{name: "bootTaskRoles", run: func() error { return d.bootTaskRoles(ctx, state) }},
		{name: "startBootLoopCoordinators", run: func() error { return startBootLoopCoordinators(ctx, state) }},
		{name: "bootScheduler", run: func() error { return d.bootScheduler(ctx, state, cleanup) }},
		{name: "bootSessionSupervision", run: func() error { return d.bootSessionSupervision(ctx, state, cleanup) }},
		{name: "bootSessionLaterReturns",
			run: func() error { return d.bootSessionLaterReturns(ctx, state, cleanup) },
		},
	}
	for _, step := range steps {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("daemon: boot canceled: %w", err)
		}
		phase := contract.DaemonReadinessStarting
		if d.serving.Load() {
			phase = contract.DaemonReadinessServing
		}
		// state.logger exists only after bootConfig; earlier steps log through the default logger.
		err := d.timeLifecycleStep(ctx, state.logger, phase, step.name, step.run)
		if err != nil {
			return err
		}
	}
	return nil
}

// enterServingPhase marks the point where HTTP and UDS listen and the read model (session catalog,
// transcripts, status) is served while recovery is still running. `status` stays `draining` and
// new-work admission stays closed until boot publishes readiness, so an older app keeps waiting
// for `running` exactly as before; newer clients read `readiness.phase`.
func (d *Daemon) enterServingPhase(ctx context.Context, state *bootState, bootStarted time.Time) error {
	d.serving.Store(true)
	if marker, ok := state.sessions.(interface{ MarkTransportsListening() }); ok {
		marker.MarkTransportsListening()
	}
	state.logger.InfoContext(ctx, "daemon: serving", "since_boot_ms", time.Since(bootStarted).Milliseconds())
	return nil
}
