package cli

import (
	compozyconfig "github.com/compozy/compozy/internal/config"
)

const (
	configManagedValue         = "Managed"
	configManagerValue         = "Manager"
	configPathValue            = "Path"
	configRedactedValue        = "Redacted"
	configScopeValue           = "Scope"
	configStatusValue          = "Status"
	configTargetValue          = "Target"
	configValueValue           = "Value"
	configWorkspaceValue       = "Workspace"
	configBackendKey           = "backend"
	configCommandKey           = "command"
	configReloadCommandName    = "reload"
	configConfigKey            = "config"
	configDaemonKey            = "daemon"
	configDefaultsProviderPath = "defaults.provider"
	configEditKey              = "edit"
	configEnabledKey           = cliEnabledKey
	configInvalidKey           = "invalid"
	configListKey              = "list"
	configManagedKey           = "managed"
	configManagerKey           = "manager"
	configPathKey              = cliPathKey
	configReadKey              = "read"
	configRedactedKey          = "redacted"
	configRequiredKey          = "required"
	configScopeKey             = "scope"
	configShowKey              = "show"
	configSkillsKey            = "skills"
	configStatusKey            = "status"
	configStatusOverridden     = "ok_overridden"
	configTargetKey            = "target"
	configUnsetKey             = "unset"
	configWorkspaceRootKey     = "workspace_root"
)

const (
	configEnvKey        = "env"
	configSecretEnvKey  = "secret_env"
	configProvidersKey  = "providers"
	configModelsKey     = "models"
	configDiscoveryKey  = "discovery"
	configDefaultKey    = "default"
	configSessionMCPKey = "session_mcp"
)

type configEntry struct {
	Path     string `json:"path"`
	Value    any    `json:"value"`
	Redacted bool   `json:"redacted"`
}

type configShowRecord struct {
	Scope         string         `json:"scope"`
	WorkspaceRoot string         `json:"workspace_root,omitempty"`
	Redacted      bool           `json:"redacted"`
	Config        map[string]any `json:"config"`
}

type configListRecord struct {
	Scope         string        `json:"scope"`
	WorkspaceRoot string        `json:"workspace_root,omitempty"`
	Redacted      bool          `json:"redacted"`
	Entries       []configEntry `json:"entries"`
}

type configValueRecord struct {
	Path     string `json:"path"`
	Value    any    `json:"value"`
	Redacted bool   `json:"redacted"`
}

type configSetRecord struct {
	Path             string `json:"path"`
	Value            any    `json:"value"`
	Scope            string `json:"scope"`
	Target           string `json:"target"`
	Redacted         bool   `json:"redacted"`
	Lifecycle        string `json:"lifecycle"`
	ApplyRecordID    string `json:"apply_record_id,omitempty"`
	Applied          bool   `json:"applied"`
	ActiveGeneration int64  `json:"active_generation,omitzero"`
	ActiveConfigHash string `json:"active_config_hash,omitempty"`
	NextAction       string `json:"next_action,omitempty"`
	RestartRequired  bool   `json:"restart_required"`
	RestartScope     string `json:"restart_scope,omitempty"`
	Status           string `json:"status,omitempty"`
	WinningLayer     string `json:"winning_layer,omitempty"`
}

type configUnsetRecord struct {
	Path             string `json:"path"`
	Scope            string `json:"scope"`
	Target           string `json:"target"`
	Deleted          bool   `json:"deleted"`
	Lifecycle        string `json:"lifecycle"`
	ApplyRecordID    string `json:"apply_record_id,omitempty"`
	Applied          bool   `json:"applied"`
	ActiveGeneration int64  `json:"active_generation,omitzero"`
	ActiveConfigHash string `json:"active_config_hash,omitempty"`
	NextAction       string `json:"next_action,omitempty"`
	RestartRequired  bool   `json:"restart_required"`
	RestartScope     string `json:"restart_scope,omitempty"`
}

type configPathRecord struct {
	HomeDir              string `json:"home_dir"`
	GlobalConfig         string `json:"global_config"`
	GlobalMCPJSON        string `json:"global_mcp_json"`
	Scope                string `json:"scope"`
	WorkspaceRoot        string `json:"workspace_root,omitempty"`
	WorkspaceConfig      string `json:"workspace_config,omitempty"`
	WorkspaceMCPJSON     string `json:"workspace_mcp_json,omitempty"`
	Managed              bool   `json:"managed"`
	Manager              string `json:"manager,omitempty"`
	SelectedConfigTarget string `json:"selected_config_target"`
}

type configValidateRecord struct {
	Status        string                            `json:"status"`
	Scope         string                            `json:"scope"`
	WorkspaceRoot string                            `json:"workspace_root,omitempty"`
	ConfigFile    string                            `json:"config_file"`
	Redacted      bool                              `json:"redacted"`
	Errors        []configValidationError           `json:"errors,omitempty"`
	DotEnv        *compozyconfig.DotEnvRepairReport `json:"dot_env,omitzero"`
}

