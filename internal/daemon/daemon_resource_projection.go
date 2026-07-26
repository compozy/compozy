package daemon

import (
	"context"

	automationpkg "github.com/compozy/agh/internal/automation"

	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/heartbeat"

	"github.com/compozy/agh/internal/resources"

	"github.com/compozy/agh/internal/soul"
	"github.com/compozy/agh/internal/windowmanager"

	toolspkg "github.com/compozy/agh/internal/tools"
)

func (d *Daemon) applyAutomationManagerFactoryDefault() {
	if d.newAutomationManager != nil {
		return
	}
	d.newAutomationManager = func(deps automationManagerDeps) (automationRuntime, error) {
		jobStore, triggerStore, err := automationResourceStores(deps.ResourceStore, deps.ResourceCodecs)
		if err != nil {
			return nil, err
		}
		resourceOpts := []automationpkg.Option(nil)
		if jobStore != nil && triggerStore != nil {
			resourceOpts = append(resourceOpts, automationpkg.WithResourceDefinitions(
				jobStore,
				triggerStore,
				resourceReconcileActor(),
				deps.ResourceTrigger,
			))
		}
		loopStarter, err := newAutomationLoopStarter(
			deps.Store,
			deps.LoopCatalog,
			deps.ToolRegistry,
			d.homePaths,
			deps.WorkspaceResolver,
			deps.ParticipationResolver,
		)
		if err != nil {
			return nil, err
		}

		managerOpts := []automationpkg.Option{
			automationpkg.WithStore(deps.Store),
			automationpkg.WithSessions(deps.Sessions),
			automationpkg.WithTasks(deps.Tasks),
			automationpkg.WithWorkspaceResolver(deps.WorkspaceResolver),
			automationpkg.WithConfig(deps.Config),
			automationpkg.WithHooks(deps.Hooks),
			automationpkg.WithWebhookSecretStore(deps.WebhookSecrets),
			automationpkg.WithLogger(deps.Logger),
			automationpkg.WithGlobalWorkspacePath(deps.GlobalWorkspacePath),
		}
		if loopStarter != nil {
			managerOpts = append(managerOpts, automationpkg.WithLoopStarter(loopStarter))
		}
		managerOpts = append(managerOpts, resourceOpts...)

		manager, err := automationpkg.New(managerOpts...)
		if err != nil {
			return nil, err
		}
		return manager, nil
	}
}

func (d *Daemon) applyResourceReconcileDriverFactoryDefault() {
	if d.newResourceReconcile != nil {
		return
	}
	d.newResourceReconcile = func(
		_ context.Context,
		deps resourceReconcileDriverDeps,
	) (resources.ReconcileDriver, error) {
		if deps.ResourceStore == nil || deps.CodecRegistry == nil {
			return resources.NewReconcileDriver(
				nil,
				resources.MutationActor{},
				nil,
				resources.WithReconcileLogger(deps.Logger),
			)
		}

		registrations, err := buildResourceProjectorRegistrations(&deps)
		if err != nil {
			return nil, err
		}

		return resources.NewReconcileDriver(
			deps.ResourceStore,
			resourceReconcileActor(),
			registrations,
			resources.WithReconcileLogger(deps.Logger),
		)
	}
}

func buildResourceProjectorRegistrations(
	deps *resourceReconcileDriverDeps,
) ([]resources.ProjectorRegistration, error) {
	var registrations []resources.ProjectorRegistration
	var err error
	registrations, err = appendCoreProjectorRegistrations(registrations, deps)
	if err != nil {
		return nil, err
	}
	if deps.Automation != nil {
		registrations, err = appendAutomationProjectorRegistrations(registrations, deps)
		if err != nil {
			return nil, err
		}
	}
	if deps.Bridges != nil {
		registrations, err = appendBridgeProjectorRegistration(registrations, deps)
		if err != nil {
			return nil, err
		}
	}
	if deps.Bundles != nil {
		registrations, err = appendBundleProjectorRegistrations(registrations, deps)
		if err != nil {
			return nil, err
		}
	}
	return registrations, nil
}

func appendCoreProjectorRegistrations(
	registrations []resources.ProjectorRegistration,
	deps *resourceReconcileDriverDeps,
) ([]resources.ProjectorRegistration, error) {
	registrations, err := appendAuthoredCoreProjectorRegistrations(registrations, deps)
	if err != nil {
		return nil, err
	}
	return appendCapabilityCoreProjectorRegistrations(registrations, deps)
}

