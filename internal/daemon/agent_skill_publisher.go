package daemon

import (
	"context"
	"errors"

	"fmt"
	"log/slog"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"

	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/soul"

	skillspkg "github.com/compozy/compozy/internal/skills"

	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func (d *Daemon) newAgentSkillPublisher(
	state *bootState,
	registry *extensionpkg.Registry,
) (agentSkillPublisher, error) {
	publisher := agentSkillPublisher(agentSkillPublisherFunc(func(context.Context) error { return nil }))
	if state == nil {
		return publisher, nil
	}
	if state.resourceKernel == nil || state.resourceCodecs == nil {
		return publisher, nil
	}
	resolved, err := resolveAgentSkillPublisherResources(state)
	if err != nil {
		return nil, err
	}

	publisher = newAgentSkillSourceSyncer(
		state.resourceKernel,
		resolved.agentStore,
		resolved.agentCodec,
		newAgentProjector(state.agentCatalog),
		resolved.skillStore,
		resolved.skillCodec,
		newSkillProjector(state.skillsRegistry),
		resolved.mcpStore,
		resolved.mcpCodec,
		agentSkillSyncActor(),
		state.logger,
		func(ctx context.Context, kind resources.ResourceKind, reason resources.ReconcileReason) error {
			if state.resourceReconcile == nil {
				return nil
			}
			return state.resourceReconcile.Trigger(ctx, kind, reason)
		},
		daemonAgentSkillDeclarationProvider(
			d.homePaths,
			state.registry,
			state.workspaceResolver,
			state.skillsRegistry,
			state.logger,
		),
		extensionAgentSkillDeclarationProvider(registry, state.currentExtensionRuntime, state.logger),
	)
	syncer, ok := publisher.(*agentSkillSourceSyncer)
	if !ok {
		return nil, errors.New("daemon: create agent/skill source syncer")
	}
	syncer.soulCodec = resolved.soulCodec
	syncer.soulStore = resolved.soulStore
	syncer.heartbeatCodec = resolved.heartbeatCodec
	syncer.heartbeatStore = resolved.heartbeatStore
	return publisher, nil
}

type agentSkillPublisherResources struct {
	agentCodec     resources.KindCodec[compozyconfig.AgentDef]
	agentStore     resources.Store[compozyconfig.AgentDef]
	skillCodec     resources.KindCodec[skillspkg.SkillResourceSpec]
	skillStore     resources.Store[skillspkg.SkillResourceSpec]
	mcpCodec       resources.KindCodec[compozyconfig.MCPServer]
	mcpStore       resources.Store[compozyconfig.MCPServer]
	soulCodec      resources.KindCodec[soul.ResourceSpec]
	soulStore      resources.Store[soul.ResourceSpec]
	heartbeatCodec resources.KindCodec[heartbeat.ResourceSpec]
	heartbeatStore resources.Store[heartbeat.ResourceSpec]
}

func resolveAgentSkillPublisherResources(state *bootState) (agentSkillPublisherResources, error) {
	var resolved agentSkillPublisherResources
	var err error
	resolved.agentCodec, err = resources.ResolveCodec[compozyconfig.AgentDef](
		state.resourceCodecs,
		compozyconfig.AgentResourceKind,
	)
	if err != nil {
		return agentSkillPublisherResources{}, fmt.Errorf("daemon: resolve agent codec: %w", err)
	}
	resolved.agentStore, err = resources.NewStore(state.resourceKernel, resolved.agentCodec)
	if err != nil {
		return agentSkillPublisherResources{}, fmt.Errorf("daemon: create agent store: %w", err)
	}
	resolved.skillCodec, err = resources.ResolveCodec[skillspkg.SkillResourceSpec](
		state.resourceCodecs,
		skillspkg.SkillResourceKind,
	)
	if err != nil {
		return agentSkillPublisherResources{}, fmt.Errorf("daemon: resolve skill codec: %w", err)
	}
	resolved.skillStore, err = resources.NewStore(state.resourceKernel, resolved.skillCodec)
	if err != nil {
		return agentSkillPublisherResources{}, fmt.Errorf("daemon: create skill store: %w", err)
	}
	resolved.mcpCodec, err = resources.ResolveCodec[compozyconfig.MCPServer](
		state.resourceCodecs,
		compozyconfig.MCPServerResourceKind,
	)
	if err != nil {
		return agentSkillPublisherResources{}, fmt.Errorf("daemon: resolve mcp server codec: %w", err)
	}
	resolved.mcpStore, err = resources.NewStore(state.resourceKernel, resolved.mcpCodec)
	if err != nil {
		return agentSkillPublisherResources{}, fmt.Errorf("daemon: create mcp server store: %w", err)
	}
	resolved.soulCodec, resolved.soulStore, err = resolveDaemonResourceStore[soul.ResourceSpec](
		state,
		state.resourceKernel,
		soul.ResourceKind,
		"extension soul",
	)
	if err != nil {
		return agentSkillPublisherResources{}, err
	}
	resolved.heartbeatCodec, resolved.heartbeatStore, err = resolveDaemonResourceStore[heartbeat.ResourceSpec](
		state,
		state.resourceKernel,
		heartbeat.ResourceKind,
		"extension heartbeat",
	)
	if err != nil {
		return agentSkillPublisherResources{}, err
	}
	return resolved, nil
}

func daemonAgentSkillDeclarationProvider(
	homePaths compozyconfig.HomePaths,
	registry Registry,
	workspaceResolver workspacepkg.RuntimeResolver,
	skillsRegistry *skillspkg.Registry,
	logger *slog.Logger,
) agentSkillDeclarationProvider {
	return func(ctx context.Context) (agentSkillDesiredResources, error) {
		desired := agentSkillDesiredResources{}
		globalScope := resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal}
		globalAgents, err := compozyconfig.LoadWorkspaceAgentDefs("", nil, homePaths)
		if err != nil {
			return agentSkillDesiredResources{}, fmt.Errorf("daemon: discover global agents: %w", err)
		}
		appendAgentResources(&desired, globalScope, "config/global", globalAgents)

		if skillsRegistry != nil {
			globalSkills, _, err := skillsRegistry.DiscoverGlobal(ctx)
			if err != nil {
				return agentSkillDesiredResources{}, fmt.Errorf("daemon: discover global skills: %w", err)
			}
			appendSkillResources(&desired, globalScope, "skills/global", globalSkills)
		}

		workspaces, err := registeredWorkspaces(ctx, registry, workspaceResolver, logger)
		if err != nil {
			return agentSkillDesiredResources{}, err
		}
		for idx := range workspaces {
			resolved := &workspaces[idx]
			scope := resources.ResourceScope{
				Kind: resources.ResourceScopeKindWorkspace,
				ID:   strings.TrimSpace(resolved.ID),
			}
			appendAgentResources(&desired, scope, "config/workspace/"+scope.ID, resolved.Agents)
			if skillsRegistry == nil {
				continue
			}
			workspaceSkills, _, err := skillsRegistry.DiscoverWorkspace(ctx, resolved)
			if err != nil {
				return agentSkillDesiredResources{}, fmt.Errorf(
					"daemon: discover workspace %q skills: %w",
					scope.ID,
					err,
				)
			}
			appendSkillResources(&desired, scope, "skills/workspace/"+scope.ID, workspaceSkills)
		}

		return desired, nil
	}
}

