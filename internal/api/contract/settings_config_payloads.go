package contract

import (
	"time"

	automationmodel "github.com/compozy/agh/internal/automation/model"
	"github.com/compozy/agh/internal/resources"
)

type SettingsGlobalSectionResponseMetaPayload struct {
	Section         SettingsSectionName       `json:"section"`
	Scope           SettingsGlobalScopeKind   `json:"scope"`
	AvailableScopes []SettingsGlobalScopeKind `json:"available_scopes"`
}

type SettingsSkillsSectionResponseMetaPayload struct {
	Section         SettingsSectionName      `json:"section"`
	Scope           SettingsAgentScopeKind   `json:"scope"`
	WorkspaceID     string                   `json:"workspace_id,omitempty"`
	AgentName       string                   `json:"agent_name,omitempty"`
	AvailableScopes []SettingsAgentScopeKind `json:"available_scopes"`
}

type SettingsGlobalCollectionResponseMetaPayload struct {
	Collection      SettingsCollectionName    `json:"collection"`
	Scope           SettingsGlobalScopeKind   `json:"scope"`
	AvailableScopes []SettingsGlobalScopeKind `json:"available_scopes"`
}

type SettingsGlobalWorkspaceCollectionResponseMetaPayload struct {
	Collection      SettingsCollectionName       `json:"collection"`
	Scope           SettingsWorkspaceScopeKind   `json:"scope"`
	WorkspaceID     string                       `json:"workspace_id,omitempty"`
	AvailableScopes []SettingsWorkspaceScopeKind `json:"available_scopes"`
}

type SettingsGeneralConfigPayload struct {
	Defaults       SettingsDefaultsPayload    `json:"defaults"`
	Limits         SettingsLimitsPayload      `json:"limits"`
	Permissions    SettingsPermissionsPayload `json:"permissions"`
	SessionTimeout string                     `json:"session_timeout"`
	HTTP           SettingsHTTPPayload        `json:"http"`
	Daemon         SettingsDaemonPayload      `json:"daemon"`
	Redact         SettingsRedactPayload      `json:"redact"`
}

type SettingsDefaultsPayload struct {
	Agent    string `json:"agent"`
	Provider string `json:"provider,omitempty"`
	Sandbox  string `json:"sandbox,omitempty"`
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
	Bridges   string `json:"bridges"`
}

type SettingsRedactPayload struct {
	Enabled bool `json:"enabled"`
}

type SettingsMemoryConfigPayload struct {
	Enabled    bool                            `json:"enabled"`
	GlobalDir  string                          `json:"global_dir,omitempty"`
	Controller SettingsMemoryControllerPayload `json:"controller"`
	Recall     SettingsMemoryRecallPayload     `json:"recall"`
	Decisions  SettingsMemoryDecisionsPayload  `json:"decisions"`
	Extractor  SettingsMemoryExtractorPayload  `json:"extractor"`
	Dream      SettingsMemoryDreamPayload      `json:"dream"`
	Session    SettingsMemorySessionPayload    `json:"session"`
	Daily      SettingsMemoryDailyPayload      `json:"daily"`
	File       SettingsMemoryFilePayload       `json:"file"`
	Provider   SettingsMemoryProviderPayload   `json:"provider"`
	Workspace  SettingsMemoryWorkspacePayload  `json:"workspace"`
}

type SettingsMemoryControllerPayload struct {
	Mode            string                                `json:"mode"`
	MaxLatency      string                                `json:"max_latency"`
	DefaultOpOnFail string                                `json:"default_op_on_fail"`
	Policy          SettingsMemoryControllerPolicyPayload `json:"policy"`
}

type SettingsMemoryControllerPolicyPayload struct {
	MaxContentChars int      `json:"max_content_chars"`
	MaxWritesPerMin int      `json:"max_writes_per_min"`
	AllowOrigins    []string `json:"allow_origins"`
}

