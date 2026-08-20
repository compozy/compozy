package settings

import (
	"time"

	automationmodel "github.com/compozy/compozy/internal/automation/model"
	compozyconfig "github.com/compozy/compozy/internal/config"
	skillspkg "github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/windowmanager"
)

// AutomationSettings groups the editable automation-engine settings.
type AutomationSettings struct {
	Enabled           bool
	Timezone          string
	MaxConcurrentJobs int
	DefaultFireLimit  automationmodel.FireLimitConfig
}

// GeneralSection is the general section read model.
type GeneralSection struct {
	Runtime     DaemonRuntimeStatus
	ConfigPaths ConfigPaths
	Settings    GeneralSettings
	Actions     GeneralActions
}

// MemorySection is the memory section read model.
type MemorySection struct {
	Config  compozyconfig.MemoryConfig
	Health  MemoryHealthStatus
	Actions MemoryActions
}

// RolesSection is the background-role routing read model.
type RolesSection struct {
	Config compozyconfig.RolesConfig
}

// SkillsSection is the skills section read model.
type SkillsSection struct {
	Config           compozyconfig.SkillsConfig
	DiscoveredCount  int
	DisabledCount    int
	RuntimeAvailable bool
	Diagnostics      []skillspkg.SkillDiagnostic
	Links            []OperationalLink
}

// AutomationSection is the automation section read model.
type AutomationSection struct {
	Config  AutomationSettings
	Runtime AutomationRuntimeStatus
	Links   []OperationalLink
}

// NetworkSection is the network section read model.
type NetworkSection struct {
	Config  compozyconfig.NetworkConfig
	Runtime NetworkRuntimeStatus
	Links   []OperationalLink
}

// GatewaySection is the remote gateway section read model.
type GatewaySection struct {
	Config compozyconfig.GatewayConfig
}

// WindowManagerSection is the window-manager section read model.
type WindowManagerSection struct {
	Config             compozyconfig.WindowManagerConfig
	Commands           []WindowManagerShortcutCommand
	EffectiveShortcuts map[string]windowmanager.ShortcutBinding
	Aliases            map[string]string
	ExtensionDefaults  []WindowManagerExtensionDefault
	Diagnostics        []windowmanager.ShortcutDiagnostic
	GlobalShortcuts    []WindowManagerGlobalShortcut
}

// WindowManagerGlobalShortcut projects intended and shell-confirmed global binding state.
type WindowManagerGlobalShortcut struct {
	CommandID     string
	IntendedChord string
	ActiveChord   string
	Status        string
	Reason        string
	SettingsURL   string
}

// WindowManagerShortcutCommand identifies one bindable command in the current catalog.
type WindowManagerShortcutCommand struct {
	ID      string
	Title   string
	Section string
	Source  string
}

// WindowManagerExtensionDefault reserves the response shape populated by extension contributions.
type WindowManagerExtensionDefault struct {
	CommandID    string
	Binding      windowmanager.ShortcutBinding
	Dormant      bool
	ConflictWith string
}

// CmdPaletteSection is the command-palette settings read model.
type CmdPaletteSection struct {
	FallbackAgentEnabled bool
	Personalization      bool
}

// CmdPaletteUpdate changes either live command-palette control while preserving omitted values.
type CmdPaletteUpdate struct {
	FallbackAgentEnabled *bool
	Personalization      *bool
}

// AttentionSection is the operator attention section read model.
type AttentionSection struct {
	Config compozyconfig.AttentionConfig
}

// ShellSection is the operator shell-preferences read model.
type ShellSection struct {
	Config compozyconfig.ShellConfig
}

// ObservabilitySection is the observability section read model.
type ObservabilitySection struct {
	Config         compozyconfig.ObservabilityConfig
	Runtime        ObservabilityRuntimeStatus
	LogTailSupport CapabilityStatus
}

// HooksExtensionsSection is the hooks and extensions section read model.
type HooksExtensionsSection struct {
	Hooks           []HookItem
	Extensions      compozyconfig.ExtensionsConfig
	Installed       []InstalledExtension
	TransportParity TransportParityStatus
}