func extensionAgentSkillDeclarationProvider(
	registry *extensionpkg.Registry,
	runtime func() extensionRuntime,
	logger *slog.Logger,
) agentSkillDeclarationProvider {
	return func(ctx context.Context) (agentSkillDesiredResources, error) {
		if err := ctx.Err(); err != nil {
			return agentSkillDesiredResources{}, err
		}
		if registry == nil || runtime == nil {
			return agentSkillDesiredResources{}, nil
		}
		manager := runtime()
		if manager == nil {
			return agentSkillDesiredResources{}, nil
		}

		infos, err := registry.List()
		if err != nil {
			return agentSkillDesiredResources{}, fmt.Errorf("daemon: list extensions for agent/skill sync: %w", err)
		}
		slices.SortFunc(infos, func(left, right extensionpkg.ExtensionInfo) int {
			return strings.Compare(left.Name, right.Name)
		})

		desired := agentSkillDesiredResources{}
		globalScope := resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal}
		for _, info := range infos {
			if !info.Enabled {
				continue
			}
			ext, err := loadExtensionSnapshot(registry, manager, logger, info.Name)
			if err != nil {
				return agentSkillDesiredResources{}, fmt.Errorf(
					"daemon: load extension %q for agent/skill sync: %w",
					info.Name,
					err,
				)
			}
			if ext == nil || ext.Manifest == nil || !ext.Status.Registered {
				continue
			}
			appendExtensionAgentResources(&desired, globalScope, ext.Info.Name, ext.StaticAgents)
			appendExtensionSkillResources(&desired, globalScope, ext.Info.Name, ext.Skills)
		}

		return desired, nil
	}
}