type SettingsMemoryRecallPayload struct {
	TopK                   int                                  `json:"top_k"`
	RawCandidates          int                                  `json:"raw_candidates"`
	Fusion                 string                               `json:"fusion"`
	IncludeAlreadySurfaced bool                                 `json:"include_already_surfaced"`
	IncludeSystem          bool                                 `json:"include_system"`
	Weights                SettingsMemoryRecallWeightsPayload   `json:"weights"`
	Freshness              SettingsMemoryRecallFreshnessPayload `json:"freshness"`
	Signals                SettingsMemoryRecallSignalsPayload   `json:"signals"`
}

type SettingsMemoryRecallWeightsPayload struct {
	BM25Unicode  float64 `json:"bm25_unicode"`
	BM25Trigram  float64 `json:"bm25_trigram"`
	Recency      float64 `json:"recency"`
	RecallSignal float64 `json:"recall_signal"`
}

type SettingsMemoryRecallFreshnessPayload struct {
	BannerAfterDays int `json:"banner_after_days"`
}

type SettingsMemoryRecallSignalsPayload struct {
	QueueCapacity  int  `json:"queue_capacity"`
	WorkerRetryMax int  `json:"worker_retry_max"`
	MetricsEnabled bool `json:"metrics_enabled"`
}

type SettingsMemoryDecisionsPayload struct {
	PruneAfterAppliedDays int   `json:"prune_after_applied_days"`
	KeepAuditSummary      bool  `json:"keep_audit_summary"`
	MaxPostContentBytes   int64 `json:"max_post_content_bytes"`
}

type SettingsMemoryExtractorPayload struct {
	Mode             string                              `json:"mode"`
	ThrottleTurns    int                                 `json:"throttle_turns"`
	Deadline         string                              `json:"deadline"`
	SandboxInboxOnly bool                                `json:"sandbox_inbox_only"`
	InboxPath        string                              `json:"inbox_path"`
	DLQPath          string                              `json:"dlq_path"`
	Queue            SettingsMemoryExtractorQueuePayload `json:"queue"`
}

type SettingsMemoryExtractorQueuePayload struct {
	Capacity    int `json:"capacity"`
	CoalesceMax int `json:"coalesce_max"`
}

type SettingsMemoryDreamPayload struct {
	MinHours      float64                           `json:"min_hours"`
	MinSessions   int                               `json:"min_sessions"`
	Debounce      string                            `json:"debounce"`
	PromptVersion string                            `json:"prompt_version"`
	CheckInterval string                            `json:"check_interval"`
	Gates         SettingsMemoryDreamGatesPayload   `json:"gates"`
	Scoring       SettingsMemoryDreamScoringPayload `json:"scoring"`
}

type SettingsRolesConfigPayload struct {
	Coordinator       SettingsCoordinatorRoleConfigPayload      `json:"coordinator"`
	Dream             SettingsRoleConfigPayload                 `json:"dream"`
	CheckpointSummary SettingsRoleConfigPayload                 `json:"checkpoint_summary"`
	MemoryExtractor   SettingsRoleConfigPayload                 `json:"memory_extractor"`
	AutoTitle         SettingsRoleConfigPayload                 `json:"auto_title"`
	MemoryController  SettingsMemoryControllerRoleConfigPayload `json:"memory_controller"`
}

type SettingsRoleConfigPayload struct {
	Enabled         bool                          `json:"enabled"`
	Agent           string                        `json:"agent"`
	Provider        string                        `json:"provider"`
	Model           string                        `json:"model"`
	ReasoningEffort string                        `json:"reasoning_effort"`
	FallbackChain   []SettingsRoleFallbackPayload `json:"fallback_chain"`
}

type SettingsCoordinatorRoleConfigPayload struct {
	SettingsRoleConfigPayload
	TTL                           string `json:"ttl"`
	MaxChildren                   int    `json:"max_children"`
	MaxActiveSessionsPerWorkspace int    `json:"max_active_sessions_per_workspace"`
}

type SettingsMemoryControllerRoleConfigPayload struct {
	Enabled         bool                          `json:"enabled"`
	Provider        string                        `json:"provider"`
	Model           string                        `json:"model"`
	ReasoningEffort string                        `json:"reasoning_effort"`
	Timeout         string                        `json:"timeout"`
	TopK            int                           `json:"top_k"`
	PromptVersion   string                        `json:"prompt_version"`
	MaxTokensOut    int                           `json:"max_tokens_out"`
	FallbackChain   []SettingsRoleFallbackPayload `json:"fallback_chain"`
}

