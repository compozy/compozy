package daemon

import (
	"context"

	"fmt"
	"log/slog"
	"slices"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"

	"github.com/compozy/agh/internal/resources"

	skillspkg "github.com/compozy/agh/internal/skills"

	workspacepkg "github.com/compozy/agh/internal/workspace"
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

	agentCodec, err := resources.ResolveCodec[aghconfig.AgentDef](state.resourceCodecs, aghconfig.AgentResourceKind)
	if err != nil {
		return nil, fmt.Errorf("daemon: resolve agent codec: %w", err)
	}
	agentStore, err := resources.NewStore(state.resourceKernel, agentCodec)
	if err != nil {
		return nil, fmt.Errorf("daemon: create agent store: %w", err)
	}
	skillCodec, err := resources.ResolveCodec[skillspkg.SkillResourceSpec](
		state.resourceCodecs,
		skillspkg.SkillResourceKind,
	)
	if err != nil {
		return nil, fmt.Errorf("daemon: resolve skill codec: %w", err)
	}
	skillStore, err := resources.NewStore(state.resourceKernel, skillCodec)
	if err != nil {
		return nil, fmt.Errorf("daemon: create skill store: %w", err)
	}
	mcpCodec, err := resources.ResolveCodec[aghconfig.MCPServer](state.resourceCodecs, aghconfig.MCPServerResourceKind)
	if err != nil {
		return nil, fmt.Errorf("daemon: resolve mcp server codec for agent/skill sync: %w", err)
	}
	mcpStore, err := resources.NewStore(state.resourceKernel, mcpCodec)
	if err != nil {
		return nil, fmt.Errorf("daemon: create mcp server store for agent/skill sync: %w", err)
	}

	return newAgentSkillSourceSyncer(
		state.resourceKernel,
		agentStore,
		agentCodec,
		newAgentProjector(state.agentCatalog),
		skillStore,
		skillCodec,
		newSkillProjector(state.skillsRegistry),
		mcpStore,
		mcpCodec,
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
	), nil
}

func daemonAgentSkillDeclarationProvider(
	homePaths aghconfig.HomePaths,
	registry Registry,
	workspaceResolver workspacepkg.RuntimeResolver,
	skillsRegistry *skillspkg.Registry,
	logger *slog.Logger,
) agentSkillDeclarationProvider {
	return func(ctx context.Context) (agentSkillDesiredResources, error) {
		desired := agentSkillDesiredResources{}
		globalScope := resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal}
		globalAgents, err := aghconfig.LoadWorkspaceAgentDefs("", nil, homePaths)
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
			ext, err := loadExtensionSnapshot(ctx, registry, manager, logger, info.Name)
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
			agents, err := extensionpkg.LoadAgentResources(ext.RootDir, ext.Manifest)
			if err != nil {
				return agentSkillDesiredResources{}, fmt.Errorf(
					"daemon: load extension %q agents for sync: %w",
					ext.Info.Name,
					err,
				)
			}
			appendAgentResources(&desired, globalScope, "extension/"+ext.Info.Name+"/agents", agents)
			appendSkillResources(&desired, globalScope, "extension/"+ext.Info.Name+"/skills", ext.Skills)
		}

		return desired, nil
	}
}
