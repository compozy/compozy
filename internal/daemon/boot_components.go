package daemon

import (
	"context"
	"fmt"
)

func (d *Daemon) bootComponents(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	steps := []func() error{
		func() error { return d.bootConfig(state, cleanup) },
		func() error { return d.bootPromptProviders(ctx, state) },
		func() error { return d.bootRuntime(ctx, state, cleanup) },
		func() error { return d.bootSessionRepair(ctx, state) },
		func() error { return d.bootGoalSessionOutboxRelay(ctx, state, cleanup) },
		func() error { return d.bootTasks(ctx, state) },
		func() error { return d.bootSpawnReaper(ctx, state, cleanup) },
		func() error { return d.bootNetwork(ctx, state, cleanup) },
		func() error { return d.bootHooks(ctx, state, cleanup) },
		func() error { return d.startNetworkWakeRunner(ctx, state, cleanup) },
		func() error { return d.bootToolRegistry(ctx, state, cleanup) },
		func() error { return d.bootCoordinator(ctx, state, cleanup) },
		func() error { return d.bootAutomation(ctx, state, cleanup) },
		func() error { return d.bootBundles(ctx, state) },
		func() error { return d.bootResourceReconcile(ctx, state, cleanup) },
		func() error { return d.bootExtensions(ctx, state, cleanup) },
		func() error { return d.bootBridgeDeliveryReconcile(ctx, state) },
		func() error { return d.bootSettings(ctx, state) },
		func() error { return d.bootSupportBundles(state) },
		func() error { return d.bootServers(ctx, state, cleanup) },
		func() error { return d.bootFinalize(ctx, state) },
		func() error { return d.bootTaskRoles(ctx, state) },
		func() error { return startBootLoopCoordinators(ctx, state) },
		func() error { return d.bootScheduler(ctx, state, cleanup) },
	}
	for _, step := range steps {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("daemon: boot canceled: %w", err)
		}
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}