func appendAuthoredCoreProjectorRegistrations(
	registrations []resources.ProjectorRegistration,
	deps *resourceReconcileDriverDeps,
) ([]resources.ProjectorRegistration, error) {
	var err error
	if deps.Hooks != nil {
		registrations, err = appendTypedProjectorRegistration(
			registrations,
			deps.CodecRegistry,
			hookBindingResourceKind,
			newHookBindingProjector(deps.Hooks),
		)
	}
	if err != nil {
		return nil, err
	}
	if deps.AgentCatalog != nil {
		registrations, err = appendTypedProjectorRegistration(
			registrations,
			deps.CodecRegistry,
			aghconfig.AgentResourceKind,
			newAgentProjector(deps.AgentCatalog),
		)
	}
	if err != nil {
		return nil, err
	}
	if deps.SoulCatalog != nil {
		registrations, err = appendTypedProjectorRegistration(
			registrations,
			deps.CodecRegistry,
			soul.ResourceKind,
			newSoulProjector(deps.SoulCatalog),
		)
	}
	if err != nil {
		return nil, err
	}
	if deps.HeartbeatCatalog != nil {
		registrations, err = appendTypedProjectorRegistration(
			registrations,
			deps.CodecRegistry,
			heartbeat.ResourceKind,
			newHeartbeatProjector(deps.HeartbeatCatalog),
		)
	}
	if err != nil {
		return nil, err
	}
	return registrations, nil
}

func appendCapabilityCoreProjectorRegistrations(
	registrations []resources.ProjectorRegistration,
	deps *resourceReconcileDriverDeps,
) ([]resources.ProjectorRegistration, error) {
	var err error
	if deps.ToolCatalog != nil {
		registrations, err = appendTypedProjectorRegistration(
			registrations,
			deps.CodecRegistry,
			toolspkg.ToolResourceKind,
			newToolProjector(deps.ToolCatalog),
		)
	}
	if err != nil {
		return nil, err
	}
	if deps.MCPServerCatalog != nil {
		registrations, err = appendTypedProjectorRegistration(
			registrations,
			deps.CodecRegistry,
			aghconfig.MCPServerResourceKind,
			newMCPServerProjector(deps.MCPServerCatalog),
		)
	}
	if err != nil {
		return nil, err
	}
	if deps.WindowLayoutCatalog != nil {
		registrations, err = appendTypedProjectorRegistration(
			registrations,
			deps.CodecRegistry,
			windowmanager.WindowLayoutResourceKind,
			newWindowLayoutProjector(deps.WindowLayoutCatalog),
		)
	}
	if err != nil {
		return nil, err
	}
	registrations, err = appendLoopProjectorRegistration(registrations, deps)
	if err != nil {
		return nil, err
	}
	return appendSkillProjectorRegistration(registrations, deps)
}

func appendTypedProjectorRegistration[T any](
	registrations []resources.ProjectorRegistration,
	registry *resources.CodecRegistry,
	kind resources.ResourceKind,
	projector resources.TypedProjector[T],
) ([]resources.ProjectorRegistration, error) {
	codec, err := resources.ResolveCodec[T](registry, kind)
	if err != nil {
		return nil, err
	}
	registration, err := resources.NewTypedProjectorRegistration(codec, projector)
	if err != nil {
		return nil, err
	}
	return append(registrations, registration), nil
}

func appendAutomationProjectorRegistrations(
	registrations []resources.ProjectorRegistration,
	deps *resourceReconcileDriverDeps,
) ([]resources.ProjectorRegistration, error) {
	jobCodec, err := resources.ResolveCodec[automationpkg.Job](deps.CodecRegistry, automationpkg.JobResourceKind)
	if err != nil {
		return nil, err
	}
	jobRegistration, err := resources.NewTypedProjectorRegistration(
		jobCodec,
		newAutomationJobProjector(deps.Automation),
	)
	if err != nil {
		return nil, err
	}

	triggerCodec, err := resources.ResolveCodec[automationpkg.Trigger](
		deps.CodecRegistry,
		automationpkg.TriggerResourceKind,
	)
	if err != nil {
		return nil, err
	}
	triggerRegistration, err := resources.NewTypedProjectorRegistration(
		triggerCodec,
		newAutomationTriggerProjector(deps.Automation),
	)
	if err != nil {
		return nil, err
	}

	registrations = append(registrations, jobRegistration, triggerRegistration)
	return registrations, nil
}

func resourceReconcileActor() resources.MutationActor {
	return resources.MutationActor{
		Kind: resources.MutationActorKindDaemon,
		ID:   "daemon-control",
		Source: resources.ResourceSource{
			Kind: resources.ResourceSourceKind("daemon"),
			ID:   string(SessionClassSystem),
		},
		MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
	}
}
