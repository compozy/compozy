package config

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"time"

	"github.com/compozy/compozy/internal/resources"
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
	Agent           *string `toml:"agent"`
	Provider        *string `toml:"provider"`
	Model           *string `toml:"model"`
	ReasoningEffort *string `toml:"reasoning_effort"`
}

type agentsOverlay struct {
	AlwaysOn alwaysOnOverlay `toml:"always_on"`
	// StaysOnAlias decodes the v0.5.0 [agents.stays_on] table. It is applied before AlwaysOn so
	// per-key new values win within a layer. Removal: v0.5.2.
	StaysOnAlias alwaysOnOverlay                 `toml:"stays_on"`
	Soul         soulOverlay                     `toml:"soul"`
	Heartbeat    heartbeatOverlay                `toml:"heartbeat"`
	Overrides    map[string]agentOverrideOverlay `toml:"overrides"`
}

type soulOverlay struct {
	Enabled                *bool  `toml:"enabled"`
	MaxBodyBytes           *int64 `toml:"max_body_bytes"`
	ContextProjectionBytes *int64 `toml:"context_projection_bytes"`
}

type heartbeatOverlay struct {
	Sources                      heartbeatSourcesOverlay `toml:"sources"`
	TriageModels                 map[string]string       `toml:"triage_models"`
	MaxInterval                  *time.Duration          `toml:"max_interval"`
	MaxConcurrentTriage          *int                    `toml:"max_concurrent_triage"`
	QuietBackoffCap              *time.Duration          `toml:"quiet_backoff_cap"`
	FatigueLimit                 *int                    `toml:"fatigue_limit"`
	Enabled                      *bool                   `toml:"enabled"`
	MaxBodyBytes                 *int64                  `toml:"max_body_bytes"`
	ContextProjectionBytes       *int64                  `toml:"context_projection_bytes"`
	MinInterval                  *time.Duration          `toml:"min_interval"`
	DefaultInterval              *time.Duration          `toml:"default_interval"`
	WakeCooldown                 *time.Duration          `toml:"wake_cooldown"`
	MaxWakesPerCycle             *int                    `toml:"max_wakes_per_cycle"`
	ActiveSessionOnly            *bool                   `toml:"active_session_only"`
	AllowActiveHoursPreferences  *bool                   `toml:"allow_active_hours_preferences"`
	WakeEventRetention           *time.Duration          `toml:"wake_event_retention"`
	SessionHealthStaleAfter      *time.Duration          `toml:"session_health_stale_after"`
	SessionHealthHookMinInterval *time.Duration          `toml:"session_health_hook_min_interval"`
}

type limitsOverlay struct {
	MaxConcurrentAgents *int `toml:"max_concurrent_agents"`
}

type sessionOverlay struct {
	Restart      sessionRestartOverlay      `toml:"restart"`
	AuthRecovery sessionAuthRecoveryOverlay `toml:"auth_recovery"`
	Stop         sessionStopOverlay         `toml:"stop"`
	Limits       sessionLimitsOverlay       `toml:"limits"`
	Supervision  sessionSupervisionOverlay  `toml:"supervision"`
	BusyInput    sessionBusyInputOverlay    `toml:"busy_input"`
	Messaging    sessionMessagingOverlay    `toml:"messaging"`
	Attachments  sessionAttachmentsOverlay  `toml:"attachments"`
	Derive       sessionDeriveOverlay       `toml:"derive"`
}

type sessionStopOverlay struct {
	CooperativeGrace *time.Duration `toml:"cooperative_grace"`
}

type sessionLimitsOverlay struct {
	Timeout *time.Duration `toml:"timeout"`
}

type sessionSupervisionOverlay struct {
	QuietAfter *time.Duration `toml:"quiet_after"`
	StopGrace  *time.Duration `toml:"stop_grace"`
	// IdleReleaseAfter is independent of the quiet watchdog keys.
	IdleReleaseAfter *time.Duration `toml:"idle_release_after"`
	// Released keys are translated only at load; remove in v0.5.0.
	ActivityHeartbeatInterval *time.Duration `toml:"activity_heartbeat_interval"`
	ProgressNotifyInterval    *time.Duration `toml:"progress_notify_interval"`
	PromptDeadline            *time.Duration `toml:"prompt_deadline"`
	InactivityWarningAfter    *time.Duration `toml:"inactivity_warning_after"`
	InactivityTimeout         *time.Duration `toml:"inactivity_timeout"`
	TimeoutCancelGrace        *time.Duration `toml:"timeout_cancel_grace"`
}

type sessionBusyInputOverlay struct {
	SteerBoundaryTimeout *time.Duration `toml:"steer_boundary_timeout"`
	DefaultMode          *string        `toml:"default_mode"`
	QueueCap             *int           `toml:"queue_cap"`
	MaxTextBytes         *int           `toml:"max_text_bytes"`
}

type permissionsOverlay struct {
	Mode               *PermissionMode `toml:"mode"`
	ProviderFullAccess *bool           `toml:"provider_full_access"`
}

