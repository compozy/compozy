package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store/globaldb"
)

type instanceRepositoryProvider interface {
	SetUnattendedTurnLimit(int64) error
	Instances() *globaldb.InstanceRepo
	InstanceReports() *globaldb.InstanceReportRepo
	SkillReviews() *globaldb.SkillReviewRepo
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
	if err := provider.SetUnattendedTurnLimit(state.cfg.Agents.AlwaysOn.UnattendedTurnLimit); err != nil {
		return err
	}
	repo := provider.Instances()
	manager.SetInstanceCeilingReader(instanceCeilingReader(repo))
	catalog := agentCatalogDependency(state.agentCatalog, agentSidecarCatalogs{
		soul: state.soulCatalog, heartbeat: state.heartbeatCatalog,
	})
	definitions := instanceDefinitions{
		sourceDeclarations: &instanceSourceDeclarations{},
		monitors:           state.sessionMonitors,
		workspaces:         state.workspaceResolver,
		profiles:           state.profiles,
		agents:             catalog,
		sessions:           manager,
		scriptsEnabled:     state.cfg.Agents.Heartbeat.Sources.ScriptsEnabled,
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
		agentinstance.WithActiveLimit(state.cfg.Agents.AlwaysOn.MaxActiveInstances),
		agentinstance.WithClock(d.now),
		agentinstance.WithInstanceChanged(func(instance agentinstance.Instance) {
			manager.PublishInstanceCatalogChanged(instance.ProfileID, instance.WorkspaceID, instance.ID)
		}),
		agentinstance.WithLifecycleHooks(newInstanceLifecycleHooks(state.hookDispatcher, definitions)),
		agentinstance.WithStateEffects(instanceStateEffects(state, repo)),
	)
	if err != nil {
		return fmt.Errorf("daemon: create instance service: %w", err)
	}
	manager.SetInstancePauseHandler(service.AfterPause)
	state.agentInstances = &core.AgentInstanceAPI{
		ReadPeer:      definitions.instancePeer,
		ReadAttention: repo.NeedsAttention,
		Metrics:       repo.Metrics,
		Service:       service, Store: repo, Reports: provider.InstanceReports(),
		WakeState: repo.WakeState, CountActive: repo.CountActive,
		HeldWakes: repo.HeldWakes, UnattendedTurnLimit: state.cfg.Agents.AlwaysOn.UnattendedTurnLimit,
		MaxActiveInstances: state.cfg.Agents.AlwaysOn.MaxActiveInstances,
		FatigueLimit:       state.cfg.Agents.Heartbeat.FatigueLimit,
		ReadPayload: func(ctx context.Context, instance agentinstance.Instance) (contract.AgentInstancePayload, error) {
			return definitions.instancePayload(ctx, instance, repo)
		},
		Authority: agentinstance.Authority{Callers: repo},
		Profiles:  instanceProfileOwners(state.profiles),
	}
	d.bindInstanceSkillReviews(state, provider, definitions)
	state.deps.AgentInstances = state.agentInstances
	return reconcileBootInstances(ctx, state, service)
}

func reconcileBootInstances(ctx context.Context, state *bootState, service *agentinstance.Service) error {
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

func instanceStateEffects(
	state *bootState,
	repo *globaldb.InstanceRepo,
) func(context.Context, agentinstance.Instance) error {
	return func(ctx context.Context, instance agentinstance.Instance) error {
		err := reconcileInstanceState(ctx, state, repo, instance)
		if instance.State == "paused" {
			return errors.Join(err, sweepInstanceReports(ctx, state))
		}
		return err
	}
}

// instanceProfileOwners keeps an unwired profile manager a nil reader, so payloads fall back to
// the default owner label instead of calling through a nil manager.
func instanceProfileOwners(profiles *profile.Manager) core.ProfileOwnerReader {
	if profiles == nil {
		return nil
	}
	return profiles
}
