package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"sync"
	"time"

	core "github.com/compozy/agh/internal/api/core"

	bundlepkg "github.com/compozy/agh/internal/bundles"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/deadentity"
	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/compozy/agh/internal/heartbeat"
	hookspkg "github.com/compozy/agh/internal/hooks"
	looppkg "github.com/compozy/agh/internal/loop"
	marketplacepkg "github.com/compozy/agh/internal/marketplace"
	mcppkg "github.com/compozy/agh/internal/mcp"
	mcpauth "github.com/compozy/agh/internal/mcp/auth"
	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/memory/consolidation"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	localprovider "github.com/compozy/agh/internal/memory/provider/local"
	"github.com/compozy/agh/internal/memory/provider/local/memstore"

	"github.com/compozy/agh/internal/network/participation"

	presetspkg "github.com/compozy/agh/internal/notifications/presets"

	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/sandbox"

	"github.com/compozy/agh/internal/session"

	"github.com/compozy/agh/internal/situation"
	"github.com/compozy/agh/internal/skills"
	"github.com/compozy/agh/internal/soul"

	"github.com/compozy/agh/internal/toolruntime"
	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/compozy/agh/internal/vault"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

const (
	bootNameKey = "name"
)

type bootState struct {
	cfg                    aghconfig.Config
	logger                 *slog.Logger
	closeLogger            func() error
	lock                   *Lock
	harnessResolver        *HarnessContextResolver
	harnessRecorder        *harnessLifecycleRecorder
	memoryStore            *memory.Store
	localMemoryProvider    *localprovider.Provider
	memoryProviderRegistry *extensionpkg.MemoryProviderRegistry
	memoryExtractor        *daemonMemoryExtractor
	runtimeWorkers         daemonRuntimeWorkers
	checkpointRuntime      *checkpointSummaryRuntime
	ledgerMaterializer     session.LedgerMaterializer
	skillsRegistry         *skills.Registry
	mcpResolver            *skills.MCPResolver
	dreamSvc               consolidation.Service
	dreamRuntime           *consolidation.Runtime
	globalMemoryDir        string
	situationContext       *situation.Service
	promptAssembler        session.PromptAssembler
	startupOverlay         session.StartupPromptOverlay
	promptAugmenter        session.PromptInputAugmenter
	notifier               *hooksNotifier
	registry               Registry
	deadEntities           *deadentity.Service
	processRegistry        *toolruntime.Registry
	sandboxRegistry        *sandbox.Registry
	workspaceResolver      *workspacepkg.Resolver
	windowManagerBootState
	sessions              SessionManager
	hostedMCP             *mcppkg.HostedService
	providerVault         *vault.Service
	modelCatalog          *modelCatalogRuntime
	marketplace           *marketplaceRuntime
	marketplaceNotifier   marketplacepkg.Notifier
	tasks                 *taskRuntime
	subprocessHealth      *subprocessHealthEscalator
	reviewRequests        *runReviewRequestedForwarder
	spawnReaper           *spawnReaper
	scheduler             *schedulerRuntime
	coordinator           *coordinatorRuntime
	network               networkRuntime
	networkWakeRunner     *networkWakeRunner
	participationResolver participation.Resolver
	toolRegistry          toolspkg.Registry
	toolArtifacts         toolspkg.ToolArtifactStore
	toolsets              core.ToolsetRegistry
	toolApprovals         toolspkg.ApprovalTokenIssuer
	clarify               *clarifyBridge
	observer              Observer
	lifecycleObservers    *sessionLifecycleFanout
	hookTelemetrySinks    *hookTelemetryFanout
	hooks                 hookRuntime
	hookDispatcher        *hookspkg.Hooks
	hookBindings          hookBindingPublisher
	resourceKernel        *resources.Kernel
	resourceCodecs        *resources.CodecRegistry
	agentCatalog          *resourceCatalog[aghconfig.AgentDef]
	roleResolver          *roleResolver
	soulCatalog           *resourceCatalog[soul.ResourceSpec]
	heartbeatCatalog      *resourceCatalog[heartbeat.ResourceSpec]
	toolCatalog           *resourceCatalog[toolspkg.Tool]
	mcpServerCatalog      *resourceCatalog[aghconfig.MCPServer]
	mcpAuthGeneration     *mcpauth.MutationGeneration
	loopCatalog           *resourceCatalog[looppkg.ResourceSpec]
	agentSkillResources   agentSkillPublisher
	toolMCPResources      toolMCPPublisher
	bundleResources       bundleResourcePublisher
	loopResources         loopResourcePublisher
	extMu                 sync.RWMutex
	extensions            extensionRuntime
	resourceReconcile     resources.ReconcileDriver
	automation            automationRuntime
	bridges               *bridgeRuntime
	notificationPresets   *presetspkg.Service
	bundles               *bundlepkg.Service
	httpServer            Server
	udsServer             Server
	skillsCancel          context.CancelFunc
	skillsDone            chan struct{}
	loopsCancel           context.CancelFunc
	loopsDone             chan struct{}
	goalOutboxCancel      context.CancelFunc
	goalOutboxDone        chan struct{}
	startedAt             time.Time
	info                  Info
	deps                  RuntimeDeps
}

