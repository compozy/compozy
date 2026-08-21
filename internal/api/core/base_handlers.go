package core

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/internal/cmdpalette"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/doctor"
	"github.com/compozy/compozy/internal/memory"
	authproviders "github.com/compozy/compozy/internal/providers"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/workspaceaccess"
	"github.com/gin-gonic/gin"
)

// TaskActorContextResolver derives the trusted task-domain actor envelope for one API request.
type TaskActorContextResolver func(c *gin.Context, action string) (taskpkg.ActorContext, error)

// BaseHandlerConfig configures a shared handler set for one transport.
type BaseHandlerConfig struct {
	TransportName                string
	MaskInternalErrors           bool
	IncludeSessionWorkspaceInSSE bool
	Sessions                     SessionManager
	SessionAcceptance            SessionAcceptanceManager
	DrainController              DaemonDrainController
	SessionCatalog               SessionCatalog
	Network                      NetworkService
	NetworkStore                 NetworkStore
	NetworkUsage                 store.NetworkUsageStore
	Coordination                 workspacepkg.CoordinationCommands
	Observer                     Observer
	SchemaStreams                SchemaStreamStatusReader
	Resources                    ResourceService
	Extensions                   ExtensionService
	Tools                        ToolRegistry
	ToolArtifacts                toolspkg.ToolArtifactStore
	SessionAttachments           SessionAttachmentStore
	Toolsets                     ToolsetRegistry
	ToolApprovals                ToolApprovalIssuer
	ApprovalCoordinator          toolspkg.ApprovalCoordinator
	ApprovalGrants               ToolApprovalGrantService
	CmdPalette                   cmdpalette.Registry
	Clarify                      toolspkg.ClarifyBroker
	Automation                   AutomationManager
	Loops                        LoopService
	Tasks                        TaskService
	Bridges                      BridgeService
	Notifications                NotificationPresetService
	Profiles                     ProfileService
	SupportBundles               SupportBundleService
	Settings                     SettingsService
	SettingsRestart              SettingsRestartController
	SettingsUpdate               SettingsUpdateController
	Vault                        VaultService
	Workspaces                   WorkspaceService
	Worktrees                    WorktreeService
	WorkspaceAccess              workspaceaccess.Policy
	WindowManager                WindowManagerService
	Onboarding                   OnboardingStore
	AgentCatalog                 AgentCatalog
	AgentDefinitionSync          AgentDefinitionSync
	ModelCatalog                 ModelCatalogService
	MarketplaceCatalog           MarketplaceCatalogService
	ProviderAuthRunner           authproviders.ProviderAuthCommandRunner
	ProviderAuthCommandResolver  authproviders.ProviderAuthCommandResolver
	AgentContextService          AgentContextService
	SoulAuthoring                SoulAuthoringService
	SoulHistoryPurger            SoulHistoryPurger
	SoulRefresher                SoulRefresher
	HeartbeatAuthoring           HeartbeatAuthoringService
	HeartbeatHistoryPurger       HeartbeatHistoryPurger
	HeartbeatStatus              HeartbeatStatusService
	HeartbeatWake                HeartbeatWakeService
	SessionHealth                SessionHealthReader
	HeartbeatWakeEvents          HeartbeatWakeEventReader
	CoordinatorRole              CoordinatorRoleResolver
	Roles                        RolesStatusProvider
	SkillsRegistry               SkillsRegistry
	SkillResources               SkillResourceSyncer
	SkillMarketplace             SkillMarketplaceService
	InstalledSkillMarketplace    InstalledSkillMarketplaceService
	TaskActorContextResolver     TaskActorContextResolver
	MemoryStore                  *memory.Store
	DreamTrigger                 DreamTrigger
	MemoryExtractor              MemoryExtractorService
	MemoryProviders              MemoryProviderService
	MemorySessionLedger          MemorySessionLedgerService
	RuntimeMemory                doctor.RuntimeMemorySnapshotSource
	DeadEntities                 doctor.DeadEntitySource
	Gateway                      GatewayService
	GatewayPairingSource         string
	HomePaths                    compozyconfig.HomePaths
	Config                       compozyconfig.Config
	Logger                       *slog.Logger
	StartedAt                    time.Time
	Now                          func() time.Time
	PollInterval                 time.Duration
	AgentLoader                  AgentLoader
	StreamDone                   <-chan struct{}
	HTTPPort                     int
	PID                          func() int
}

