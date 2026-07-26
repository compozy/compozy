package settings

import (
	"context"
	"fmt"
	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/config/lifecycle"
	diagnosticcontract "github.com/compozy/agh/internal/diagnosticcontract"
	hookspkg "github.com/compozy/agh/internal/hooks"
)

// ScopeKind identifies the supported settings scope.
type ScopeKind string

const (
	// ScopeGlobal selects the global AGH home scope.
	ScopeGlobal ScopeKind = "global"
	// ScopeWorkspace selects one workspace-local overlay scope.
	ScopeWorkspace ScopeKind = "workspace"
	// ScopeAgent selects one effective agent-local overlay scope.
	ScopeAgent ScopeKind = "agent"
)

// Validate ensures the requested settings scope is supported.
func (s ScopeKind) Validate() error {
	switch s {
	case ScopeGlobal, ScopeWorkspace, ScopeAgent:
		return nil
	default:
		return fmt.Errorf("settings: invalid scope %q", s)
	}
}

func (s ScopeKind) configWriteScope() aghconfig.WriteScope {
	if s == ScopeWorkspace {
		return aghconfig.WriteScopeWorkspace
	}
	return aghconfig.WriteScopeGlobal
}

// WriteTargetKind identifies the semantic persistence target for one mutation.
type WriteTargetKind = aghconfig.WriteTargetKind

const (
	// WriteTargetGlobalConfig persists to `~/.agh/config.toml`.
	WriteTargetGlobalConfig = aghconfig.WriteTargetGlobalConfig
	// WriteTargetWorkspaceConfig persists to `<workspace>/.agh/config.toml`.
	WriteTargetWorkspaceConfig = aghconfig.WriteTargetWorkspaceConfig
	// WriteTargetGlobalMCPSidecar persists to `~/.agh/mcp.json`.
	WriteTargetGlobalMCPSidecar = aghconfig.WriteTargetGlobalMCPSidecar
	// WriteTargetWorkspaceMCPSidecar persists to `<workspace>/.agh/mcp.json`.
	WriteTargetWorkspaceMCPSidecar = aghconfig.WriteTargetWorkspaceMCPSidecar
	// WriteTargetGlobalAgentFile persists to `~/.agh/agents/<name>/AGENT.md`.
	WriteTargetGlobalAgentFile WriteTargetKind = "global-agent-file"
	// WriteTargetWorkspaceAgentFile persists to `<root>/.agh/agents/<name>/AGENT.md`.
	WriteTargetWorkspaceAgentFile WriteTargetKind = "workspace-agent-file"
)

// SectionName names one section-oriented settings resource.
type SectionName string

const (
	// SectionGeneral exposes daemon-wide runtime and config defaults.
	SectionGeneral SectionName = "general"
	// SectionMemory exposes memory and dream settings.
	SectionMemory SectionName = "memory"
	// SectionRoles exposes background role routing settings.
	SectionRoles SectionName = "roles"
	// SectionSkills exposes global skills-engine settings.
	SectionSkills SectionName = "skills"
	// SectionAutomation exposes automation engine settings.
	SectionAutomation SectionName = "automation"
	// SectionNetwork exposes embedded network settings.
	SectionNetwork SectionName = "network"
	// SectionWindowManager exposes daemon-owned window behavior defaults.
	SectionWindowManager SectionName = "window-manager"
	// SectionObservability exposes observe and transcript settings.
	SectionObservability SectionName = "observability"
	// SectionHooksExtensions exposes hook declarations plus extension policy.
	SectionHooksExtensions SectionName = "hooks-extensions"
)

// CollectionName names one collection-oriented settings resource.
type CollectionName string

const (
	// CollectionProviders exposes the provider catalog.
	CollectionProviders CollectionName = "providers"
	// CollectionMCPServers exposes the scoped MCP server catalog.
	CollectionMCPServers CollectionName = "mcp-servers"
	// CollectionSandboxes exposes execution sandboxes.
	CollectionSandboxes CollectionName = "sandboxes"
	// CollectionHooks exposes config-defined hook declarations.
	CollectionHooks CollectionName = "hooks"
)

// TargetSelector selects one MCP persistence destination.
type TargetSelector string

