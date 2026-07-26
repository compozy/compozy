package config

import (
	"fmt"

	"time"

	"github.com/compozy/agh/internal/resources"
)

const (
	mergeReadKey = "read"
)

const providersConfigKey = "providers"

// FileError preserves the source file for configuration read/decode failures.
type FileError struct {
	Op   string
	Path string
	Err  error
}

func (e FileError) Error() string {
	return fmt.Sprintf("%s config file %q: %v", e.Op, e.Path, e.Err)
}

func (e FileError) Unwrap() error {
	return e.Err
}

type httpOverlay struct {
	Host *string `toml:"host"`
	Port *int    `toml:"port"`
}

type defaultsOverlay struct {
	Agent    *string `toml:"agent"`
	Provider *string `toml:"provider"`
	Sandbox  *string `toml:"sandbox"`
}

type agentsOverlay struct {
	Soul      soulOverlay      `toml:"soul"`
	Heartbeat heartbeatOverlay `toml:"heartbeat"`
}

type soulOverlay struct {
	Enabled                *bool  `toml:"enabled"`
	MaxBodyBytes           *int64 `toml:"max_body_bytes"`
	ContextProjectionBytes *int64 `toml:"context_projection_bytes"`
}

type heartbeatOverlay struct {
	Enabled                      *bool          `toml:"enabled"`
	MaxBodyBytes                 *int64         `toml:"max_body_bytes"`
	ContextProjectionBytes       *int64         `toml:"context_projection_bytes"`
	MinInterval                  *time.Duration `toml:"min_interval"`
	DefaultInterval              *time.Duration `toml:"default_interval"`
	WakeCooldown                 *time.Duration `toml:"wake_cooldown"`
	MaxWakesPerCycle             *int           `toml:"max_wakes_per_cycle"`
	ActiveSessionOnly            *bool          `toml:"active_session_only"`
	AllowActiveHoursPreferences  *bool          `toml:"allow_active_hours_preferences"`
	WakeEventRetention           *time.Duration `toml:"wake_event_retention"`
	SessionHealthStaleAfter      *time.Duration `toml:"session_health_stale_after"`
	SessionHealthHookMinInterval *time.Duration `toml:"session_health_hook_min_interval"`
}

type limitsOverlay struct {
	MaxConcurrentAgents *int `toml:"max_concurrent_agents"`
}

type sessionOverlay struct {
	Limits      sessionLimitsOverlay      `toml:"limits"`
	Supervision sessionSupervisionOverlay `toml:"supervision"`
	BusyInput   sessionBusyInputOverlay   `toml:"busy_input"`
	Compaction  sessionCompactionOverlay  `toml:"compaction"`
}

type sessionLimitsOverlay struct {
	Timeout *time.Duration `toml:"timeout"`
}

type sessionSupervisionOverlay struct {
	ActivityHeartbeatInterval *time.Duration `toml:"activity_heartbeat_interval"`
	ProgressNotifyInterval    *time.Duration `toml:"progress_notify_interval"`
	PromptDeadline            *time.Duration `toml:"prompt_deadline"`
	InactivityWarningAfter    *time.Duration `toml:"inactivity_warning_after"`
	InactivityTimeout         *time.Duration `toml:"inactivity_timeout"`
	TimeoutCancelGrace        *time.Duration `toml:"timeout_cancel_grace"`
}

type sessionBusyInputOverlay struct {
	DefaultMode  *string `toml:"default_mode"`
	QueueCap     *int    `toml:"queue_cap"`
	MaxTextBytes *int    `toml:"max_text_bytes"`
}

type sessionCompactionOverlay struct {
	Enabled            *bool          `toml:"enabled"`
	PressureThreshold  *float64       `toml:"pressure_threshold"`
	MaxAttemptsPerTurn *int           `toml:"max_attempts_per_turn"`
	FailureCooldown    *time.Duration `toml:"failure_cooldown"`
}

type permissionsOverlay struct {
	Mode *PermissionMode `toml:"mode"`
}

