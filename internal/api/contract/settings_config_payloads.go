package contract

import (
	"time"

	automationmodel "github.com/compozy/compozy/internal/automation/model"
	"github.com/compozy/compozy/internal/resources"
)

type SettingsBusyInputPayload struct {
	DefaultMode string `json:"default_mode"`
}

type SettingsGeneralConfigPayload struct {
	BusyInput      *SettingsBusyInputPayload  `json:"busy_input,omitzero"`
	Limits         SettingsLimitsPayload      `json:"limits"`
	Permissions    SettingsPermissionsPayload `json:"permissions"`
	SessionTimeout string                     `json:"session_timeout"`
	HTTP           SettingsHTTPPayload        `json:"http"`
	Daemon         SettingsDaemonPayload      `json:"daemon"`
	Redact         SettingsRedactPayload      `json:"redact"`
	Terminal       SettingsTerminalPayload    `json:"terminal"`
}

type SettingsDefaultsPayload struct {
	Agent    string `json:"agent"`
	Provider string `json:"provider,omitempty"`
}

type SettingsLimitsPayload struct {
	MaxConcurrentAgents int `json:"max_concurrent_agents"`
}

type SettingsPermissionsPayload struct {
	Mode SettingsPermissionMode `json:"mode"`
}

type SettingsHTTPPayload struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type SettingsDaemonPayload struct {
	Socket               string                              `json:"socket"`
	MemoryReportInterval string                              `json:"memory_report_interval"`
	ReloadTimeouts       SettingsDaemonReloadTimeoutsPayload `json:"reload_timeouts"`
}

type SettingsDaemonReloadTimeoutsPayload struct {
	Providers string `json:"providers"`
	MCP       string `json:"mcp"`
}

type SettingsRedactPayload struct {
	Enabled bool `json:"enabled"`
}

type SettingsRolesConfigPayload struct {
	Coordinator SettingsCoordinatorRoleConfigPayload `json:"coordinator"`
	AutoTitle   SettingsRoleConfigPayload            `json:"auto_title"`
}

type SettingsRoleConfigPayload struct {
	Enabled         bool                          `json:"enabled"`
	Agent           string                        `json:"agent"`
	Provider        string                        `json:"provider"`
	Model           string                        `json:"model"`
	ReasoningEffort string                        `json:"reasoning_effort"`
	Speed           *Speed                        `json:"speed,omitzero"`
	ACPOptions      []AgentACPOptionSelection     `json:"acp_options"`
	FallbackChain   []SettingsRoleFallbackPayload `json:"fallback_chain"`
}

type SettingsCoordinatorRoleConfigPayload struct {
	SettingsRoleConfigPayload
	TTL                           string `json:"ttl"`
	MaxChildren                   int    `json:"max_children"`
	MaxActiveSessionsPerWorkspace int    `json:"max_active_sessions_per_workspace"`
}

type SettingsRoleFallbackPayload struct {
	Provider        string                    `json:"provider"`
	Model           string                    `json:"model"`
	ReasoningEffort string                    `json:"reasoning_effort"`
	Speed           *Speed                    `json:"speed,omitzero"`
	ACPOptions      []AgentACPOptionSelection `json:"acp_options"`
	// Command is the route account; always present on the settings surface, empty inherits.
	Command string `json:"command"`
}

type SettingsExtensionTrustPayload struct {
	AllowUnverified bool `json:"allow_unverified"`
}

type SettingsExtensionSourcesPayload struct {
	GitHub SettingsExtensionGitHubSourcePayload `json:"github"`
	Git    SettingsExtensionGitSourcePayload    `json:"git"`
}

type SettingsExtensionGitHubSourcePayload struct {
	Enabled bool   `json:"enabled"`
	BaseURL string `json:"base_url"`
}

type SettingsExtensionGitSourcePayload struct {
	Enabled bool `json:"enabled"`
}

type SettingsExtensionDevPayload struct {
	WatchInterval string `json:"watch_interval"`
}

type SettingsSkillsConfigPayload struct {
	Enabled                 bool     `json:"enabled"`
	Sources                 []string `json:"sources"`
	CustomSources           []string `json:"custom_sources"`
	DisabledSkills          []string `json:"disabled_skills,omitempty"`
	PollInterval            string   `json:"poll_interval"`
	AllowedMarketplaceHooks []string `json:"allowed_marketplace_hooks,omitempty"`
}

type SettingsAutomationConfigPayload struct {
	Enabled           bool                            `json:"enabled"`
	Timezone          string                          `json:"timezone"`
	MaxConcurrentJobs int                             `json:"max_concurrent_jobs"`
	DefaultFireLimit  automationmodel.FireLimitConfig `json:"default_fire_limit"`
}

type SettingsObservabilityConfigPayload struct {
	Enabled        bool                                   `json:"enabled"`
	RetentionDays  int                                    `json:"retention_days"`
	MaxGlobalBytes int64                                  `json:"max_global_bytes"`
	Transcripts    SettingsObservabilityTranscriptPayload `json:"transcripts"`
}

type SettingsObservabilityTranscriptPayload struct {
	Enabled            bool  `json:"enabled"`
	SegmentBytes       int   `json:"segment_bytes"`
	MaxBytesPerSession int64 `json:"max_bytes_per_session"`
}

