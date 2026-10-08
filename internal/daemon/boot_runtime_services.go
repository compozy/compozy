package daemon

import (
	"context"
	"fmt"
)

func (d *Daemon) bootRuntimeServices(
	ctx context.Context,
	state *bootState,
	cleanup *bootCleanup,
) error {
	state.notifier = newHooksNotifier(state.logger, d.now)
	if err := d.bootRuntimeMemoryMonitor(ctx, state, cleanup); err != nil {
		return err
	}
	if err := d.bootProcessRegistry(ctx, state); err != nil {
		return err
	}
	if err := d.bootTerminal(ctx, state, cleanup); err != nil {
		return err
	}
	providerVault, err := d.buildProviderVault(state)
	if err != nil {
		return err
	}
	state.providerVault = providerVault
	if err := d.bootModelCatalog(ctx, state, cleanup); err != nil {
		return err
	}
	if err := d.bootMarketplace(ctx, state, cleanup); err != nil {
		return err
	}
	hostedMCP, err := d.buildHostedMCPService(state)
	if err != nil {
		return err
	}
	state.hostedMCP = hostedMCP

	if err := d.bootRuntimeResourceGraph(state); err != nil {
		return err
	}
	initializeRoleResolver(state)
	return d.bootSessionRuntime(ctx, state, cleanup)
}

func (d *Daemon) bootRuntimeMemoryMonitor(
	ctx context.Context,
	state *bootState,
	cleanup *bootCleanup,
) error {
	monitor := newRuntimeMemoryMonitor(
		state.cfg.Daemon.MemoryReportInterval,
		state.startedAt,
		state.logger,
		func(options *runtimeMemoryMonitorOptions) { options.now = d.now },
	)
	if err := monitor.Start(ctx); err != nil {
		return fmt.Errorf("daemon: start runtime memory monitor: %w", err)
	}
	cleanup.add(monitor.Shutdown)
	state.runtimeWorkers.runtimeMemory = monitor
	return nil
}
