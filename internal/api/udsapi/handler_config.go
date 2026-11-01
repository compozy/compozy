package udsapi

import (
	"log/slog"
	"time"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/cmdpalette"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/doctor"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

type handlerConfig struct {
	agentInstances        *core.AgentInstanceAPI
	sessions              core.SessionManager
	drainController       core.DaemonDrainController
	subagents             session.SubagentService
	sessionMonitors       core.SessionMonitorService
	sessionProcesses      core.SessionProcessService
	sessionCatalog        core.SessionCatalog
	taskDesignations      core.TaskDesignationStore
	skillExposureStore    store.SkillExposureRepository
	skillExposureEvents   store.EventSummaryStore
	tasks                 core.TaskService
	observer              core.Observer
	schemaStreams         core.SchemaStreamStatusReader
	resources             core.ResourceService
	windowManager         core.WindowManagerProvider
	terminal              core.TerminalProvider
	automation            core.AutomationManager
	loops                 core.LoopService
	profiles              core.ProfileService
	supportBundles        core.SupportBundleService
	tools                 core.ToolRegistry
	toolArtifacts         toolspkg.ToolArtifactStore
	sessionAttachments    core.SessionAttachmentStore
	sessionFiles          core.SessionFileSearcher
	toolsets              core.ToolsetRegistry
	toolApprovals         core.ToolApprovalIssuer
	approvalGrants        core.ToolApprovalGrantService
	approvalCoordinator   toolspkg.ApprovalCoordinator
	cmdPalette            cmdpalette.Registry
	clarify               toolspkg.ClarifyBroker
	settings              core.SettingsService
	settingsRestart       core.SettingsRestartController
	settingsUpdate        core.SettingsUpdateController
	vault                 core.VaultService
	workspaces            core.WorkspaceService
	worktrees             core.WorktreeService
	workspaceAccess       workspaceaccess.Policy
	onboarding            core.OnboardingStore
	agentCatalog          core.AgentCatalog
	agentSync             core.AgentDefinitionSync
	contextWindowResolver core.ContextWindowResolver
	modelCatalog          core.ModelCatalogService
	providerUsage         core.ProviderUsageService
	marketplaceCatalog    core.MarketplaceCatalogService
	agentContext          core.AgentContextService
	soulAuthoring         core.SoulAuthoringService
	soulHistoryPurger     core.SoulHistoryPurger
	soulRefresher         core.SoulRefresher
	heartbeatAuthor       core.HeartbeatAuthoringService
	heartbeatPurger       core.HeartbeatHistoryPurger
	heartbeatStatus       core.HeartbeatStatusService
	heartbeatWake         core.HeartbeatWakeService
	wakeEvents            core.HeartbeatWakeEventReader
	sessionHealth         core.SessionHealthReader
	coordinatorRole       core.CoordinatorRoleResolver
	roles                 core.RolesStatusProvider
	skillsRegistry        core.SkillsRegistry
	skillResources        core.SkillResourceSyncer
	runtimeMemory         doctor.RuntimeMemorySnapshotSource
	hostLoad              doctor.HostLoadSnapshotSource
	deadEntities          doctor.DeadEntitySource
	appTokens             core.AppTokenManager
	openAPI               *core.OpenAPIDocument
	gateway               core.GatewayService
	homePaths             compozyconfig.HomePaths
	config                compozyconfig.Config
	logger                *slog.Logger
	startedAt             time.Time
	now                   func() time.Time
	pollInterval          time.Duration
	agentLoader           core.AgentLoader
	extensions            ExtensionService
	hostedMCP             *mcppkg.HostedService
	mcpHostAPI            mcppkg.HostAPIInvoker

	onProviderAuthSuccess func()
}