type configValidationError struct {
	Code    string `json:"code"`
	Path    string `json:"path,omitempty"`
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitzero"`
	Column  int    `json:"column,omitzero"`
	Message string `json:"message"`
}

type configValidationFailedError struct {
	err error
}

func (e configValidationFailedError) Error() string {
	return e.err.Error()
}

func (e configValidationFailedError) Unwrap() error {
	return e.err
}

type configSetValueKind int

const (
	configSetString configSetValueKind = iota
	configSetBool
	configSetInt
	configSetInt64
	configSetUint64
	configSetFloat
	configSetDuration
	configSetStringSlice
	configSetFloatSlice
	configSetTable
	configSetScalar
	configSetLoopInput
	configSetStringOrStringSlice
	configSetACPOptions
)

var configScalarMutationKinds = mergeConfigSetValueKinds(map[string]configSetValueKind{
	"daemon.socket":                                   configSetString,
	"daemon.reload_timeouts.providers":                configSetDuration,
	"daemon.reload_timeouts.mcp":                      configSetDuration,
	"http.host":                                       configSetString,
	"http.port":                                       configSetInt,
	"defaults.agent":                                  configSetString,
	configDefaultsProviderPath:                        configSetString,
	"limits.max_concurrent_agents":                    configSetInt,
	"session.limits.timeout":                          configSetDuration,
	"session.supervision.activity_heartbeat_interval": configSetDuration,
	"session.supervision.prompt_deadline":             configSetDuration,
	"session.supervision.progress_notify_interval":    configSetDuration,
	"session.supervision.inactivity_warning_after":    configSetDuration,
	"session.supervision.inactivity_timeout":          configSetDuration,
	"session.supervision.timeout_cancel_grace":        configSetDuration,
	"permissions.mode":                                configSetString,
	"observability.enabled":                           configSetBool,
	"observability.retention_days":                    configSetInt,
	"observability.max_global_bytes":                  configSetInt64,
	"observability.transcripts.enabled":               configSetBool,
	"observability.transcripts.segment_bytes":         configSetInt,
	"observability.transcripts.max_bytes_per_session": configSetInt64,
	"log.level":                                         configSetString,
	"log.max_size_mb":                                   configSetInt,
	"log.max_backups":                                   configSetInt,
	"log.max_age_days":                                  configSetInt,
	"log.compress_backups":                              configSetBool,
	"skills.enabled":                                    configSetBool,
	"skills.sources":                                    configSetStringSlice,
	"skills.custom_sources":                             configSetStringSlice,
	"skills.disabled_skills":                            configSetStringSlice,
	"skills.poll_interval":                              configSetDuration,
	"skills.allowed_marketplace_hooks":                  configSetStringSlice,
	"model_catalog.sources.models_dev.enabled":          configSetBool,
	"model_catalog.sources.models_dev.endpoint":         configSetString,
	"model_catalog.sources.models_dev.ttl":              configSetDuration,
	"model_catalog.sources.models_dev.timeout":          configSetDuration,
	"automation.enabled":                                configSetBool,
	"automation.timezone":                               configSetString,
	"automation.max_concurrent_jobs":                    configSetInt,
	"agents.soul.enabled":                               configSetBool,
	"agents.soul.max_body_bytes":                        configSetInt64,
	"agents.soul.context_projection_bytes":              configSetInt64,
	"agents.heartbeat.enabled":                          configSetBool,
	"agents.heartbeat.max_body_bytes":                   configSetInt64,
	"agents.heartbeat.context_projection_bytes":         configSetInt64,
	"agents.heartbeat.min_interval":                     configSetDuration,
	"agents.heartbeat.default_interval":                 configSetDuration,
	"agents.heartbeat.wake_cooldown":                    configSetDuration,
	"agents.heartbeat.max_wakes_per_cycle":              configSetInt,
	"agents.heartbeat.active_session_only":              configSetBool,
	"agents.heartbeat.allow_active_hours_preferences":   configSetBool,
	"agents.heartbeat.wake_event_retention":             configSetDuration,
	"agents.heartbeat.session_health_stale_after":       configSetDuration,
	"agents.heartbeat.session_health_hook_min_interval": configSetDuration,
},
	roleConfigSetPathKinds(),
	gatewayConfigSetPathKinds(),
	loopAndGoalConfigSetPathKinds(),
	extensionConfigSetPathKinds(),
	marketplaceConfigSetPathKinds(),
	attentionConfigSetPathKinds(),
	shellConfigSetPathKinds(),
	terminalConfigSetPathKinds(),
)
