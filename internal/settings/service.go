package settings

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/marketplace"
	mcpauth "github.com/compozy/agh/internal/mcp/auth"
	"github.com/compozy/agh/internal/modelcatalog"
	skillspkg "github.com/compozy/agh/internal/skills"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/vault"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

// WorkspaceResolver resolves and lists registered workspaces for settings flows.
type WorkspaceResolver interface {
	Resolve(ctx context.Context, idOrNameOrPath string) (workspacepkg.ResolvedWorkspace, error)
	List(ctx context.Context) ([]workspacepkg.Workspace, error)
}

// GeneralRuntimeProvider returns general daemon runtime metadata.
type GeneralRuntimeProvider interface {
	GeneralRuntimeStatus(ctx context.Context) (DaemonRuntimeStatus, error)
}

// MemoryRuntimeProvider returns memory runtime metadata.
type MemoryRuntimeProvider interface {
	MemoryHealthStatus(ctx context.Context) (MemoryHealthStatus, error)
}

// SkillsRuntime exposes the global skills registry state used by settings.
type SkillsRuntime interface {
	List() []*skillspkg.Skill
	ForAgent(
		ctx context.Context,
		resolved *workspacepkg.ResolvedWorkspace,
		agentName string,
	) ([]*skillspkg.Skill, error)
	SetEnabled(name string, resolved *workspacepkg.ResolvedWorkspace, enabled bool) error
	SetEnabledForAgent(name string, resolved *workspacepkg.ResolvedWorkspace, agentName string, enabled bool) error
}

// SkillsDiagnosticsRuntime optionally exposes resolver diagnostics for settings.
type SkillsDiagnosticsRuntime interface {
	SkillDiagnostics(
		ctx context.Context,
		resolved *workspacepkg.ResolvedWorkspace,
		agentName string,
	) ([]skillspkg.SkillDiagnostic, error)
}

// AutomationRuntimeProvider returns automation runtime metadata.
type AutomationRuntimeProvider interface {
	AutomationRuntimeStatus(ctx context.Context) (AutomationRuntimeStatus, error)
}

// NetworkRuntimeProvider returns network runtime metadata.
type NetworkRuntimeProvider interface {
	NetworkRuntimeStatus(ctx context.Context) (NetworkRuntimeStatus, error)
}

// ObservabilityRuntimeProvider returns observability runtime metadata.
type ObservabilityRuntimeProvider interface {
	ObservabilityRuntimeStatus(ctx context.Context) (ObservabilityRuntimeStatus, error)
}

// ExtensionStatusProvider returns installed extension summaries.
type ExtensionStatusProvider interface {
	InstalledExtensions(ctx context.Context) ([]InstalledExtension, error)
}

// TransportParityProvider returns settings transport parity metadata.
type TransportParityProvider interface {
	TransportParityStatus(ctx context.Context) (TransportParityStatus, error)
}

// MCPAuthRuntimeProvider owns daemon-mediated MCP OAuth sessions and status.
type MCPAuthRuntimeProvider interface {
	MCPAuthStatus(ctx context.Context, target mcpauth.Target, server aghconfig.MCPServer) (mcpauth.Status, error)
	MCPAuthBegin(
		ctx context.Context,
		target mcpauth.Target,
		server aghconfig.MCPServer,
		callbackURL string,
	) (mcpauth.BeginResult, error)
	MCPAuthExchange(
		ctx context.Context,
		target mcpauth.Target,
		server aghconfig.MCPServer,
		input mcpauth.ExchangeInput,
	) (mcpauth.Status, error)
	MCPAuthCallbackTarget(callbackURL string) (mcpauth.Target, error)
	MCPAuthCompleteCallback(
		ctx context.Context,
		target mcpauth.Target,
		server aghconfig.MCPServer,
		callbackURL string,
	) (mcpauth.Status, error)
	MCPAuthInvalidate(target mcpauth.Target) error
	MCPAuthLogout(
		ctx context.Context,
		target mcpauth.Target,
		server aghconfig.MCPServer,
	) (mcpauth.Status, error)
}

// MCPRuntimeProvider returns daemon-observed runtime probe status for settings rows.
type MCPRuntimeProvider interface {
	MCPServerRuntimeStatus(
		ctx context.Context,
		target mcpauth.Target,
		server aghconfig.MCPServer,
	) (MCPServerRuntimeStatus, error)
}