type providerOverlay struct {
	ReviewerMode        *string                     `toml:"reviewer_mode"`
	Command             *string                     `toml:"command"`
	SessionEnv          SessionEnv                  `toml:"session_env"`
	SteerCapability     *SteerCapability            `toml:"steer_capability"`
	DisplayName         *string                     `toml:"display_name"`
	Models              *providerModelsOverlay      `toml:"models"`
	Harness             *ProviderHarness            `toml:"harness"`
	RuntimeProvider     *string                     `toml:"runtime_provider"`
	Transport           *string                     `toml:"transport"`
	BaseURL             *string                     `toml:"base_url"`
	AuthMode            *ProviderAuthMode           `toml:"auth_mode"`
	EnvPolicy           *ProviderEnvPolicy          `toml:"env_policy"`
	HomePolicy          *ProviderHomePolicy         `toml:"home_policy"`
	NoneSecurity        *ProviderNoneSecurity       `toml:"none_security"`
	AuthStatusCmd       *string                     `toml:"auth_status_command"`
	AuthLoginCmd        *string                     `toml:"auth_login_command"`
	AuthACPMethod       *string                     `toml:"auth_acp_method"`
	SessionMCP          *bool                       `toml:"session_mcp"`
	SessionMCPTransport *MCPServerTransport         `toml:"session_mcp_transport"`
	CredentialSlots     []providerCredentialOverlay `toml:"credential_slots"`
	MCPServers          []mcpServerOverlay          `toml:"mcp_servers"`
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

type skillsOverlay struct {
	Enabled                 *bool          `toml:"enabled"`
	Sources                 *[]string      `toml:"sources"`
	CustomSources           *[]string      `toml:"custom_sources"`
	DisabledSkills          *[]string      `toml:"disabled_skills"`
	PollInterval            *time.Duration `toml:"poll_interval"`
	AllowedMarketplaceHooks *[]string      `toml:"allowed_marketplace_hooks"`
}

type extensionsOverlay struct {
	Trust     extensionsTrustOverlay     `toml:"trust"`
	Sources   extensionsSourcesOverlay   `toml:"sources"`
	Dev       extensionsDevOverlay       `toml:"dev"`
	Resources extensionsResourcesOverlay `toml:"resources"`
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

type hooksOverlay struct {
	Declarations []parsedHookDeclaration `toml:"declarations"`
}

type mcpAuthOverlay struct {
	Registration    *MCPAuthRegistration `toml:"registration"`
	IssuerURL       *string              `toml:"issuer_url"`
	ClientID        *string              `toml:"client_id"`
	ClientSecretRef *string              `toml:"client_secret_ref"`
	Scopes          *[]string            `toml:"scopes"`
}

func applyOptional[T any](source *T, target *T) {
	if source != nil {
		*target = *source
	}
}

// keys lists the TOML keys an overlay sets, in declaration order.
func (o alwaysOnOverlay) keys() []string {
	var keys []string
	for _, field := range []struct {
		set bool
		key string
	}{
		{o.UnattendedTurnLimit != nil, "unattended_turn_limit"},
		{o.RotationThreshold != nil, "rotation_threshold"},
		{o.ApprovalTimeout != nil, "approval_timeout"},
		{o.MaxProposals != nil, "max_proposals"},
		{o.TriageTimeout != nil, "triage_timeout"},
		{o.MaxActiveInstances != nil, "max_active_instances"},
	} {
		if field.set {
			keys = append(keys, field.key)
		}
	}
	return keys
}

// applyAlwaysOn applies one layer's v0.5.0 alias table, then its current table, so per-key new
// values win within the layer. Removal of the alias: v0.5.2.
func (o agentsOverlay) applyAlwaysOn(dst *AlwaysOnConfig) {
	warnStaysOnAlias(o.StaysOnAlias, o.AlwaysOn)
	o.StaysOnAlias.Apply(dst)
	o.AlwaysOn.Apply(dst)
}

// configTableAliasWarnings holds configTableAliasKey once the config-table warning has fired.
var configTableAliasWarnings sync.Map

const configTableAliasKey = "agents.stays_on"

// warnStaysOnAlias warns once per process for the first layer that carries [agents.stays_on] keys
// (Business Rule 5). ignored lists keys that layer's [agents.always_on] overrides. Removal: v0.5.2.
func warnStaysOnAlias(alias alwaysOnOverlay, current alwaysOnOverlay) {
	keys := alias.keys()
	if len(keys) == 0 {
		return
	}
	set := current.keys()
	var ignored []string
	for _, key := range keys {
		if slices.Contains(set, key) {
			ignored = append(ignored, key)
		}
	}
	attrs := []any{"keys", strings.Join(keys, ",")}
	if len(ignored) > 0 {
		attrs = append(attrs, "ignored", strings.Join(ignored, ","))
	}
	if _, loaded := configTableAliasWarnings.LoadOrStore(configTableAliasKey, struct{}{}); loaded {
		return
	}
	DeprecationLogger().Warn(ConfigPathAliasWarning, attrs...)
}
