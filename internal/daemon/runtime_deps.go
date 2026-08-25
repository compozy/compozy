package daemon

import (
	"log/slog"
	"time"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/udsapi"
	attachmentspkg "github.com/compozy/compozy/internal/attachments"
	"github.com/compozy/compozy/internal/cmdpalette"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/doctor"
	"github.com/compozy/compozy/internal/gateway"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	"github.com/compozy/compozy/internal/memory"
	"github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/situation"
	taskpkg "github.com/compozy/compozy/internal/task"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

// RuntimeDeps captures the composition-root dependencies available to server factories.
type RuntimeDeps struct {
	Config              compozyconfig.Config
	AgentProbeConfig    *agentProbeConfigState
	HomePaths           compozyconfig.HomePaths
	Logger              *slog.Logger
	Sessions            SessionManager
	DrainController     core.DaemonDrainController
	Tasks               taskpkg.Manager
	Network             core.NetworkService
	ToolRegistry        toolspkg.Registry
	Toolsets            core.ToolsetRegistry
	ToolArtifacts       toolspkg.ToolArtifactStore
	SessionAttachments  attachmentspkg.Store
	ToolApprovals       toolspkg.ApprovalTokenIssuer
	ApprovalCoordinator toolspkg.ApprovalCoordinator
	CmdPalette          cmdpalette.Registry
	ApprovalGrants      toolspkg.ApprovalGrantStore
	Clarify             toolspkg.ClarifyBroker
	HostedMCP           *mcppkg.HostedService
	MCPHostAPI          mcppkg.HostAPIInvoker
	Observer            Observer
	SchemaStreams       core.SchemaStreamStatusReader
	Automation          core.AutomationManager
	Loops               core.LoopService
	Bridges             core.BridgeService
	Notifications       core.NotificationPresetService
	Registry            Registry
	Profiles            *profile.Manager
	MemoryStore         *memory.Store
	MemoryExtractor     core.MemoryExtractorService
	MemoryProviders     core.MemoryProviderService
	MemorySessionLedger core.MemorySessionLedgerService
	RuntimeMemory       doctor.RuntimeMemorySnapshotSource
	DeadEntities        doctor.DeadEntitySource
	WorkspaceResolver   workspacepkg.RuntimeResolver
	WorkspaceService    core.WorkspaceService
	Worktrees           core.WorktreeService
	WorkspaceAccess     workspaceaccess.Policy
	AgentCatalog        core.AgentCatalog
	AgentResolver       session.AgentResolver
	AgentDefinitionSync core.AgentDefinitionSync
	ModelCatalog        core.ModelCatalogService
	MarketplaceCatalog  core.MarketplaceCatalogService
	AgentContext        *situation.Service
	SoulAuthoring       core.SoulAuthoringService
	SoulHistoryPurger   core.SoulHistoryPurger
	SoulRefresher       core.SoulRefresher
	HeartbeatAuthor     core.HeartbeatAuthoringService
	HeartbeatPurger     core.HeartbeatHistoryPurger
	HeartbeatStatus     core.HeartbeatStatusService
	HeartbeatWake       core.HeartbeatWakeService
	SessionHealth       core.SessionHealthReader
	WakeEvents          core.HeartbeatWakeEventReader
	CoordinatorRole     CoordinatorRoleResolver
	Roles               core.RolesStatusProvider
	SkillsRegistry      core.SkillsRegistry
	SkillResources      core.SkillResourceSyncer
	DreamTrigger        DreamTrigger
	Settings            core.SettingsService
	SettingsRestart     core.SettingsRestartController
	SettingsUpdate      core.SettingsUpdateController
	SupportBundles      core.SupportBundleService
	Vault               core.VaultService
	Extensions          udsapi.ExtensionService
	Resources           core.ResourceService
	WindowManagers      *windowManagerRegistry
	Terminals           terminalpkg.Manager
	Gateway             *gateway.Service
	GatewayChallenges   *gateway.ChallengeRegistry
	GatewayAuthLimiter  *gateway.AuthFailureLimiter
	StartedAt           time.Time
}
