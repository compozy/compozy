package udsapi

import (
	"log/slog"
	"time"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/cmdpalette"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/doctor"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	"github.com/compozy/compozy/internal/memory"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

type handlerConfig struct {
	sessions            core.SessionManager
	drainController     core.DaemonDrainController
	sessionCatalog      core.SessionCatalog
	tasks               core.TaskService
	network             core.NetworkService
	networkStore        core.NetworkStore
	networkUsage        store.NetworkUsageStore
	coordination        workspacepkg.CoordinationCommands
	observer            core.Observer
	schemaStreams       core.SchemaStreamStatusReader
	resources           core.ResourceService
	windowManager       core.WindowManagerProvider
	terminal            core.TerminalProvider
	automation          core.AutomationManager
	loops               core.LoopService
	bridges             core.BridgeService
	notifications       core.NotificationPresetService
	profiles            core.ProfileService
	supportBundles      core.SupportBundleService
	tools               core.ToolRegistry
	toolArtifacts       toolspkg.ToolArtifactStore
	sessionAttachments  core.SessionAttachmentStore
	toolsets            core.ToolsetRegistry
	toolApprovals       core.ToolApprovalIssuer
	approvalGrants      core.ToolApprovalGrantService
	approvalCoordinator toolspkg.ApprovalCoordinator
	cmdPalette          cmdpalette.Registry
	clarify             toolspkg.ClarifyBroker
	settings            core.SettingsService
	settingsRestart     core.SettingsRestartController
	settingsUpdate      core.SettingsUpdateController
	vault               core.VaultService
	workspaces          core.WorkspaceService
	worktrees           core.WorktreeService
	workspaceAccess     workspaceaccess.Policy
	onboarding          core.OnboardingStore
	agentCatalog        core.AgentCatalog
	agentSync           core.AgentDefinitionSync
	modelCatalog        core.ModelCatalogService
	marketplaceCatalog  core.MarketplaceCatalogService
	agentContext        core.AgentContextService
	soulAuthoring       core.SoulAuthoringService
	soulHistoryPurger   core.SoulHistoryPurger
	soulRefresher       core.SoulRefresher
	heartbeatAuthor     core.HeartbeatAuthoringService
	heartbeatPurger     core.HeartbeatHistoryPurger
	heartbeatStatus     core.HeartbeatStatusService
	heartbeatWake       core.HeartbeatWakeService
	sessionHealth       core.SessionHealthReader
	wakeEvents          core.HeartbeatWakeEventReader
	coordinatorRole     core.CoordinatorRoleResolver
	roles               core.RolesStatusProvider
	skillsRegistry      core.SkillsRegistry
	skillResources      core.SkillResourceSyncer
	memoryStore         *memory.Store
	dreamTrigger        core.DreamTrigger
	memoryExtractor     core.MemoryExtractorService
	memoryProviders     core.MemoryProviderService
	memoryLedger        core.MemorySessionLedgerService
	runtimeMemory       doctor.RuntimeMemorySnapshotSource
	deadEntities        doctor.DeadEntitySource
	gateway             core.GatewayService
	homePaths           compozyconfig.HomePaths
	config              compozyconfig.Config
	logger              *slog.Logger
	startedAt           time.Time
	now                 func() time.Time
	pollInterval        time.Duration
	agentLoader         core.AgentLoader
	extensions          ExtensionService
	hostedMCP           *mcppkg.HostedService
	mcpHostAPI          mcppkg.HostAPIInvoker
}