type SettingsRoleFallbackPayload struct {
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort"`
}

type SettingsMemoryDreamGatesPayload struct {
	MinUnpromoted  int     `json:"min_unpromoted"`
	MinRecallCount int     `json:"min_recall_count"`
	MinScore       float64 `json:"min_score"`
}

type SettingsMemoryDreamScoringPayload struct {
	RecencyHalfLifeDays int                                      `json:"recency_half_life_days"`
	Weights             SettingsMemoryDreamScoringWeightsPayload `json:"weights"`
}

type SettingsMemoryDreamScoringWeightsPayload struct {
	Frequency float64 `json:"frequency"`
	Relevance float64 `json:"relevance"`
	Recency   float64 `json:"recency"`
	Freshness float64 `json:"freshness"`
}

type SettingsMemorySessionPayload struct {
	LedgerFormat     string `json:"ledger_format"`
	LedgerRoot       string `json:"ledger_root"`
	EventsPurgeGrace string `json:"events_purge_grace"`
	ColdArchiveDays  int    `json:"cold_archive_days"`
	HardDeleteDays   int    `json:"hard_delete_days"`
	MaxArchiveBytes  int64  `json:"max_archive_bytes"`
	UnboundPartition string `json:"unbound_partition"`
}

type SettingsMemoryDailyPayload struct {
	MaxBytes        int64  `json:"max_bytes"`
	MaxLines        int    `json:"max_lines"`
	RotateFormat    string `json:"rotate_format"`
	DreamingWindow  int    `json:"dreaming_window"`
	ColdArchiveDays int    `json:"cold_archive_days"`
	HardDeleteDays  int    `json:"hard_delete_days"`
	MaxArchiveBytes int64  `json:"max_archive_bytes"`
	SweepHour       int    `json:"sweep_hour"`
	ArchivePath     string `json:"archive_path"`
}

type SettingsMemoryFilePayload struct {
	MaxLines int   `json:"max_lines"`
	MaxBytes int64 `json:"max_bytes"`
}

type SettingsMemoryProviderPayload struct {
	Name             string `json:"name"`
	Timeout          string `json:"timeout"`
	FailureThreshold int    `json:"failure_threshold"`
	Cooldown         string `json:"cooldown"`
}

type SettingsMemoryWorkspacePayload struct {
	TOMLPath   string `json:"toml_path"`
	AutoCreate bool   `json:"auto_create"`
}

type SettingsMarketplacePayload struct {
	Registry string `json:"registry"`
	BaseURL  string `json:"base_url,omitempty"`
}

type SettingsExtensionMarketplacePayload struct {
	Registry        string `json:"registry"`
	BaseURL         string `json:"base_url,omitempty"`
	AllowUnverified bool   `json:"allow_unverified"`
}

type SettingsSkillsConfigPayload struct {
	Enabled                 bool                       `json:"enabled"`
	DisabledSkills          []string                   `json:"disabled_skills,omitempty"`
	PollInterval            string                     `json:"poll_interval"`
	AllowedMarketplaceMCP   []string                   `json:"allowed_marketplace_mcp,omitempty"`
	AllowedMarketplaceHooks []string                   `json:"allowed_marketplace_hooks,omitempty"`
	Marketplace             SettingsMarketplacePayload `json:"marketplace"`
}

type SettingsAutomationConfigPayload struct {
	Enabled           bool                            `json:"enabled"`
	Timezone          string                          `json:"timezone"`
	MaxConcurrentJobs int                             `json:"max_concurrent_jobs"`
	DefaultFireLimit  automationmodel.FireLimitConfig `json:"default_fire_limit"`
}

type SettingsNetworkConfigPayload struct {
	Enabled      bool                             `json:"enabled"`
	MaxReplayAge int                              `json:"max_replay_age"`
	Live         SettingsNetworkLiveConfigPayload `json:"live"`
}