type providerOverlay struct {
	Command         *string                     `toml:"command"`
	DisplayName     *string                     `toml:"display_name"`
	Models          *providerModelsOverlay      `toml:"models"`
	Harness         *ProviderHarness            `toml:"harness"`
	RuntimeProvider *string                     `toml:"runtime_provider"`
	Transport       *string                     `toml:"transport"`
	BaseURL         *string                     `toml:"base_url"`
	AuthMode        *ProviderAuthMode           `toml:"auth_mode"`
	EnvPolicy       *ProviderEnvPolicy          `toml:"env_policy"`
	HomePolicy      *ProviderHomePolicy         `toml:"home_policy"`
	NoneSecurity    *ProviderNoneSecurity       `toml:"none_security"`
	AuthStatusCmd   *string                     `toml:"auth_status_command"`
	AuthLoginCmd    *string                     `toml:"auth_login_command"`
	SessionMCP      *bool                       `toml:"session_mcp"`
	CredentialSlots []providerCredentialOverlay `toml:"credential_slots"`
	MCPServers      []mcpServerOverlay          `toml:"mcp_servers"`
}

type providerModelsOverlay struct {
	Default   *string                        `toml:"default"`
	Curated   []ProviderModelConfig          `toml:"curated"`
	Discovery providerModelsDiscoveryOverlay `toml:"discovery"`
	Reasoning providerReasoningOverlay       `toml:"reasoning"`
}

type modelCatalogOverlay struct {
	Sources modelCatalogSourcesOverlay `toml:"sources"`
}

type modelCatalogSourcesOverlay struct {
	ModelsDev modelsDevSourceOverlay `toml:"models_dev"`
}

type modelsDevSourceOverlay struct {
	Enabled  *bool   `toml:"enabled"`
	Endpoint *string `toml:"endpoint"`
	TTL      *string `toml:"ttl"`
	Timeout  *string `toml:"timeout"`
}

type providerCredentialOverlay struct {
	Name      *string `toml:"name"`
	TargetEnv *string `toml:"target_env"`
	SecretRef *string `toml:"secret_ref"`
	Kind      *string `toml:"kind"`
	Required  *bool   `toml:"required"`
}

type sandboxOverlay struct {
	Backend     *string               `toml:"backend"`
	SyncMode    *string               `toml:"sync_mode"`
	Persistence *string               `toml:"persistence"`
	RuntimeRoot *string               `toml:"runtime_root"`
	Env         *map[string]string    `toml:"env"`
	SecretEnv   *map[string]string    `toml:"secret_env"`
	Network     networkProfileOverlay `toml:"network"`
	Daytona     daytonaProfileOverlay `toml:"daytona"`
}

type networkProfileOverlay struct {
	AllowPublicIngress *bool     `toml:"allow_public_ingress"`
	AllowOutbound      *bool     `toml:"allow_outbound"`
	AllowList          *[]string `toml:"allow_list"`
	DenyList           *[]string `toml:"deny_list"`
	Required           *bool     `toml:"required"`
}

type daytonaProfileOverlay struct {
	APIURL      *string `toml:"api_url"`
	Target      *string `toml:"target"`
	Image       *string `toml:"image"`
	Snapshot    *string `toml:"snapshot"`
	Class       *string `toml:"class"`
	AutoStop    *string `toml:"auto_stop"`
	AutoArchive *string `toml:"auto_archive"`
}

type observabilityOverlay struct {
	Enabled           *bool                           `toml:"enabled"`
	RetentionDays     *int                            `toml:"retention_days"`
	MaxGlobalBytes    *int64                          `toml:"max_global_bytes"`
	AgentProbeTimeout *time.Duration                  `toml:"agent_probe_timeout"`
	Transcripts       observabilityTranscriptsOverlay `toml:"transcripts"`
}

type observabilityTranscriptsOverlay struct {
	Enabled            *bool  `toml:"enabled"`
	SegmentBytes       *int   `toml:"segment_bytes"`
	MaxBytesPerSession *int64 `toml:"max_bytes_per_session"`
}

type memoryOverlay struct {
	Enabled    *bool                   `toml:"enabled"`
	GlobalDir  *string                 `toml:"global_dir"`
	Controller memoryControllerOverlay `toml:"controller"`
	Recall     memoryRecallOverlay     `toml:"recall"`
	Decisions  memoryDecisionsOverlay  `toml:"decisions"`
	Extractor  memoryExtractorOverlay  `toml:"extractor"`
	Dream      dreamOverlay            `toml:"dream"`
	Session    memorySessionOverlay    `toml:"session"`
	Daily      memoryDailyOverlay      `toml:"daily"`
	File       memoryFileOverlay       `toml:"file"`
	Provider   memoryProviderOverlay   `toml:"provider"`
	Workspace  memoryWorkspaceOverlay  `toml:"workspace"`
}

