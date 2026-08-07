package daemon

import (
	"context"

	core "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	memorypkg "github.com/compozy/compozy/internal/memory"
	skillspkg "github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/windowmanager"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type daemonNativeSkillsRegistry interface {
	core.SkillsRegistry
	ForAgentSession(
		ctx context.Context,
		resolved *workspacepkg.ResolvedWorkspace,
		agentName string,
		sessionID string,
	) ([]*skillspkg.Skill, error)
	ForAgentDefSession(
		ctx context.Context,
		resolved *workspacepkg.ResolvedWorkspace,
		agent compozyconfig.AgentDef,
		sessionID string,
	) ([]*skillspkg.Skill, error)
}

type extensionPublishSecretResolver interface {
	ResolveRef(context.Context, string) (string, error)
}

type daemonNativeToolsDeps struct {
	Registry                   func() toolspkg.Registry
	ToolArtifacts              toolspkg.ToolArtifactStore
	Config                     compozyconfig.Config
	Skills                     daemonNativeSkillsRegistry
	Sessions                   core.SessionManager
	Workspaces                 core.WorkspaceService
	WorkspaceResolver          workspacepkg.RuntimeResolver
	ModelCatalog               core.ModelCatalogService
	MarketplaceCatalog         core.MarketplaceCatalogService
	MarketplaceSkills          core.SkillMarketplaceService
	MarketplaceInstalledSkills core.InstalledSkillMarketplaceService
	Settings                   func() core.SettingsService
	Network                    core.NetworkService
	NetworkStore               core.NetworkStore
	NetworkUsage               store.NetworkUsageStore
	Tasks                      taskpkg.Manager
	MemoryStore                *memorypkg.Store
	MemoryToolWrites           memoryToolWriteRecorder
	DreamTrigger               core.DreamTrigger
	MemoryExtractor            core.MemoryExtractorService
	MemoryProviders            core.MemoryProviderService
	MemorySessionLedger        core.MemorySessionLedgerService
	Bridges                    core.BridgeService
	Gateway                    func() core.GatewayService
	GatewayPermissionMode      func(context.Context, string) (string, error)
	HomePaths                  compozyconfig.HomePaths
	Observer                   core.Observer
	HookBindings               hookBindingPublisher
	AgentCatalog               core.AgentCatalog
	HeartbeatStatus            core.HeartbeatStatusService
	HeartbeatWake              core.HeartbeatWakeService
	SessionHealth              core.SessionHealthReader
	WakeEvents                 core.HeartbeatWakeEventReader
	Automation                 core.AutomationManager
	AutomationRuntime          func() core.AutomationManager
	ExtensionRegistry          *extensionpkg.Registry
	Extensions                 func() core.ExtensionService
	ExtensionRuntime           func() extensionRuntime
	ExtensionConfig            compozyconfig.ExtensionsConfig
	ExtensionSources           extensionMarketplaceSourceLoader
	ExtensionEvents            extensionLifecycleEventWriter
	ExtensionSecrets           extensionPublishSecretResolver
	AgentSkills                agentSkillPublisher
	AgentSkillsRuntime         func() agentSkillPublisher
	ToolMCP                    toolMCPPublisher
	MCPAuth                    func() toolspkg.MCPAuthStatusProvider
	ApprovalGrants             toolspkg.ApprovalGrantStore
	Clarify                    func() toolspkg.ClarifyBroker
	LoopResources              loopResourcePublisher
	Loops                      func() core.LoopService
	Resources                  core.ResourceService
	WindowManager              windowmanager.Service
}

func (d *daemonNativeToolsDeps) agentSkills() agentSkillPublisher {
	if d == nil {
		return nil
	}
	if d.AgentSkillsRuntime != nil {
		if publisher := d.AgentSkillsRuntime(); publisher != nil {
			return publisher
		}
	}
	return d.AgentSkills
}

func (d *daemonNativeToolsDeps) gatewayService() core.GatewayService {
	if d == nil || d.Gateway == nil {
		return nil
	}
	return d.Gateway()
}
