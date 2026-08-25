package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"sync"
	"time"

	core "github.com/compozy/compozy/internal/api/core"
	attachmentspkg "github.com/compozy/compozy/internal/attachments"
	"github.com/compozy/compozy/internal/cmdpalette"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/deadentity"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/heartbeat"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	looppkg "github.com/compozy/compozy/internal/loop"
	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	"github.com/compozy/compozy/internal/memory"
	"github.com/compozy/compozy/internal/memory/consolidation"
	memcontract "github.com/compozy/compozy/internal/memory/contract"
	localprovider "github.com/compozy/compozy/internal/memory/provider/local"
	"github.com/compozy/compozy/internal/memory/provider/local/memstore"
	"github.com/compozy/compozy/internal/profile"

	"github.com/compozy/compozy/internal/network/participation"

	presetspkg "github.com/compozy/compozy/internal/notifications/presets"

	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/sandbox"

	"github.com/compozy/compozy/internal/session"

	"github.com/compozy/compozy/internal/situation"
	"github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/soul"

	"github.com/compozy/compozy/internal/toolruntime"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/vault"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/workspaceaccess"
	"github.com/compozy/compozy/internal/worktree"
)

const (
	bootNameKey   = "name"
	bootSourceKey = "source"
)

type bootState struct {
	cfg                    compozyconfig.Config
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
	commandService         session.CommandService
	notifier               *hooksNotifier
	registry               Registry
	profiles               *profile.Manager
	deadEntities           *deadentity.Service
	loopTargetHealth       *loopTargetHealthSlot
	processRegistry        *toolruntime.Registry
	sandboxRegistry        *sandbox.Registry
	workspaceResolver      *workspacepkg.Resolver
	worktrees              *worktree.Service
	windowManagerBootState
	sessions              SessionManager
	sessionWakeBridge     *sessionWakeBridge
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
	gateway               gateway.Policy
	gatewayVerifier       *gateway.EndpointVerifier
	networkWakeRunner     *networkWakeRunner
	participationResolver participation.Resolver
	accessPolicy          workspaceaccess.Policy
	accessConsent         *workspaceAccessConsentCache
	toolRegistry          toolspkg.Registry
	mcpToolProvider       daemonMCPToolProvider
	toolArtifacts         toolspkg.ToolArtifactStore
	sessionAttachments    attachmentspkg.Store
	toolsets              core.ToolsetRegistry
	toolApprovals         toolspkg.ApprovalTokenIssuer
	approvalCoordinator   toolspkg.ApprovalCoordinator
	cmdPalette            cmdpalette.Registry
	viewPatches           *extensionCmdPaletteProvider
	clarify               *clarifyBridge
	observer              Observer
	lifecycleObservers    *sessionLifecycleFanout
	hookTelemetrySinks    *hookTelemetryFanout
	hooks                 hookRuntime
	hookDispatcher        *hookspkg.Hooks
	hookBindings          hookBindingPublisher
	resourceKernel        *resources.Kernel
	resourceCodecs        *resources.CodecRegistry
	agentCatalog          *resourceCatalog[compozyconfig.AgentDef]
	roleResolver          *roleResolver
	soulCatalog           *resourceCatalog[soul.ResourceSpec]
	heartbeatCatalog      *resourceCatalog[heartbeat.ResourceSpec]
	toolCatalog           *resourceCatalog[toolspkg.Tool]
	mcpServerCatalog      *resourceCatalog[compozyconfig.MCPServer]
	extensionEnvBindings  extensionpkg.EnvBindingStore
	mcpAuthGeneration     *mcpauth.MutationGeneration
	mcpRuntimeHealth      *mcppkg.RuntimeHealthRegistry
	toolProjectionEpoch   *mcppkg.ProjectionEpoch
	agentProbeConfig      *agentProbeConfigState
	loopCatalog           *resourceCatalog[looppkg.ResourceSpec]
	agentSkillResources   agentSkillPublisher
	toolMCPResources      toolMCPPublisher
	loopResources         loopResourcePublisher
	extensionKitResources extensionKitResourcePublisher
	extMu                 sync.RWMutex
	extensions            extensionRuntime
	resourceReconcile     resources.ReconcileDriver
	automation            automationRuntime
	bridges               *bridgeRuntime
	notificationPresets   *presetspkg.Service
	supportBundles        supportBundleShutdowner
	updateManager         settingsUpdateManager
	backgroundUpdates     *backgroundUpdateRuntime
	httpServer            Server
	udsServer             Server
	skillsCancel          context.CancelFunc
	skillsDone            chan struct{}
	loopsCancel           context.CancelFunc
	loopsDone             chan struct{}
	goalOutboxCancel      context.CancelFunc
	goalOutboxDone        chan struct{}
	effectRelayCancel     context.CancelFunc
	effectRelayDone       chan struct{}
	startedAt             time.Time
	info                  Info
	deps                  RuntimeDeps

	sessionWindowReconciler session.WindowReconciler
}

type daemonMCPToolProvider interface {
	ForgetMCPServer(workspaceID string, serverName string)
	ForgetWorkspace(workspaceID string)
}