// ConfigPaths exposes resolved daemon config file locations.
type ConfigPaths struct {
	HomeDir          string
	GlobalConfig     string
	GlobalMCPSidecar string
	LogFile          string
	DaemonInfo       string
}

// DaemonRuntimeStatus summarizes daemon-local runtime state.
type DaemonRuntimeStatus struct {
	Available      bool
	Status         string
	PID            int
	StartedAt      time.Time
	UptimeSeconds  int64
	Socket         string
	HTTPHost       string
	HTTPPort       int
	ActiveSessions int
	ActiveAgents   int
	TotalSessions  int
	Version        string
}

// MemoryHealthStatus summarizes memory runtime state.
type MemoryHealthStatus struct {
	Available          bool
	FileCount          int
	DreamEnabled       bool
	LastConsolidatedAt *time.Time
}

// AutomationRuntimeStatus summarizes automation runtime state.
type AutomationRuntimeStatus struct {
	Available        bool
	Running          bool
	SchedulerRunning bool
	JobTotal         int
	JobEnabled       int
	TriggerTotal     int
	TriggerEnabled   int
	NextFire         *time.Time
	LastSyncedAt     *time.Time
}

// NetworkRuntimeStatus summarizes network runtime state.
type NetworkRuntimeStatus struct {
	Available         bool
	Enabled           bool
	Status            string
	LocalPeers        int
	Channels          int
	MessagesReceived  int64
	MessagesDelivered int64
	MessagesRejected  int64
}

// ObservabilityRuntimeStatus summarizes observability runtime state.
type ObservabilityRuntimeStatus struct {
	Available          bool
	Status             string
	GlobalDBSizeBytes  int64
	SessionDBSizeBytes int64
	ActiveSessions     int
	ActiveAgents       int
	UptimeSeconds      int64
}

// CapabilityStatus reports whether one auxiliary feature is available.
type CapabilityStatus struct {
	Available bool
}

// GeneralActions reports general-section action metadata.
type GeneralActions struct {
	Restart ActionMetadata
}

// MemoryActions reports memory-section action metadata.
type MemoryActions struct {
	Consolidate ActionMetadata
}

// ActionMetadata reports one action trigger's semantic behavior.
type ActionMetadata struct {
	Name      string
	Available bool
	Behavior  MutationBehavior
}

// OperationalLink reports one related operational destination.
type OperationalLink struct {
	Label string
	Path  string
}

// TransportParityStatus reports whether required settings transports are present.
type TransportParityStatus struct {
	Known          bool
	SettingsHTTP   bool
	SettingsUDS    bool
	ExtensionsHTTP bool
	ExtensionsUDS  bool
}

// InstalledExtension summarizes one installed extension for settings surfaces.
type InstalledExtension struct {
	Name          string
	Version       string
	Enabled       bool
	State         string
	Health        string
	HealthMessage string
	LastError     string
	RequiresEnv   []string
	MissingEnv    []string
	Palette       *InstalledExtensionPalette
}

// InstalledExtensionPalette is the operator-facing palette contribution summary.
type InstalledExtensionPalette struct {
	Commands []InstalledExtensionPaletteCommand
	Views    []InstalledExtensionPaletteView
}

// InstalledExtensionPaletteCommand reports one command and its effective shortcut state.
type InstalledExtensionPaletteCommand struct {
	ID             string
	Title          string
	Bindings       []string
	DefaultBinding string
	DefaultDormant bool
	ConflictWith   string
	Available      bool
	Reason         string
}

// InstalledExtensionPaletteView reports one contributed view.
type InstalledExtensionPaletteView struct {
	ID        string
	Title     string
	Available bool
	Reason    string
}

// SourceRef identifies one semantic source for a resolved resource.
type SourceRef struct {
	Kind        SourceKind
	Scope       ScopeKind
	WorkspaceID string
	AgentName   string
}

// SourceMetadata reports precedence and target information for one resource.
type SourceMetadata struct {
	EffectiveSource  SourceRef
	ShadowedSources  []SourceRef
	AvailableTargets []WriteTargetKind
}