// BaseHandlers contains the shared transport-independent API handler logic.
type BaseHandlers struct {
	TransportName                string
	MaskInternalErrors           bool
	IncludeSessionWorkspaceInSSE bool
	Sessions                     SessionManager
	SessionAcceptance            SessionAcceptanceManager
	DrainController              DaemonDrainController
	SessionCatalog               SessionCatalog
	Network                      NetworkService
	NetworkStore                 NetworkStore
	NetworkUsage                 store.NetworkUsageStore
	Coordination                 workspacepkg.CoordinationCommands
	Observer                     Observer
	SchemaStreams                SchemaStreamStatusReader
	Resources                    ResourceService
	Extensions                   ExtensionService
	Tools                        ToolRegistry
	ToolArtifacts                toolspkg.ToolArtifactStore
	SessionAttachments           SessionAttachmentStore
	Toolsets                     ToolsetRegistry
	ToolApprovals                ToolApprovalIssuer
	ApprovalCoordinator          toolspkg.ApprovalCoordinator
	ApprovalGrants               ToolApprovalGrantService
	CmdPalette                   cmdpalette.Registry
	Clarify                      toolspkg.ClarifyBroker
	Automation                   AutomationManager
	Loops                        LoopService
	Tasks                        TaskService
	Bridges                      BridgeService
	Notifications                NotificationPresetService
	Profiles                     ProfileService
	SupportBundles               SupportBundleService
	Settings                     SettingsService
	SettingsRestart              SettingsRestartController
	SettingsUpdate               SettingsUpdateController
	Vault                        VaultService
	Workspaces                   WorkspaceService
	Worktrees                    WorktreeService
	WorkspaceAccess              workspaceaccess.Policy
	WindowManager                WindowManagerService
	Onboarding                   OnboardingStore
	AgentCatalog                 AgentCatalog
	AgentDefinitionSync          AgentDefinitionSync
	ModelCatalog                 ModelCatalogService
	MarketplaceCatalog           MarketplaceCatalogService
	ProviderAuthRunner           authproviders.ProviderAuthCommandRunner
	ProviderAuthCommandResolver  authproviders.ProviderAuthCommandResolver
	AgentContextService          AgentContextService
	SoulAuthoring                SoulAuthoringService
	SoulHistoryPurger            SoulHistoryPurger
	SoulRefresher                SoulRefresher
	HeartbeatAuthoring           HeartbeatAuthoringService
	HeartbeatHistoryPurger       HeartbeatHistoryPurger
	HeartbeatStatus              HeartbeatStatusService
	HeartbeatWake                HeartbeatWakeService
	SessionHealth                SessionHealthReader
	HeartbeatWakeEvents          HeartbeatWakeEventReader
	CoordinatorRole              CoordinatorRoleResolver
	Roles                        RolesStatusProvider
	SkillsRegistry               SkillsRegistry
	SkillResources               SkillResourceSyncer
	SkillMarketplace             SkillMarketplaceService
	InstalledSkillMarketplace    InstalledSkillMarketplaceService
	TaskActorContextResolver     TaskActorContextResolver
	MemoryStore                  *memory.Store
	DreamTrigger                 DreamTrigger
	MemoryExtractor              MemoryExtractorService
	MemoryProviders              MemoryProviderService
	MemorySessionLedger          MemorySessionLedgerService
	RuntimeMemory                doctor.RuntimeMemorySnapshotSource
	DeadEntities                 doctor.DeadEntitySource
	Gateway                      GatewayService
	GatewayPairingSource         string
	HomePaths                    compozyconfig.HomePaths
	Config                       compozyconfig.Config
	Logger                       *slog.Logger
	StartedAt                    time.Time
	Now                          func() time.Time
	PollInterval                 time.Duration
	AgentLoader                  AgentLoader
	PID                          func() int

	settingsMu           sync.RWMutex
	streamDone           <-chan struct{}
	httpPort             atomic.Int64
	activeSessionStreams atomic.Int64
	windowManagerStreams *windowManagerStreamLifecycle
}