// ConfigRuntimeApplier reconciles a validated config snapshot with daemon-owned runtime state.
type ConfigRuntimeApplier interface {
	// ApplyActiveConfig installs a fully projected next-active snapshot, not the persisted desired config.
	ApplyActiveConfig(ctx context.Context, snap *aghconfig.Config) []ApplyFailure
}

// ProviderSecretStore stores provider-bound secrets and returns redacted metadata.
type ProviderSecretStore interface {
	GetMetadata(ctx context.Context, ref string) (vault.Metadata, error)
	ResolveRef(ctx context.Context, ref string) (string, error)
	PutSecret(ctx context.Context, ref string, kind string, plaintext string) (vault.Metadata, error)
	DeleteSecret(ctx context.Context, ref string) error
}

// MCPCatalog provides the single curated entry read needed by settings-owned install orchestration.
type MCPCatalog interface {
	Detail(ctx context.Context, kind marketplace.Kind, entryID string) (*marketplace.Entry, error)
}

// MarketplaceInstallNotifier persists redacted install outcomes.
type MarketplaceInstallNotifier interface {
	NotifyInstall(ctx context.Context, outcome marketplace.InstallOutcome) error
}

// MCPDefinitionWriter persists one MCP definition to its selected config target.
type MCPDefinitionWriter func(
	homePaths aghconfig.HomePaths,
	workspaceRoot string,
	name string,
	target aghconfig.WriteTarget,
	server aghconfig.MCPServer,
) error

// Dependencies captures the runtime dependencies required by the settings service.
type Dependencies struct {
	WorkspaceResolver          WorkspaceResolver
	GeneralRuntime             GeneralRuntimeProvider
	MemoryRuntime              MemoryRuntimeProvider
	SkillsRuntime              SkillsRuntime
	AutomationRuntime          AutomationRuntimeProvider
	NetworkRuntime             NetworkRuntimeProvider
	ObservabilityRuntime       ObservabilityRuntimeProvider
	Extensions                 ExtensionStatusProvider
	TransportParity            TransportParityProvider
	MCPAuth                    MCPAuthRuntimeProvider
	MCPRuntime                 MCPRuntimeProvider
	MCPCatalog                 MCPCatalog
	MarketplaceInstallEvents   MarketplaceInstallNotifier
	MCPDefinitionWriter        MCPDefinitionWriter
	ModelCatalog               modelcatalog.Service
	RuntimeApplier             ConfigRuntimeApplier
	ProviderSecrets            ProviderSecretStore
	EventSummaries             store.EventSummaryStore
	ApplyRecords               ApplyRecordStore
	RestartActionAvailable     bool
	ConsolidateActionAvailable bool
	LogTailAvailable           bool
	CommandLookPath            func(string) (string, error)
	LookupEnv                  func(string) (string, bool)
}

type service struct {
	homePaths                  aghconfig.HomePaths
	workspaceResolver          WorkspaceResolver
	generalRuntime             GeneralRuntimeProvider
	memoryRuntime              MemoryRuntimeProvider
	skillsRuntime              SkillsRuntime
	automationRuntime          AutomationRuntimeProvider
	networkRuntime             NetworkRuntimeProvider
	observabilityRuntime       ObservabilityRuntimeProvider
	extensions                 ExtensionStatusProvider
	transportParity            TransportParityProvider
	mcpAuth                    MCPAuthRuntimeProvider
	mcpRuntime                 MCPRuntimeProvider
	mcpCatalog                 MCPCatalog
	marketplaceInstallEvents   MarketplaceInstallNotifier
	mcpDefinitionWriter        MCPDefinitionWriter
	modelCatalog               modelcatalog.Service
	runtimeApplier             ConfigRuntimeApplier
	providerSecrets            ProviderSecretStore
	eventSummaries             store.EventSummaryStore
	applyRecords               ApplyRecordStore
	activeConfig               activeConfigState
	applyMu                    sync.Mutex
	restartActionAvailable     bool
	consolidateActionAvailable bool
	logTailAvailable           bool
	commandLookPath            func(string) (string, error)
	lookupEnv                  func(string) (string, bool)
}

var _ Service = (*service)(nil)