type memoryControllerOverlay struct {
	Mode            *string                       `toml:"mode"`
	MaxLatency      *time.Duration                `toml:"max_latency"`
	DefaultOpOnFail *string                       `toml:"default_op_on_fail"`
	Policy          memoryControllerPolicyOverlay `toml:"policy"`
}

type memoryControllerPolicyOverlay struct {
	MaxContentChars *int      `toml:"max_content_chars"`
	MaxWritesPerMin *int      `toml:"max_writes_per_min"`
	AllowOrigins    *[]string `toml:"allow_origins"`
}

type memoryRecallOverlay struct {
	TopK                   *int                         `toml:"top_k"`
	RawCandidates          *int                         `toml:"raw_candidates"`
	Fusion                 *string                      `toml:"fusion"`
	IncludeAlreadySurfaced *bool                        `toml:"include_already_surfaced"`
	IncludeSystem          *bool                        `toml:"include_system"`
	Weights                memoryRecallWeightsOverlay   `toml:"weights"`
	Freshness              memoryRecallFreshnessOverlay `toml:"freshness"`
	Signals                memoryRecallSignalsOverlay   `toml:"signals"`
}

type memoryRecallWeightsOverlay struct {
	BM25Unicode  *float64 `toml:"bm25_unicode"`
	BM25Trigram  *float64 `toml:"bm25_trigram"`
	Recency      *float64 `toml:"recency"`
	RecallSignal *float64 `toml:"recall_signal"`
}

type memoryRecallFreshnessOverlay struct {
	BannerAfterDays *int `toml:"banner_after_days"`
}

type memoryRecallSignalsOverlay struct {
	QueueCapacity  *int  `toml:"queue_capacity"`
	WorkerRetryMax *int  `toml:"worker_retry_max"`
	MetricsEnabled *bool `toml:"metrics_enabled"`
}

type memoryDecisionsOverlay struct {
	PruneAfterAppliedDays *int   `toml:"prune_after_applied_days"`
	KeepAuditSummary      *bool  `toml:"keep_audit_summary"`
	MaxPostContentBytes   *int64 `toml:"max_post_content_bytes"`
}

type memoryExtractorOverlay struct {
	Mode             *string                     `toml:"mode"`
	ThrottleTurns    *int                        `toml:"throttle_turns"`
	Deadline         *time.Duration              `toml:"deadline"`
	SandboxInboxOnly *bool                       `toml:"sandbox_inbox_only"`
	InboxPath        *string                     `toml:"inbox_path"`
	DLQPath          *string                     `toml:"dlq_path"`
	Queue            memoryExtractorQueueOverlay `toml:"queue"`
}

type memoryExtractorQueueOverlay struct {
	Capacity    *int `toml:"capacity"`
	CoalesceMax *int `toml:"coalesce_max"`
}

type dreamOverlay struct {
	MinHours      *float64                  `toml:"min_hours"`
	MinSessions   *int                      `toml:"min_sessions"`
	Debounce      *time.Duration            `toml:"debounce"`
	PromptVersion *string                   `toml:"prompt_version"`
	CheckInterval *time.Duration            `toml:"check_interval"`
	Gates         memoryDreamGatesOverlay   `toml:"gates"`
	Scoring       memoryDreamScoringOverlay `toml:"scoring"`
}

type memoryDreamGatesOverlay struct {
	MinUnpromoted  *int     `toml:"min_unpromoted"`
	MinRecallCount *int     `toml:"min_recall_count"`
	MinScore       *float64 `toml:"min_score"`
}

type memoryDreamScoringOverlay struct {
	RecencyHalfLifeDays *int                             `toml:"recency_half_life_days"`
	Weights             memoryDreamScoringWeightsOverlay `toml:"weights"`
}

type memoryDreamScoringWeightsOverlay struct {
	Frequency *float64 `toml:"frequency"`
	Relevance *float64 `toml:"relevance"`
	Recency   *float64 `toml:"recency"`
	Freshness *float64 `toml:"freshness"`
}

