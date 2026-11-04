package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/heartbeat/source"
	"github.com/compozy/compozy/internal/session"
)

func (d *Daemon) bootProactiveChecks(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	manager, ok := state.sessions.(*session.Manager)
	if !ok {
		return nil
	}
	provider, ok := state.registry.(instanceRepositoryProvider)
	if !ok {
		return errors.New("daemon: proactive check repositories unavailable")
	}
	catalog := agentCatalogDependency(
		state.agentCatalog,
		agentSidecarCatalogs{soul: state.soulCatalog, heartbeat: state.heartbeatCatalog},
	)
	if catalog == nil {
		return errors.New("daemon: proactive policy resolver unavailable")
	}
	workspaces, ok := state.registry.(checkWorkspaceReader)
	if !ok {
		return errors.New("daemon: proactive workspace reader unavailable")
	}
	cfg, hb := state.cfg.Agents.StaysOn, state.cfg.Agents.Heartbeat
	environment := &instanceCheckEnvironment{
		sourceConfig: hb.Sources, sourceNow: d.now, sourceMaxInterval: hb.MaxInterval,
		instanceDigestCollector: instanceDigestCollector{
			instances: provider.Instances(), sessions: manager, monitors: state.sessionMonitors,
			workspaces: state.workspaceResolver, logger: state.logger,
		},
		definitions: instanceDefinitions{
			workspaces: state.workspaceResolver,
			profiles:   state.profiles,
			agents:     catalog,
			sessions:   manager,
		},
		policies:   catalog,
		hooks:      state.notifier,
		workspaces: workspaces,
	}
	if poller, ok := state.extensions.(source.ExtensionPoller); ok {
		environment.sourceExtensions = poller
		state.agentInstances.SourceExtensions = poller
	}
	dispatcher := &session.TriageDispatcher{
		Manager:      manager,
		Reports:      provider.InstanceReports(),
		MaxProposals: cfg.MaxProposals,
		StopTimeout:  state.cfg.Session.Supervision.StopGrace,
	}
	service, err := heartbeat.NewCheckService(ctx, provider.Instances(), environment, dispatcher, heartbeat.CheckConfig{
		Sources:             hb.Sources,
		UnattendedTurnLimit: cfg.UnattendedTurnLimit,
		QuietBackoffCap:     hb.QuietBackoffCap,
		FatigueLimit:        hb.FatigueLimit,
		Enabled:             hb.Enabled,
		Interval:            hb.DefaultInterval,
		MinInterval:         hb.MinInterval,
		MaxInterval:         hb.MaxInterval,
		Retention:           hb.WakeEventRetention,
		Timeout:             cfg.TriageTimeout,
		MaxConcurrent:       hb.MaxConcurrentTriage,
		BatchSize:           hb.MaxWakesPerCycle,
	}, d.now)
	if err != nil {
		return fmt.Errorf("daemon: create proactive checks: %w", err)
	}
	service.SetPauseHandler(state.agentInstances.Service.ApplyPause)
	if err := service.Recover(ctx); err != nil {
		return errors.Join(err, service.Shutdown(ctx))
	}
	state.runtimeWorkers.proactiveChecks = service
	state.agentInstances.Checks = service
	state.agentInstances.CheckSettings = service
	cleanup.add(service.Shutdown)
	return nil
}