// NewService constructs the daemon-facing settings orchestration service.
func NewService(homePaths aghconfig.HomePaths, deps Dependencies) (Service, error) {
	if strings.TrimSpace(homePaths.HomeDir) == "" {
		return nil, errors.New("settings: home paths are required")
	}

	commandLookPath := deps.CommandLookPath
	if commandLookPath == nil {
		commandLookPath = exec.LookPath
	}
	lookupEnv := deps.LookupEnv
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	mcpDefinitionWriter := deps.MCPDefinitionWriter
	if mcpDefinitionWriter == nil {
		mcpDefinitionWriter = writeMCPDefinition
	}

	return &service{
		homePaths:                  homePaths,
		workspaceResolver:          deps.WorkspaceResolver,
		generalRuntime:             deps.GeneralRuntime,
		memoryRuntime:              deps.MemoryRuntime,
		skillsRuntime:              deps.SkillsRuntime,
		automationRuntime:          deps.AutomationRuntime,
		networkRuntime:             deps.NetworkRuntime,
		observabilityRuntime:       deps.ObservabilityRuntime,
		extensions:                 deps.Extensions,
		transportParity:            deps.TransportParity,
		mcpAuth:                    deps.MCPAuth,
		mcpRuntime:                 deps.MCPRuntime,
		mcpCatalog:                 deps.MCPCatalog,
		marketplaceInstallEvents:   deps.MarketplaceInstallEvents,
		mcpDefinitionWriter:        mcpDefinitionWriter,
		modelCatalog:               deps.ModelCatalog,
		runtimeApplier:             deps.RuntimeApplier,
		providerSecrets:            deps.ProviderSecrets,
		eventSummaries:             deps.EventSummaries,
		applyRecords:               deps.ApplyRecords,
		restartActionAvailable:     deps.RestartActionAvailable,
		consolidateActionAvailable: deps.ConsolidateActionAvailable,
		logTailAvailable:           deps.LogTailAvailable,
		commandLookPath:            commandLookPath,
		lookupEnv:                  lookupEnv,
	}, nil
}

func (s *service) normalizeReadScope(scope ScopeKind, workspaceID string) (ScopeKind, string, error) {
	normalized := scope
	if normalized == "" {
		normalized = ScopeGlobal
	}
	if err := normalized.Validate(); err != nil {
		return "", "", validationError(err)
	}

	trimmedWorkspaceID := strings.TrimSpace(workspaceID)
	if normalized == ScopeGlobal && trimmedWorkspaceID != "" {
		return "", "", conflictError(errors.New("settings: workspace_id requires workspace scope"))
	}
	return normalized, trimmedWorkspaceID, nil
}

func normalizeAgentName(agentName string) (string, error) {
	normalized := aghconfig.NormalizeAgentName(agentName)
	if normalized == "" {
		return "", nil
	}
	if err := aghconfig.ValidateAgentName(normalized); err != nil {
		return "", validationError(err)
	}
	return normalized, nil
}

func (s *service) resolveWorkspace(
	ctx context.Context,
	scope ScopeKind,
	workspaceID string,
) (*workspacepkg.ResolvedWorkspace, error) {
	trimmedWorkspaceID := strings.TrimSpace(workspaceID)
	if scope != ScopeWorkspace && (scope != ScopeAgent || trimmedWorkspaceID == "") {
		return nil, nil
	}
	if trimmedWorkspaceID == "" {
		return nil, conflictError(errors.New("settings: workspace scope requires a workspace_id"))
	}
	if s.workspaceResolver == nil {
		return nil, errors.New("settings: workspace resolver is required for workspace scope")
	}

	resolved, err := s.workspaceResolver.Resolve(ctx, trimmedWorkspaceID)
	if err != nil {
		return nil, err
	}
	return &resolved, nil
}

func (s *service) loadConfig(
	ctx context.Context,
	scope ScopeKind,
	workspaceID string,
) (aghconfig.Config, *workspacepkg.ResolvedWorkspace, error) {
	normalizedScope, normalizedWorkspaceID, err := s.normalizeReadScope(scope, workspaceID)
	if err != nil {
		return aghconfig.Config{}, nil, err
	}

	resolved, err := s.resolveWorkspace(ctx, normalizedScope, normalizedWorkspaceID)
	if err != nil {
		return aghconfig.Config{}, nil, err
	}

	if resolved != nil {
		cfg, loadErr := aghconfig.LoadForHome(s.homePaths, aghconfig.WithWorkspaceRoot(resolved.RootDir))
		return cfg, resolved, loadErr
	}

	cfg, loadErr := aghconfig.LoadForHome(s.homePaths)
	return cfg, nil, loadErr
}

func workspaceConfigPath(root string) string {
	return filepath.Join(strings.TrimSpace(root), aghconfig.DirName, aghconfig.ConfigName)
}