type memorySessionOverlay struct {
	LedgerFormat     *string        `toml:"ledger_format"`
	LedgerRoot       *string        `toml:"ledger_root"`
	EventsPurgeGrace *time.Duration `toml:"events_purge_grace"`
	ColdArchiveDays  *int           `toml:"cold_archive_days"`
	HardDeleteDays   *int           `toml:"hard_delete_days"`
	MaxArchiveBytes  *int64         `toml:"max_archive_bytes"`
	UnboundPartition *string        `toml:"unbound_partition"`
}

type memoryDailyOverlay struct {
	MaxBytes        *int64  `toml:"max_bytes"`
	MaxLines        *int    `toml:"max_lines"`
	RotateFormat    *string `toml:"rotate_format"`
	DreamingWindow  *int    `toml:"dreaming_window"`
	ColdArchiveDays *int    `toml:"cold_archive_days"`
	HardDeleteDays  *int    `toml:"hard_delete_days"`
	MaxArchiveBytes *int64  `toml:"max_archive_bytes"`
	SweepHour       *int    `toml:"sweep_hour"`
	ArchivePath     *string `toml:"archive_path"`
}

type memoryFileOverlay struct {
	MaxLines *int   `toml:"max_lines"`
	MaxBytes *int64 `toml:"max_bytes"`
}

type memoryProviderOverlay struct {
	Name             *string        `toml:"name"`
	Timeout          *time.Duration `toml:"timeout"`
	FailureThreshold *int           `toml:"failure_threshold"`
	Cooldown         *time.Duration `toml:"cooldown"`
}

type memoryWorkspaceOverlay struct {
	TOMLPath   *string `toml:"toml_path"`
	AutoCreate *bool   `toml:"auto_create"`
}

type skillsOverlay struct {
	Enabled                 *bool              `toml:"enabled"`
	DisabledSkills          *[]string          `toml:"disabled_skills"`
	PollInterval            *time.Duration     `toml:"poll_interval"`
	AllowedMarketplaceMCP   *[]string          `toml:"allowed_marketplace_mcp"`
	AllowedMarketplaceHooks *[]string          `toml:"allowed_marketplace_hooks"`
	Marketplace             marketplaceOverlay `toml:"marketplace"`
}

type extensionsOverlay struct {
	Marketplace extensionsMarketplaceOverlay `toml:"marketplace"`
	Resources   extensionsResourcesOverlay   `toml:"resources"`
}

type extensionsResourcesOverlay struct {
	AllowedKinds           *[]resources.ResourceKind    `toml:"allowed_kinds"`
	MaxScope               *resources.ResourceScopeKind `toml:"max_scope"`
	SnapshotRateLimit      extensionsRateLimitOverlay   `toml:"snapshot_rate_limit"`
	OperatorWriteRateLimit extensionsRateLimitOverlay   `toml:"operator_write_rate_limit"`
}

type extensionsRateLimitOverlay struct {
	Requests *int           `toml:"requests"`
	Window   *time.Duration `toml:"window"`
	Queue    *int           `toml:"queue"`
}

type autonomyOverlay struct {
	BlockRecurrenceLimit *int             `toml:"block_recurrence_limit"`
	Scheduler            schedulerOverlay `toml:"scheduler"`
}

type schedulerOverlay struct {
	FanOutAfter         *int           `toml:"fan_out_after"`
	SpawnAfter          *int           `toml:"spawn_after"`
	EventAfter          *int           `toml:"event_after"`
	NeedsAttentionAfter *int           `toml:"needs_attention_after"`
	MinQueuedAge        *time.Duration `toml:"min_queued_age"`
}

type marketplaceOverlay struct {
	Registry *string `toml:"registry"`
	BaseURL  *string `toml:"base_url"`
}

type hooksOverlay struct {
	Declarations []parsedHookDeclaration `toml:"declarations"`
}

type mcpAuthOverlay struct {
	Type             *MCPAuthType `toml:"type"`
	IssuerURL        *string      `toml:"issuer_url"`
	MetadataURL      *string      `toml:"metadata_url"`
	AuthorizationURL *string      `toml:"authorization_url"`
	TokenURL         *string      `toml:"token_url"`
	RevocationURL    *string      `toml:"revocation_url"`
	ClientID         *string      `toml:"client_id"`
	ClientSecretRef  *string      `toml:"client_secret_ref"`
	Scopes           *[]string    `toml:"scopes"`
}
