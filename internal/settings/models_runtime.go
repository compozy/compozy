package settings

import (
	"time"

	automationmodel "github.com/compozy/agh/internal/automation/model"
	aghconfig "github.com/compozy/agh/internal/config"
	skillspkg "github.com/compozy/agh/internal/skills"
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
	Config  aghconfig.MemoryConfig
	Health  MemoryHealthStatus
	Actions MemoryActions
}

// RolesSection is the background-role routing read model.
type RolesSection struct {
	Config aghconfig.RolesConfig
}

// SkillsSection is the skills section read model.
type SkillsSection struct {
	Config           aghconfig.SkillsConfig
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
	Config  aghconfig.NetworkConfig
	Runtime NetworkRuntimeStatus
	Links   []OperationalLink
}

// WindowManagerSection is the window-manager section read model.
type WindowManagerSection struct {
	Config aghconfig.WindowManagerConfig
}

// ObservabilitySection is the observability section read model.
type ObservabilitySection struct {
	Config         aghconfig.ObservabilityConfig
	Runtime        ObservabilityRuntimeStatus
	LogTailSupport CapabilityStatus
}

// HooksExtensionsSection is the hooks and extensions section read model.
type HooksExtensionsSection struct {
	Hooks           []HookItem
	Extensions      aghconfig.ExtensionsConfig
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