type SettingsNetworkLiveConfigPayload struct {
	Defaults SettingsNetworkLiveDefaultsPayload `json:"defaults"`
	Limits   SettingsNetworkLiveLimitsPayload   `json:"limits"`
}

type SettingsNetworkLiveDefaultsPayload struct {
	MaxWakes         int    `json:"max_wakes"`
	MaxWakeWallTime  string `json:"max_wake_wall_time"`
	MaxTotalWallTime string `json:"max_total_wall_time"`
	MaxInputTokens   int64  `json:"max_input_tokens"`
	MaxOutputTokens  int64  `json:"max_output_tokens"`
	MaxWakeDepth     int    `json:"max_wake_depth"`
	CoalesceWindow   string `json:"coalesce_window"`
}

type SettingsNetworkLiveLimitsPayload struct {
	MaxWakes          int    `json:"max_wakes"`
	MaxWakeWallTime   string `json:"max_wake_wall_time"`
	MaxTotalWallTime  string `json:"max_total_wall_time"`
	MaxInputTokens    int64  `json:"max_input_tokens"`
	MaxOutputTokens   int64  `json:"max_output_tokens"`
	MaxWakeDepth      int    `json:"max_wake_depth"`
	MinCoalesceWindow string `json:"min_coalesce_window"`
	MaxCoalesceWindow string `json:"max_coalesce_window"`
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
	Marketplace SettingsExtensionMarketplacePayload `json:"marketplace"`
	Resources   SettingsExtensionResourcesPayload   `json:"resources"`
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
	PID            int        `json:"pid,omitempty"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	UptimeSeconds  int64      `json:"uptime_seconds"`
	Socket         string     `json:"socket,omitempty"`
	HTTPHost       string     `json:"http_host,omitempty"`
	HTTPPort       int        `json:"http_port,omitempty"`
	ActiveSessions int        `json:"active_sessions"`
	ActiveAgents   int        `json:"active_agents"`
	TotalSessions  int        `json:"total_sessions"`
	Version        string     `json:"version,omitempty"`
}

type SettingsMemoryHealthPayload struct {
	Available          bool       `json:"available"`
	FileCount          int        `json:"file_count"`
	DreamEnabled       bool       `json:"dream_enabled"`
	LastConsolidatedAt *time.Time `json:"last_consolidated_at,omitempty"`
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

type SettingsNetworkRuntimePayload struct {
	Available         bool   `json:"available"`
	Enabled           bool   `json:"enabled"`
	Status            string `json:"status,omitempty"`
	LocalPeers        int    `json:"local_peers"`
	Channels          int    `json:"channels"`
	MessagesReceived  int64  `json:"messages_received"`
	MessagesDelivered int64  `json:"messages_delivered"`
	MessagesRejected  int64  `json:"messages_rejected"`
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

type SettingsMemoryActionsPayload struct {
	Consolidate SettingsActionMetadataPayload `json:"consolidate"`
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
	Name          string   `json:"name"`
	Version       string   `json:"version,omitempty"`
	Enabled       bool     `json:"enabled"`
	State         string   `json:"state,omitempty"`
	Health        string   `json:"health,omitempty"`
	HealthMessage string   `json:"health_message,omitempty"`
	LastError     string   `json:"last_error,omitempty"`
	RequiresEnv   []string `json:"requires_env,omitempty"`
	MissingEnv    []string `json:"missing_env,omitempty"`
}

type SettingsSourceRefPayload struct {
	Kind        SettingsSourceKind `json:"kind"`
	Scope       SettingsScopeKind  `json:"scope"`
	WorkspaceID string             `json:"workspace_id,omitempty"`
	AgentName   string             `json:"agent_name,omitempty"`
}

type SettingsSourceMetadataPayload struct {
	EffectiveSource  SettingsSourceRefPayload   `json:"effective_source"`
	ShadowedSources  []SettingsSourceRefPayload `json:"shadowed_sources,omitempty"`
	AvailableTargets []SettingsWriteTargetKind  `json:"available_targets"`
}
