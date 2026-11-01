package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store/globaldb"
)

type instanceRepositoryProvider interface {
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
	repo := provider.Instances()
	manager.SetInstanceCeilingReader(instanceCeilingReader(repo))
	catalog := agentCatalogDependency(state.agentCatalog, agentSidecarCatalogs{
		soul: state.soulCatalog, heartbeat: state.heartbeatCatalog,
	})
	admission := &instanceAdmission{
		definitions: instanceDefinitions{
			workspaces: state.workspaceResolver,
			profiles:   state.profiles,
			agents:     catalog,
			sessions:   manager,
		},
		callers:  repo,
		sessions: instanceSessionReader{sessions: manager},
		reach:    instanceWorkspaceReach{policy: state.accessPolicy},
	}
	service, err := agentinstance.NewService(
		agentinstance.WithStore(repo), agentinstance.WithAdmission(admission),
		agentinstance.WithMainSessions(instanceMainSessions{sessions: manager, attempts: repo}),
		agentinstance.WithActiveLimit(state.cfg.Agents.StaysOn.MaxActiveInstances),
		agentinstance.WithClock(d.now),
	)
	if err != nil {
		return fmt.Errorf("daemon: create instance service: %w", err)
	}
	state.agentInstances = &core.AgentInstanceAPI{
		Service: service, Store: repo, Reports: provider.InstanceReports(),
		Authority: agentinstance.Authority{Callers: repo},
	}
	state.deps.AgentInstances = state.agentInstances
	return errors.Join(service.Reconcile(ctx), sweepInstanceReports(ctx, state))
}