type SettingsExtensionsConfigPayload struct {
	Trust     SettingsExtensionTrustPayload     `json:"trust"`
	Sources   SettingsExtensionSourcesPayload   `json:"sources"`
	Dev       SettingsExtensionDevPayload       `json:"dev"`
	Resources SettingsExtensionResourcesPayload `json:"resources"`
}

type SettingsExtensionResourcesPayload struct {
	AllowedKinds           []string                          `json:"allowed_kinds,omitempty"`
	MaxScope               resources.ResourceScopeKind       `json:"max_scope,omitempty"`
	SnapshotRateLimit      SettingsExtensionRateLimitPayload `json:"snapshot_rate_limit"`
	OperatorWriteRateLimit SettingsExtensionRateLimitPayload `json:"operator_write_rate_limit"`
}

type SettingsExtensionRateLimitPayload struct {
	Requests int    `json:"requests"`
	Window   string `json:"window"`
	Queue    int    `json:"queue"`
}

type SettingsConfigPathsPayload struct {
	HomeDir          string `json:"home_dir"`
	GlobalConfig     string `json:"global_config"`
	GlobalMCPSidecar string `json:"global_mcp_sidecar"`
	LogFile          string `json:"log_file"`
	DaemonInfo       string `json:"daemon_info"`
}

type SettingsDaemonRuntimePayload struct {
	Available      bool       `json:"available"`
	Status         string     `json:"status,omitempty"`
	PID            int        `json:"pid,omitzero"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	UptimeSeconds  int64      `json:"uptime_seconds"`
	Socket         string     `json:"socket,omitempty"`
	HTTPHost       string     `json:"http_host,omitempty"`
	HTTPPort       int        `json:"http_port,omitzero"`
	ActiveSessions int        `json:"active_sessions"`
	ActiveAgents   int        `json:"active_agents"`
	TotalSessions  int        `json:"total_sessions"`
	Version        string     `json:"version,omitempty"`
}

type SettingsAutomationRuntimePayload struct {
	Available        bool       `json:"available"`
	Running          bool       `json:"running"`
	SchedulerRunning bool       `json:"scheduler_running"`
	JobTotal         int        `json:"job_total"`
	JobEnabled       int        `json:"job_enabled"`
	TriggerTotal     int        `json:"trigger_total"`
	TriggerEnabled   int        `json:"trigger_enabled"`
	NextFire         *time.Time `json:"next_fire,omitempty"`
	LastSyncedAt     *time.Time `json:"last_synced_at,omitempty"`
}

type SettingsObservabilityRuntimePayload struct {
	Available          bool   `json:"available"`
	Status             string `json:"status,omitempty"`
	GlobalDBSizeBytes  int64  `json:"global_db_size_bytes"`
	SessionDBSizeBytes int64  `json:"session_db_size_bytes"`
	ActiveSessions     int    `json:"active_sessions"`
	ActiveAgents       int    `json:"active_agents"`
	UptimeSeconds      int64  `json:"uptime_seconds"`
}

type SettingsLogTailCapabilityPayload struct {
	Available bool                    `json:"available"`
	StreamURL string                  `json:"stream_url,omitempty"`
	Transport SettingsStreamTransport `json:"transport,omitempty"`
}

type SettingsActionMetadataPayload struct {
	Name      string                   `json:"name"`
	Available bool                     `json:"available"`
	Behavior  SettingsMutationBehavior `json:"behavior"`
}

type SettingsGeneralActionsPayload struct {
	Restart SettingsActionMetadataPayload `json:"restart"`
}

type SettingsOperationalLinkPayload struct {
	Label string `json:"label"`
	Path  string `json:"path"`
}

type SettingsTransportParityPayload struct {
	Known          bool `json:"known"`
	SettingsHTTP   bool `json:"settings_http"`
	SettingsUDS    bool `json:"settings_uds"`
	ExtensionsHTTP bool `json:"extensions_http"`
	ExtensionsUDS  bool `json:"extensions_uds"`
}

type SettingsInstalledExtensionPayload struct {
	Name          string                                    `json:"name"`
	Version       string                                    `json:"version,omitempty"`
	Enabled       bool                                      `json:"enabled"`
	State         string                                    `json:"state,omitempty"`
	Health        string                                    `json:"health,omitempty"`
	HealthMessage string                                    `json:"health_message,omitempty"`
	LastError     string                                    `json:"last_error,omitempty"`
	RequiresEnv   []string                                  `json:"requires_env,omitempty"`
	MissingEnv    []string                                  `json:"missing_env,omitempty"`
	Palette       *SettingsInstalledExtensionPalettePayload `json:"palette,omitzero"`
}

type SettingsInstalledExtensionPalettePayload struct {
	Commands []SettingsInstalledExtensionPaletteCommandPayload `json:"commands"`
	Views    []SettingsInstalledExtensionPaletteViewPayload    `json:"views"`
}

type SettingsInstalledExtensionPaletteCommandPayload struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Bindings       []string `json:"bindings"`
	DefaultBinding string   `json:"default_binding,omitempty"`
	DefaultDormant bool     `json:"default_dormant"`
	ConflictWith   string   `json:"conflict_with,omitempty"`
	Available      bool     `json:"available"`
	Reason         string   `json:"reason,omitempty"`
}

type SettingsInstalledExtensionPaletteViewPayload struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}