func (d *Daemon) boot(ctx context.Context) (err error) {
	if ctx == nil {
		return errors.New("daemon: boot context is required")
	}

	if err := d.beginBoot(); err != nil {
		return err
	}
	defer d.finishBoot(&err)

	state := &bootState{mcpAuthGeneration: mcpauth.NewMutationGeneration()}
	cleanup := &bootCleanup{}
	defer cleanup.run(ctx, &err)

	if err := d.bootComponents(ctx, state, cleanup); err != nil {
		return err
	}
	if err := d.markRestartReadyIfRequested(state.info); err != nil {
		return err
	}

	d.publishBootState(state)
	return nil
}

func (d *Daemon) beginBoot() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.booting ||
		d.lock != nil ||
		d.registry != nil ||
		d.sessions != nil ||
		d.modelCatalog != nil ||
		d.marketplace != nil ||
		d.network != nil ||
		d.toolRegistry != nil ||
		d.observer != nil ||
		d.resourceReconcile != nil ||
		d.automation != nil ||
		d.bridges != nil {
		return errors.New("daemon: already booted")
	}
	d.admission.Undrain()
	d.booting = true
	return nil
}

func (d *Daemon) bootPromptProviders(ctx context.Context, state *bootState) error {
	var prependProviders []session.PromptProvider
	var appendProviders []session.PromptProvider

	if state.cfg.Memory.Enabled {
		provider, err := d.bootMemoryPromptProvider(ctx, state)
		if err != nil {
			return err
		}
		prependProviders = append(prependProviders, provider)
	}

	if state.cfg.Skills.Enabled {
		skillsCfg := d.skillsRegistryConfig(&state.cfg)
		state.skillsRegistry = skills.NewRegistry(
			skillsCfg,
			skills.WithLogger(state.logger),
			skills.WithActivationContextProvider(newSkillActivationContextProvider(state)),
		)
		state.mcpResolver = skills.NewMCPResolver(state.cfg.Skills, state.logger)
		appendProviders = append(appendProviders, skills.NewCatalogProvider(state.skillsRegistry))
	}

	state.situationContext = d.buildSituationContext(state)
	state.harnessResolver = NewHarnessContextResolver(HarnessRuntimeSignals{
		RuntimeIdentityPromptSectionEnabled: true,
		SituationPromptSectionEnabled:       state.situationContext != nil,
		MemoryPromptSectionEnabled:          state.memoryStore != nil,
		SkillsPromptSectionEnabled:          state.skillsRegistry != nil,
		ToolsPromptSectionEnabled:           state.cfg.Tools.Enabled,
		SkillsAugmenter:                     state.skillsRegistry != nil,
		SituationAugmenter:                  state.situationContext != nil,
		DurableMemoryAugmenter:              state.memoryStore != nil,
		SyntheticTurnsEnabled:               true,
		DetachedTaskRuntimeEnabled:          true,
	})
	state.harnessRecorder = newHarnessLifecycleRecorder(state.logger, d.now)
	state.promptAssembler = NewComposedAssembler(
		WithSectionSelector(NewSectionSelector(state.harnessResolver, state.harnessRecorder)),
		WithPromptSectionDescriptors(
			defaultStartupPromptSectionDescriptorsFromProviders(
				prependProviders,
				appendProviders,
				state.situationContext,
				0,
			)...,
		),
	)
	state.startupOverlay = aghRuntimePromptOverlay{}
	promptAugmenterDescriptors := defaultPromptInputAugmenterDescriptors(
		memory.NewRecallAugmenter(state.memoryStore),
		newSkillsCatalogAugmenter(state.skillsRegistry, func() promptSkillsWorkspaceResolver {
			return state.workspaceResolver
		}),
		state.situationContext.Augment,
	)
	promptAugmenter, err := newPromptInputCompositeAugmenter(
		state.logger,
		state.harnessResolver,
		state.harnessRecorder,
		promptAugmenterDescriptors...,
	)
	if err != nil {
		return fmt.Errorf("daemon: build prompt input composite: %w", err)
	}
	state.promptAugmenter = promptAugmenter
	return nil
}

