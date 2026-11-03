package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store/globaldb"
)

type instanceRepositoryProvider interface {
	SetUnattendedTurnLimit(int64) error
	Instances() *globaldb.InstanceRepo
	InstanceReports() *globaldb.InstanceReportRepo
}

func (d *Daemon) bootAgentInstances(ctx context.Context, state *bootState) error {
	manager, ok := state.sessions.(*session.Manager)
	if !ok {
		return nil
	}
	provider, ok := state.registry.(instanceRepositoryProvider)
	if !ok {
		return errors.New("daemon: instance repositories unavailable")
	}
	if err := provider.SetUnattendedTurnLimit(state.cfg.Agents.StaysOn.UnattendedTurnLimit); err != nil {
		return err
	}
	repo := provider.Instances()
	manager.SetInstanceCeilingReader(instanceCeilingReader(repo))
	catalog := agentCatalogDependency(state.agentCatalog, agentSidecarCatalogs{
		soul: state.soulCatalog, heartbeat: state.heartbeatCatalog,
	})
	definitions := instanceDefinitions{
		monitors:   state.sessionMonitors,
		workspaces: state.workspaceResolver,
		profiles:   state.profiles,
		agents:     catalog,
		sessions:   manager,
	}
	admission := &instanceAdmission{
		definitions: definitions,
		callers:     repo,
		sessions:    instanceSessionReader{sessions: manager},
		reach:       instanceWorkspaceReach{policy: state.accessPolicy},
	}
	service, err := agentinstance.NewService(
		agentinstance.WithStore(repo), agentinstance.WithAdmission(admission),
		agentinstance.WithMainSessionRotator(&instanceMainRotator{
			sessions: manager,
			store:    repo,
			monitors: state.sessionMonitors,
			starter: instanceMainSessions{
				sessions: manager,
				attempts: repo,
			},
			reports:    provider.InstanceReports(),
			now:        d.now,
			retryDelay: state.cfg.Agents.Heartbeat.DefaultInterval,
			changed: func(instance agentinstance.Instance) {
				manager.PublishInstanceCatalogChanged(instance.ProfileID, instance.WorkspaceID, instance.ID)
			},
		}),
		agentinstance.WithMainSessions(instanceMainSessions{sessions: manager, attempts: repo}),
		agentinstance.WithActiveLimit(state.cfg.Agents.StaysOn.MaxActiveInstances),
		agentinstance.WithClock(d.now),
		agentinstance.WithInstanceChanged(func(instance agentinstance.Instance) {
			manager.PublishInstanceCatalogChanged(instance.ProfileID, instance.WorkspaceID, instance.ID)
		}),
		agentinstance.WithLifecycleHooks(state.hookDispatcher),
		agentinstance.WithStateEffects(func(ctx context.Context, instance agentinstance.Instance) error {
			return reconcileInstanceState(ctx, state, repo, instance)
		}),
	)
	if err != nil {
		return fmt.Errorf("daemon: create instance service: %w", err)
	}
	manager.SetInstancePauseHandler(service.AfterPause)
	state.agentInstances = &core.AgentInstanceAPI{
		Service: service, Store: repo, Reports: provider.InstanceReports(),
		WakeState: repo.WakeState, CountActive: repo.CountActive,
		HeldWakes: repo.HeldWakes, UnattendedTurnLimit: state.cfg.Agents.StaysOn.UnattendedTurnLimit,
		MaxActiveInstances: state.cfg.Agents.StaysOn.MaxActiveInstances,
		FatigueLimit:       state.cfg.Agents.Heartbeat.FatigueLimit,
		ReadPayload: func(ctx context.Context, instance agentinstance.Instance) (contract.AgentInstancePayload, error) {
			return definitions.instancePayload(ctx, instance, repo)
		},
		Authority: agentinstance.Authority{Callers: repo},
	}
	state.deps.AgentInstances = state.agentInstances
	bindInstanceCatalogReconciliation(ctx, state, service.Reconcile)
	if err := service.Reconcile(ctx); err != nil {
		state.logger.ErrorContext(ctx, "daemon: reconcile instances at startup", "error", err)
	}
	return sweepInstanceReports(ctx, state)
}

func bindInstanceCatalogReconciliation(ctx context.Context, state *bootState, reconcile func(context.Context) error) {
	if state.agentCatalog != nil {
		state.agentCatalog.setOnChange(func() {
			if state.toolProjectionEpoch != nil {
				state.toolProjectionEpoch.Advance()
			}
			if err := reconcile(ctx); err != nil {
				state.logger.ErrorContext(ctx, "daemon: reconcile instances after agent catalog change", "error", err)
			}
		})
	}
}

func cancelInstanceCheck(ctx context.Context, state *bootState, id string) error {
	if state.runtimeWorkers.proactiveChecks == nil {
		return nil
	}
	return state.runtimeWorkers.proactiveChecks.CancelInstance(ctx, id)
}