func (d *Daemon) boot(ctx context.Context) (err error) {
	if ctx == nil {
		return errors.New("daemon: boot context is required")
	}

	if err := d.beginBoot(); err != nil {
		return err
	}
	defer d.finishBoot(&err)

	state := &bootState{
		mcpAuthGeneration:   mcpauth.NewMutationGeneration(),
		toolProjectionEpoch: mcppkg.NewProjectionEpoch(),
	}
	cleanup := &bootCleanup{}
	defer cleanup.run(ctx, &err)

	if err := d.bootComponents(ctx, state, cleanup); err != nil {
		return err
	}
	if err := d.publishDaemonInfo(state, cleanup); err != nil {
		return err
	}
	if err := d.markRestartReadyIfRequested(state.info); err != nil {
		return err
	}
	if err := d.startBackgroundUpdates(ctx, state, cleanup); err != nil {
		return err
	}

	d.publishBootState(state)
	return nil
}

func (d *Daemon) publishDaemonInfo(state *bootState, cleanup *bootCleanup) error {
	cleanup.add(func(context.Context) error {
		return RemoveInfo(d.homePaths.DaemonInfo)
	})
	if err := WriteInfo(d.homePaths.DaemonInfo, state.info); err != nil {
		return fmt.Errorf("daemon: publish daemon info: %w", err)
	}
	return nil
}

func (d *Daemon) beginBoot() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.shutdown != nil {
		select {
		case <-d.shutdown.done:
			if d.shutdown.result != nil {
				return fmt.Errorf("daemon: previous shutdown incomplete: %w", d.shutdown.result)
			}
			// Preserve the existing reusable composition-root behavior while
			// rearming readiness for the new runtime generation.
			d.shutdown = nil
			d.readyCh = make(chan struct{})
			d.readyClosed = false
		default:
			return errDaemonShutdownInProgress
		}
	}
	if d.booting ||
		d.lock != nil ||
		d.registry != nil ||
		d.sessions != nil ||
		d.modelCatalog != nil ||
		d.marketplace != nil ||
		d.network != nil ||
		d.gateway != nil ||
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
		provider, err := d.bootSkillsPromptProvider(ctx, state)
		if err != nil {
			return err
		}
		appendProviders = append(appendProviders, provider)
	}
	return d.bootHarnessPromptRuntime(state, prependProviders, appendProviders)
}

func (d *Daemon) bootSkillsPromptProvider(
	ctx context.Context,
	state *bootState,
) (session.PromptProvider, error) {
	skillsCfg := d.skillsRegistryConfig(&state.cfg)
	state.skillsRegistry = skills.NewRegistry(
		skillsCfg,
		skills.WithLogger(state.logger),
		skills.WithActivationContextProvider(newSkillActivationContextProvider(state)),
	)
	if err := state.skillsRegistry.LoadAll(ctx); err != nil {
		return nil, fmt.Errorf("daemon: load skills registry: %w", err)
	}
	state.commandService = newSessionCommandService(
		state.skillsRegistry,
		func() session.AgentResolver { return agentCatalogDependency(state.agentCatalog) },
		func() promptSkillsWorkspaceResolver { return state.workspaceResolver },
		bootProfileNameResolver{state: state},
	)
	state.mcpResolver = skills.NewMCPResolver(state.cfg.Skills, state.logger)
	return skills.NewCatalogProvider(state.skillsRegistry), nil
}

func (d *Daemon) bootHarnessPromptRuntime(
	state *bootState,
	prependProviders []session.PromptProvider,
	appendProviders []session.PromptProvider,
) error {
	state.situationContext = d.buildSituationContext(state)
	state.harnessResolver = NewHarnessContextResolver(HarnessRuntimeSignals{
		RuntimeIdentityPromptSectionEnabled: true,
		SituationPromptSectionEnabled:       state.situationContext != nil,
		MemoryPromptSectionEnabled:          state.memoryStore != nil,
		SkillsPromptSectionEnabled:          state.skillsRegistry != nil,
		ToolsPromptSectionEnabled:           state.cfg.Tools.Enabled,
		WorkspaceKnowledgeAugmenter:         true,
		SkillsAugmenter:                     state.skillsRegistry != nil,
		SituationAugmenter:                  state.situationContext != nil,
		DurableMemoryAugmenter:              state.memoryStore != nil,
		SyntheticTurnsEnabled:               true,
		DetachedTaskRuntimeEnabled:          true,
	},
		WithHarnessSkillInjectionHome(d.homePaths),
		WithHarnessSkillInjectionLogger(state.logger),
	)
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
	state.startupOverlay = compozyRuntimePromptOverlay{}
	skillsCatalogAugmenter := newSkillsCatalogAugmenterState(
		state.skillsRegistry,
		func() session.AgentResolver {
			return agentCatalogDependency(state.agentCatalog)
		},
		func() promptSkillsWorkspaceResolver {
			return state.workspaceResolver
		},
		bootProfileNameResolver{state: state},
	)
	var skillsCatalog session.PromptInputAugmenter
	if skillsCatalogAugmenter != nil {
		skillsCatalog = skillsCatalogAugmenter.Augment
	}
	promptAugmenterDescriptors := defaultPromptInputAugmenterDescriptors(
		situation.WorkspaceKnowledgeAugmenter,
		memory.NewProfileRecallAugmenter(state.memoryStore, d.memoryRecallStoreResolver(state)),
		skillsCatalog,
		state.situationContext.Augment,
	)
	if skillsCatalogAugmenter != nil {
		for index := range promptAugmenterDescriptors {
			if promptAugmenterDescriptors[index].Name == HarnessAugmenterSkills {
				promptAugmenterDescriptors[index].PolicyAugmenter = skillsCatalogAugmenter.AugmentWithPolicy
			}
		}
	}
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
	if err := d.configureMemoryStore(ctx, state); err != nil {
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