const (
	// TargetAuto edits the highest-precedence definition in the selected scope.
	TargetAuto TargetSelector = "auto"
	// TargetConfig edits the TOML-backed source in the selected scope.
	TargetConfig TargetSelector = "config"
	// TargetSidecar edits the sidecar-backed source in the selected scope.
	TargetSidecar TargetSelector = "sidecar"
)

// Normalize returns the canonical selector, defaulting an omitted selector to auto.
func (s TargetSelector) Normalize() TargetSelector {
	trimmed := TargetSelector(strings.TrimSpace(string(s)))
	if trimmed == "" {
		return TargetAuto
	}
	return trimmed
}

// Validate reports malformed MCP target selectors instead of silently selecting auto.
func (s TargetSelector) Validate() error {
	normalized := s.Normalize()
	switch normalized {
	case TargetAuto, TargetConfig, TargetSidecar:
		return nil
	default:
		return validationError(fmt.Errorf("settings: unsupported MCP target selector %q", normalized))
	}
}

// MutationBehavior classifies how a mutation takes effect at runtime.
type MutationBehavior string

const (
	// MutationBehaviorAppliedNow reports a live-applied mutation.
	MutationBehaviorAppliedNow MutationBehavior = "applied_now"
	// MutationBehaviorRestartRequired reports a persisted mutation that needs restart.
	MutationBehaviorRestartRequired MutationBehavior = "restart_required"
	// MutationBehaviorActionTrigger reports a mutation that triggers an action.
	MutationBehaviorActionTrigger MutationBehavior = "action_trigger"
)

// SourceKind identifies one semantic resource source.
type SourceKind string

const (
	// SourceKindBuiltinProvider identifies the builtin provider registry.
	SourceKindBuiltinProvider SourceKind = "builtin-provider"
	// SourceKindGlobalConfig identifies the global TOML config.
	SourceKindGlobalConfig SourceKind = "global-config"
	// SourceKindWorkspaceConfig identifies the workspace TOML config.
	SourceKindWorkspaceConfig SourceKind = "workspace-config"
	// SourceKindGlobalMCPSidecar identifies the global MCP JSON sidecar.
	SourceKindGlobalMCPSidecar SourceKind = "global-mcp-sidecar"
	// SourceKindWorkspaceMCPSidecar identifies the workspace MCP JSON sidecar.
	SourceKindWorkspaceMCPSidecar SourceKind = "workspace-mcp-sidecar"
	// SourceKindGlobalAgentFile identifies a global AGENT.md frontmatter source.
	SourceKindGlobalAgentFile SourceKind = "global-agent-file"
	// SourceKindWorkspaceAgentFile identifies a workspace/additional AGENT.md frontmatter source.
	SourceKindWorkspaceAgentFile SourceKind = "workspace-agent-file"
)

// SectionRequest identifies one section read.
type SectionRequest struct {
	Section     SectionName
	Scope       ScopeKind
	WorkspaceID string
	AgentName   string
}

// SectionUpdateRequest identifies one section mutation.
type SectionUpdateRequest struct {
	SectionRequest
	General         *GeneralSettings
	Memory          *aghconfig.MemoryConfig
	Roles           *aghconfig.RolesConfig
	Skills          *aghconfig.SkillsConfig
	Automation      *AutomationSettings
	Network         *aghconfig.NetworkConfig
	WindowManager   *aghconfig.WindowManagerConfig
	Observability   *aghconfig.ObservabilityConfig
	HooksExtensions *aghconfig.ExtensionsConfig
}

// CollectionRequest identifies one collection read.
type CollectionRequest struct {
	Collection  CollectionName
	Scope       ScopeKind
	WorkspaceID string
}

// CollectionItemPutRequest identifies one collection upsert.
type CollectionItemPutRequest struct {
	CollectionRequest
	Name                  string
	Target                TargetSelector
	Provider              *ProviderSettings
	ProviderModelCuration *ProviderModelCurationRequest
	ProviderSecrets       []ProviderSecretWrite
	MCPServer             *aghconfig.MCPServer
	MCPSecrets            MCPSecretValues
	MCPSecretPreservation MCPSecretPreservation
	MCPEnvPreservation    []string
	Sandbox               *aghconfig.SandboxProfile
	Hook                  *hookspkg.HookDecl
}

