package cli

import (
	"reflect"

	"time"

	aghconfig "github.com/compozy/agh/internal/config"
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
	configEnabledKey           = "enabled"
	configInvalidKey           = "invalid"
	configListKey              = "list"
	configManagedKey           = "managed"
	configManagerKey           = "manager"
	configMemoryKey            = "memory"
	configNetworkKey           = "network"
	configPathKey              = "path"
	configReadKey              = "read"
	configRedactedKey          = "redacted"
	configRequiredKey          = "required"
	configScopeKey             = "scope"
	configShowKey              = "show"
	configSkillsKey            = "skills"
	configStatusKey            = "status"
	configTargetKey            = "target"
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
	ActiveGeneration int64  `json:"active_generation,omitempty"`
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
	Status        string                        `json:"status"`
	Scope         string                        `json:"scope"`
	WorkspaceRoot string                        `json:"workspace_root,omitempty"`
	ConfigFile    string                        `json:"config_file"`
	Redacted      bool                          `json:"redacted"`
	Errors        []configValidationError       `json:"errors,omitempty"`
	DotEnv        *aghconfig.DotEnvRepairReport `json:"dot_env,omitempty"`
}

type configValidationError struct {
	Code    string `json:"code"`
	Path    string `json:"path,omitempty"`
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
	Column  int    `json:"column,omitempty"`
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
	configSetFloat
	configSetDuration
	configSetStringSlice
	configSetFloatSlice
	configSetTable
)

var (
	configDurationType = reflect.TypeFor[time.Duration]()

	configScalarMutationKinds = mergeConfigSetValueKinds(map[string]configSetValueKind{
		"daemon.socket":                                   configSetString,
		"daemon.reload_timeouts.providers":                configSetDuration,
		"daemon.reload_timeouts.mcp":                      configSetDuration,
		"daemon.reload_timeouts.bridges":                  configSetDuration,
		"http.host":                                       configSetString,
		"http.port":                                       configSetInt,
		"defaults.agent":                                  configSetString,
		configDefaultsProviderPath:                        configSetString,
		"defaults.sandbox":                                configSetString,
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
		"memory.enabled":                                    configSetBool,
		"memory.global_dir":                                 configSetString,
		"memory.controller.mode":                            configSetString,
		"memory.controller.max_latency":                     configSetDuration,
		"memory.controller.default_op_on_fail":              configSetString,
		"memory.controller.policy.max_content_chars":        configSetInt,
		"memory.controller.policy.max_writes_per_min":       configSetInt,
		"memory.controller.policy.allow_origins":            configSetStringSlice,
		"memory.recall.top_k":                               configSetInt,
		"memory.recall.raw_candidates":                      configSetInt,
		"memory.recall.fusion":                              configSetString,
		"memory.recall.include_already_surfaced":            configSetBool,
		"memory.recall.include_system":                      configSetBool,
		"memory.recall.weights.bm25_unicode":                configSetFloat,
		"memory.recall.weights.bm25_trigram":                configSetFloat,
		"memory.recall.weights.recency":                     configSetFloat,
		"memory.recall.weights.recall_signal":               configSetFloat,
		"memory.recall.freshness.banner_after_days":         configSetInt,
		"memory.recall.signals.queue_capacity":              configSetInt,
		"memory.recall.signals.worker_retry_max":            configSetInt,
		"memory.recall.signals.metrics_enabled":             configSetBool,
		"memory.decisions.prune_after_applied_days":         configSetInt,
		"memory.decisions.keep_audit_summary":               configSetBool,
		"memory.decisions.max_post_content_bytes":           configSetInt64,
		"memory.extractor.mode":                             configSetString,
		"memory.extractor.throttle_turns":                   configSetInt,
		"memory.extractor.deadline":                         configSetDuration,
		"memory.extractor.sandbox_inbox_only":               configSetBool,
		"memory.extractor.inbox_path":                       configSetString,
		"memory.extractor.dlq_path":                         configSetString,
		"memory.extractor.queue.capacity":                   configSetInt,
		"memory.extractor.queue.coalesce_max":               configSetInt,
		"memory.dream.min_hours":                            configSetFloat,
		"memory.dream.min_sessions":                         configSetInt,
		"memory.dream.debounce":                             configSetDuration,
		"memory.dream.prompt_version":                       configSetString,
		"memory.dream.check_interval":                       configSetDuration,
		"memory.dream.gates.min_unpromoted":                 configSetInt,
		"memory.dream.gates.min_recall_count":               configSetInt,
		"memory.dream.gates.min_score":                      configSetFloat,
		"memory.dream.scoring.recency_half_life_days":       configSetInt,
		"memory.dream.scoring.weights.frequency":            configSetFloat,
		"memory.dream.scoring.weights.relevance":            configSetFloat,
		"memory.dream.scoring.weights.recency":              configSetFloat,
		"memory.dream.scoring.weights.freshness":            configSetFloat,
		"memory.session.ledger_format":                      configSetString,
		"memory.session.ledger_root":                        configSetString,
		"memory.session.events_purge_grace":                 configSetDuration,
		"memory.session.cold_archive_days":                  configSetInt,
		"memory.session.hard_delete_days":                   configSetInt,
		"memory.session.max_archive_bytes":                  configSetInt64,
		"memory.session.unbound_partition":                  configSetString,
		"memory.daily.max_bytes":                            configSetInt64,
		"memory.daily.max_lines":                            configSetInt,
		"memory.daily.rotate_format":                        configSetString,
		"memory.daily.dreaming_window":                      configSetInt,
		"memory.daily.cold_archive_days":                    configSetInt,
		"memory.daily.hard_delete_days":                     configSetInt,
		"memory.daily.max_archive_bytes":                    configSetInt64,
		"memory.daily.sweep_hour":                           configSetInt,
		"memory.daily.archive_path":                         configSetString,
		"memory.file.max_lines":                             configSetInt,
		"memory.file.max_bytes":                             configSetInt64,
		"memory.provider.name":                              configSetString,
		"memory.provider.timeout":                           configSetDuration,
		"memory.provider.failure_threshold":                 configSetInt,
		"memory.provider.cooldown":                          configSetDuration,
		"memory.workspace.auto_create":                      configSetBool,
		"skills.enabled":                                    configSetBool,
		"skills.disabled_skills":                            configSetStringSlice,
		"skills.poll_interval":                              configSetDuration,
		"skills.allowed_marketplace_mcp":                    configSetStringSlice,
		"skills.allowed_marketplace_hooks":                  configSetStringSlice,
		"skills.marketplace.registry":                       configSetString,
		"skills.marketplace.base_url":                       configSetString,
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
		networkConfigSetPathKinds(),
		loopAndGoalConfigSetPathKinds(),
		extensionConfigSetPathKinds(),
		marketplaceConfigSetPathKinds(),
	)
)
