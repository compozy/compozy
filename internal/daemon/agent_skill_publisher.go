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

	syncer := newAgentSkillSourceSyncer(agentSkillSourceSyncerDeps{
		raw:            state.resourceKernel,
		agentStore:     resolved.agentStore,
		agentCodec:     resolved.agentCodec,
		agentProjector: newAgentProjector(state.agentCatalog),
		soulStore:      resolved.soulStore,
		soulCodec:      resolved.soulCodec,
		heartbeatStore: resolved.heartbeatStore,
		heartbeatCodec: resolved.heartbeatCodec,
		skillStore:     resolved.skillStore,
		skillCodec:     resolved.skillCodec,
		skillProjector: newSkillProjector(state.skillsRegistry),
		mcpStore:       resolved.mcpStore,
		mcpCodec:       resolved.mcpCodec,
		actor:          agentSkillSyncActor(),
		logger:         state.logger,
		trigger: func(ctx context.Context, kind resources.ResourceKind, reason resources.ReconcileReason) error {
			if state.resourceReconcile == nil {
				return nil
			}
			return state.resourceReconcile.Trigger(ctx, kind, reason)
		},
		providers: []agentSkillDeclarationProvider{
			daemonAgentSkillDeclarationProvider(
				d.homePaths,
				state.registry,
				state.workspaceResolver,
				state.skillsRegistry,
				state.logger,
			),
			extensionAgentSkillDeclarationProvider(registry, state.currentExtensionRuntime, state.logger),
		},
	})
	if syncer == nil {
		return nil, errors.New("daemon: create agent/skill source syncer")
	}
	return syncer, nil
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
	resolved.agentCodec, resolved.agentStore, err = resolveDaemonResourceStore[compozyconfig.AgentDef](
		state,
		compozyconfig.AgentResourceKind,
		"agent",
	)
	if err != nil {
		return agentSkillPublisherResources{}, err
	}
	resolved.skillCodec, resolved.skillStore, err = resolveDaemonResourceStore[skillspkg.SkillResourceSpec](
		state,
		skillspkg.SkillResourceKind,
		"skill",
	)
	if err != nil {
		return agentSkillPublisherResources{}, err
	}
	resolved.mcpCodec, resolved.mcpStore, err = resolveDaemonResourceStore[compozyconfig.MCPServer](
		state,
		compozyconfig.MCPServerResourceKind,
		"mcp server",
	)
	if err != nil {
		return agentSkillPublisherResources{}, err
	}
	resolved.soulCodec, resolved.soulStore, err = resolveDaemonResourceStore[soul.ResourceSpec](
		state,
		soul.ResourceKind,
		"extension soul",
	)
	if err != nil {
		return agentSkillPublisherResources{}, err
	}
	resolved.heartbeatCodec, resolved.heartbeatStore, err = resolveDaemonResourceStore[heartbeat.ResourceSpec](
		state,
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
	return func(ctx context.Context) (agentSkillDeclarations, error) {
		desired := agentSkillDeclarations{}
		globalScope := resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal}
		globalAgents, err := compozyconfig.LoadWorkspaceAgentDefs("", nil, homePaths)
		if err != nil {
			return agentSkillDeclarations{}, fmt.Errorf("daemon: discover global agents: %w", err)
		}
		appendAgentResources(&desired, globalScope, "config/global", globalAgents)

		if skillsRegistry != nil {
			globalSkills, _, err := skillsRegistry.DiscoverGlobal(ctx)
			if err != nil {
				return agentSkillDeclarations{}, fmt.Errorf("daemon: discover global skills: %w", err)
			}
			appendSkillResources(&desired, globalScope, skillPublicationSource{prefix: "skills/global"}, globalSkills)
		}

		workspaces, err := registeredWorkspaces(ctx, registry, workspaceResolver, logger)
		if err != nil {
			return agentSkillDeclarations{}, err
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
				return agentSkillDeclarations{}, fmt.Errorf(
					"daemon: discover workspace %q skills: %w",
					scope.ID,
					err,
				)
			}
			appendSkillResources(
				&desired,
				scope,
				skillPublicationSource{prefix: "skills/workspace/" + scope.ID},
				workspaceSkills,
			)
		}

		return desired, nil
	}
}

func extensionAgentSkillDeclarationProvider(
	registry *extensionpkg.Registry,
	runtime func() extensionRuntime,
	logger *slog.Logger,
) agentSkillDeclarationProvider {
	return func(ctx context.Context) (agentSkillDeclarations, error) {
		if err := ctx.Err(); err != nil {
			return agentSkillDeclarations{}, err
		}
		if registry == nil || runtime == nil {
			return agentSkillDeclarations{}, nil
		}
		manager := runtime()
		if manager == nil {
			return agentSkillDeclarations{}, nil
		}

		infos, err := registry.List()
		if err != nil {
			return agentSkillDeclarations{}, fmt.Errorf("daemon: list extensions for agent/skill sync: %w", err)
		}
		slices.SortFunc(infos, func(left, right extensionpkg.ExtensionInfo) int {
			return strings.Compare(left.Name, right.Name)
		})

		desired := agentSkillDeclarations{}
		globalScope := resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal}
		for _, info := range infos {
			if !info.Enabled {
				continue
			}
			ext, err := loadExtensionSnapshot(registry, manager, logger, info.Name)
			if err != nil {
				return agentSkillDeclarations{}, fmt.Errorf(
					"daemon: load extension %q for agent/skill sync: %w",
					info.Name,
					err,
				)
			}
			if ext == nil || ext.Manifest == nil || !ext.Status.Registered {
				continue
			}
			agents, err := extensionpkg.LoadAgentResources(ext.RootDir, ext.Manifest.Resources.Agents)
			if err != nil {
				return agentSkillDeclarations{}, fmt.Errorf(
					"daemon: load extension %q agents for sync: %w",
					ext.Info.Name,
					err,
				)
			}
			appendExtensionAgentResources(&desired, globalScope, ext.Info.Name, agents)
			appendSkillResources(&desired, globalScope, skillPublicationSource{
				prefix: "extension/" + strings.TrimSpace(ext.Info.Name) + "/skills",
				owner:  extensionOwner(ext.Info.Name),
			}, ext.Skills)
		}

		return desired, nil
	}
}