// CollectionItemDeleteRequest identifies one collection delete.
type CollectionItemDeleteRequest struct {
	CollectionRequest
	Name   string
	Target TargetSelector
}

// SectionEnvelope returns one typed section payload.
type SectionEnvelope struct {
	Section         SectionName
	Scope           ScopeKind
	WorkspaceID     string
	AgentName       string
	AvailableScopes []ScopeKind
	General         *GeneralSection
	Memory          *MemorySection
	Roles           *RolesSection
	Skills          *SkillsSection
	Automation      *AutomationSection
	Network         *NetworkSection
	WindowManager   *WindowManagerSection
	Observability   *ObservabilitySection
	HooksExtensions *HooksExtensionsSection
}

// CollectionEnvelope returns one typed collection payload.
type CollectionEnvelope struct {
	Collection      CollectionName
	Scope           ScopeKind
	WorkspaceID     string
	AvailableScopes []ScopeKind
	Providers       []ProviderItem
	MCPServers      []MCPServerItem
	Sandboxes       []SandboxItem
	Hooks           []HookItem
}

// MutationResult reports the semantic outcome of one settings mutation.
type MutationResult struct {
	Section         SectionName      `json:"section"`
	Scope           ScopeKind        `json:"scope"`
	WriteTarget     WriteTargetKind  `json:"write_target,omitempty"`
	WorkspaceID     string           `json:"workspace_id,omitempty"`
	AgentName       string           `json:"agent_name,omitempty"`
	Behavior        MutationBehavior `json:"behavior"`
	Applied         bool             `json:"applied"`
	RestartRequired bool             `json:"restart_required"`
	RestartScope    string           `json:"restart_scope,omitempty"`
	Warnings        []string         `json:"warnings,omitempty"`
	Lifecycle       lifecycle.Lifecycle
	DiffClass       lifecycle.DiffClass
	MCPServer       *MCPServerItem `json:"-"`
}

// MutationDescriptor identifies the changed fields or action behind one mutation.
type MutationDescriptor struct {
	Section       SectionName
	ChangedFields []string
	Action        string
}

// MutationClassification reports the classified runtime behavior for one mutation.
type MutationClassification struct {
	Behavior        MutationBehavior
	Applied         bool
	RestartRequired bool
	RestartScope    string
	Lifecycle       lifecycle.Lifecycle
	DiffClass       lifecycle.DiffClass
}

// ApplyResult is the public config-apply outcome returned by mutation, reload,
// and reconcile paths after the attempt is persisted.
type ApplyResult struct {
	Record          ApplyRecord
	Section         SectionName
	Scope           ScopeKind
	WriteTarget     WriteTargetKind
	WorkspaceID     string
	AgentName       string
	Applied         bool
	NextAction      lifecycle.NextAction
	RestartRequired bool
	RestartScope    string
	Warnings        []string
	PartialFailures []ApplyFailure
	Skipped         bool
	SkippedReason   string
	MCPServer       *MCPServerItem `json:"-"`
}

// ApplyFailure reports one subsystem failure after the desired config was written.
type ApplyFailure struct {
	Subsystem  string
	Diagnostic diagnosticcontract.DiagnosticItem
}

// ApplyRecord is one persisted config_apply_records row.
type ApplyRecord struct {
	ID          string
	DesiredHash string
	ActiveHash  string
	Generation  int64
	Actor       string
	DiffClass   lifecycle.DiffClass
	Status      lifecycle.Status
	Lifecycle   lifecycle.Lifecycle
	NextAction  lifecycle.NextAction
	Diagnostics []diagnosticcontract.DiagnosticItem
	CreatedAt   time.Time
	AppliedAt   *time.Time
	UpdatedAt   time.Time
}

// ApplyRecordFilter selects apply-record history rows.
type ApplyRecordFilter struct {
	Status lifecycle.Status
	Actor  string
	Limit  int
}

// ApplyRecordStore persists config apply attempts.
type ApplyRecordStore interface {
	CreateApplyRecord(ctx context.Context, record ApplyRecord) (ApplyRecord, error)
	UpdateApplyRecord(ctx context.Context, record ApplyRecord) (ApplyRecord, error)
	ListApplyRecords(ctx context.Context, filter ApplyRecordFilter) ([]ApplyRecord, error)
	LatestAppliedRecord(ctx context.Context) (*ApplyRecord, error)
}