// NewBaseHandlers builds a shared handler set with transport-specific defaults applied.
func NewBaseHandlers(cfg *BaseHandlerConfig) *BaseHandlers {
	if cfg == nil {
		cfg = &BaseHandlerConfig{}
	}
	defaults := normalizeBaseHandlerConfig(cfg)
	handlers := baseHandlersFromConfig(cfg, defaults)
	handlers.applyAuthoredContextConfig(cfg)
	handlers.streamDone = cfg.StreamDone
	handlers.windowManagerStreams = newWindowManagerStreamLifecycle()
	handlers.httpPort.Store(int64(cfg.HTTPPort))
	return handlers
}

func baseHandlersFromConfig(cfg *BaseHandlerConfig, defaults baseHandlerDefaults) *BaseHandlers {
	return &BaseHandlers{
		TransportName:                strings.TrimSpace(cfg.TransportName),
		MaskInternalErrors:           cfg.MaskInternalErrors,
		IncludeSessionWorkspaceInSSE: cfg.IncludeSessionWorkspaceInSSE,
		Sessions:                     cfg.Sessions,
		SessionAcceptance:            cfg.SessionAcceptance,
		DrainController:              cfg.DrainController,
		SessionCatalog:               cfg.SessionCatalog,
		Network:                      cfg.Network,
		NetworkStore:                 cfg.NetworkStore,
		NetworkUsage:                 cfg.NetworkUsage,
		Coordination:                 cfg.Coordination,
		Observer:                     cfg.Observer,
		SchemaStreams:                cfg.SchemaStreams,
		Resources:                    cfg.Resources,
		Extensions:                   cfg.Extensions,
		Tools:                        cfg.Tools,
		ToolArtifacts:                cfg.ToolArtifacts,
		SessionAttachments:           cfg.SessionAttachments,
		Toolsets:                     cfg.Toolsets,
		ToolApprovals:                cfg.ToolApprovals,
		ApprovalCoordinator:          cfg.ApprovalCoordinator,
		ApprovalGrants:               cfg.ApprovalGrants,
		CmdPalette:                   cfg.CmdPalette,
		Clarify:                      cfg.Clarify,
		Automation:                   cfg.Automation,
		Loops:                        cfg.Loops,
		Tasks:                        cfg.Tasks,
		Bridges:                      cfg.Bridges,
		Notifications:                cfg.Notifications,
		Profiles:                     cfg.Profiles,
		SupportBundles:               cfg.SupportBundles,
		Settings:                     cfg.Settings,
		SettingsRestart:              cfg.SettingsRestart,
		SettingsUpdate:               cfg.SettingsUpdate,
		Vault:                        cfg.Vault,
		Workspaces:                   cfg.Workspaces,
		Worktrees:                    cfg.Worktrees,
		WorkspaceAccess:              cfg.WorkspaceAccess,
		WindowManager:                cfg.WindowManager,
		Onboarding:                   cfg.Onboarding,
		AgentCatalog:                 cfg.AgentCatalog,
		AgentDefinitionSync:          cfg.AgentDefinitionSync,
		ModelCatalog:                 cfg.ModelCatalog,
		MarketplaceCatalog:           cfg.MarketplaceCatalog,
		ProviderAuthRunner:           defaults.providerAuthRunner,
		ProviderAuthCommandResolver:  defaults.providerAuthCommandResolver,
		AgentContextService:          cfg.AgentContextService,
		SoulHistoryPurger:            cfg.SoulHistoryPurger,
		HeartbeatHistoryPurger:       cfg.HeartbeatHistoryPurger,
		CoordinatorRole:              cfg.CoordinatorRole,
		Roles:                        cfg.Roles,
		SkillsRegistry:               cfg.SkillsRegistry,
		SkillResources:               cfg.SkillResources,
		SkillMarketplace:             cfg.SkillMarketplace,
		InstalledSkillMarketplace:    cfg.InstalledSkillMarketplace,
		TaskActorContextResolver:     cfg.TaskActorContextResolver,
		MemoryStore:                  cfg.MemoryStore,
		DreamTrigger:                 cfg.DreamTrigger,
		MemoryExtractor:              cfg.MemoryExtractor,
		MemoryProviders:              cfg.MemoryProviders,
		MemorySessionLedger:          cfg.MemorySessionLedger,
		RuntimeMemory:                cfg.RuntimeMemory,
		DeadEntities:                 cfg.DeadEntities,
		Gateway:                      cfg.Gateway,
		GatewayPairingSource:         strings.TrimSpace(cfg.GatewayPairingSource),
		HomePaths:                    cfg.HomePaths,
		Config:                       cfg.Config,
		Logger:                       defaults.logger,
		StartedAt:                    cfg.StartedAt,
		Now:                          defaults.now,
		PollInterval:                 cfg.PollInterval,
		AgentLoader:                  defaults.agentLoader,
		PID:                          defaults.pid,
	}
}

