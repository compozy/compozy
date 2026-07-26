package daemon

import (
	"context"
	"fmt"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/memory"
)

func (d *Daemon) bootRuntimeServices(
	ctx context.Context,
	state *bootState,
	cleanup *bootCleanup,
) error {
	if state.cfg.Memory.Enabled {
		state.dreamSvc = d.newDreamService(
			memory.WithMemoryStore(state.memoryStore),
			memory.WithSessionsDir(d.homePaths.SessionsDir),
			memory.WithMinHours(state.cfg.Memory.Dream.MinHours),
			memory.WithMinSessions(state.cfg.Memory.Dream.MinSessions),
			memory.WithDreamGateConfig(dreamGateConfigFromConfig(state.cfg.Memory.Dream)),
			memory.WithLogger(state.logger),
			memory.WithWorkspaceResolver(state.workspaceResolver),
		)
	}

	state.startedAt = d.now().UTC()
	state.notifier = newHooksNotifier(state.logger, d.now)
	if err := d.bootRuntimeMemoryMonitor(ctx, state, cleanup); err != nil {
		return err
	}
	if err := d.bootProcessRegistry(ctx, state); err != nil {
		return err
	}
	sandboxRegistry, err := d.buildSandboxRegistry(state)
	if err != nil {
		return err
	}
	state.sandboxRegistry = sandboxRegistry
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
	state.bridges = d.composeBridgeRuntime(state, cleanup)
	if err := d.bootNotificationPresets(ctx, state); err != nil {
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
	return d.bootMemorySessionRuntime(ctx, state, cleanup)
}

func dreamGateConfigFromConfig(cfg aghconfig.DreamConfig) memory.DreamGateConfig {
	return memory.DreamGateConfig{
		MinCandidates:   cfg.Gates.MinUnpromoted,
		MinRecallCount:  cfg.Gates.MinRecallCount,
		MinScore:        cfg.Gates.MinScore,
		HalfLife:        time.Duration(cfg.Scoring.RecencyHalfLifeDays) * 24 * time.Hour,
		FrequencyWeight: cfg.Scoring.Weights.Frequency,
		RelevanceWeight: cfg.Scoring.Weights.Relevance,
		RecencyWeight:   cfg.Scoring.Weights.Recency,
		FreshnessWeight: cfg.Scoring.Weights.Freshness,
	}
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