func (d *Daemon) bootMemoryPromptProvider(
	ctx context.Context,
	state *bootState,
) (session.PromptProvider, error) {
	if err := d.configureMemoryStore(state); err != nil {
		return nil, err
	}
	state.localMemoryProvider = localprovider.New(
		memstore.New(state.memoryStore),
		localprovider.WithLogger(state.logger),
		localprovider.WithClock(d.now),
	)
	providerCtx, cancel := d.memoryProviderInitContext(ctx, state)
	if cancel != nil {
		defer cancel()
	}
	if err := state.localMemoryProvider.Initialize(providerCtx, memcontract.ProviderInit{
		Logger: state.logger,
		Config: map[string]any{
			bootNameKey: localprovider.Name,
		},
	}); err != nil {
		return nil, fmt.Errorf("daemon: initialize local memory provider: %w", err)
	}
	return memory.NewAssembler(
		state.memoryStore,
		memory.WithSnapshotProvider(state.localMemoryProvider),
	), nil
}

func (d *Daemon) memoryProviderInitContext(
	ctx context.Context,
	state *bootState,
) (context.Context, context.CancelFunc) {
	if state.cfg.Memory.Provider.Timeout <= 0 {
		return ctx, nil
	}
	return context.WithTimeout(ctx, state.cfg.Memory.Provider.Timeout)
}

func (d *Daemon) buildSituationContext(state *bootState) *situation.Service {
	return situation.NewService(situation.Deps{
		Now: d.now,
		WorkspaceResolverFunc: func() situation.WorkspaceResolver {
			return state.workspaceResolver
		},
		AgentResolverFunc: func() situation.AgentResolver {
			return agentCatalogDependency(state.agentCatalog, agentSidecarCatalogs{
				soul:      state.soulCatalog,
				heartbeat: state.heartbeatCatalog,
			})
		},
		SkillRegistryFunc: func() situation.SkillRegistry {
			if state.skillsRegistry == nil {
				return nil
			}
			return state.skillsRegistry
		},
		TaskStoreFunc: func() situation.TaskStore {
			if state.tasks == nil {
				return nil
			}
			return state.tasks.store
		},
		NetworkFunc: func() situation.NetworkReader {
			return state.network
		},
		CoordinatorRoleFunc: func() situation.CoordinatorRoleResolver {
			return state.deps.CoordinatorRole
		},
		SoulSnapshotsFunc: func() situation.SoulSnapshotStore {
			return soulSnapshotStoreDependency(state.registry)
		},
	})
}