type baseHandlerDefaults struct {
	logger                      *slog.Logger
	now                         func() time.Time
	agentLoader                 AgentLoader
	pid                         func() int
	providerAuthRunner          authproviders.ProviderAuthCommandRunner
	providerAuthCommandResolver authproviders.ProviderAuthCommandResolver
}

func normalizeBaseHandlerConfig(cfg *BaseHandlerConfig) baseHandlerDefaults {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	now := cfg.Now
	if now == nil {
		now = func() time.Time {
			return time.Now().UTC()
		}
	}
	agentLoader := cfg.AgentLoader
	if agentLoader == nil {
		agentLoader = compozyconfig.LoadAgentDef
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaultPollInterval
	}
	if cfg.StartedAt.IsZero() {
		cfg.StartedAt = now()
	}
	pid := cfg.PID
	if pid == nil {
		pid = os.Getpid
	}
	providerAuthRunner := cfg.ProviderAuthRunner
	if providerAuthRunner == nil {
		providerAuthRunner = authproviders.DefaultProviderAuthCommandRunner
	}
	providerAuthCommandResolver := cfg.ProviderAuthCommandResolver
	if providerAuthCommandResolver == nil {
		providerAuthCommandResolver = authproviders.DefaultProviderAuthCommandResolver
	}
	if cfg.StreamDone == nil {
		cfg.StreamDone = make(chan struct{})
	}
	return baseHandlerDefaults{
		logger:                      logger,
		now:                         now,
		agentLoader:                 agentLoader,
		pid:                         pid,
		providerAuthRunner:          providerAuthRunner,
		providerAuthCommandResolver: providerAuthCommandResolver,
	}
}

func (h *BaseHandlers) applyAuthoredContextConfig(cfg *BaseHandlerConfig) {
	if h == nil || cfg == nil {
		return
	}
	h.SoulAuthoring = cfg.SoulAuthoring
	h.SoulRefresher = cfg.SoulRefresher
	h.HeartbeatAuthoring = cfg.HeartbeatAuthoring
	h.HeartbeatStatus = cfg.HeartbeatStatus
	h.HeartbeatWake = cfg.HeartbeatWake
	h.SessionHealth = cfg.SessionHealth
	h.HeartbeatWakeEvents = cfg.HeartbeatWakeEvents
}

// SetStreamDone updates the transport shutdown bridge used by streaming handlers.
func (h *BaseHandlers) SetStreamDone(done <-chan struct{}) {
	if h == nil {
		return
	}
	if done == nil {
		if h.Logger != nil {
			h.Logger.Warn(
				"api: stream shutdown bridge cleared; streaming handlers will rely on caller context " +
					"until a transport installs one",
			)
		}
		done = make(chan struct{})
	}
	h.settingsMu.Lock()
	h.streamDone = done
	h.settingsMu.Unlock()
	if h.windowManagerStreams != nil {
		h.windowManagerStreams.reset()
	}
}

// ShutdownWindowManagerStreams stops upgrades and waits for every window-manager stream.
func (h *BaseHandlers) ShutdownWindowManagerStreams(ctx context.Context) error {
	if h == nil || h.windowManagerStreams == nil {
		return nil
	}
	return h.windowManagerStreams.shutdown(ctx)
}
