package config

import (
	"errors"
	"fmt"

	"reflect"
	"sort"

	"strings"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

const (
	toolSurfaceAgentsHeartbeatContextProjectionBytesPath       = "agents.heartbeat.context_projection_bytes"
	toolSurfaceAgentsHeartbeatMaxBodyBytesPath                 = "agents.heartbeat.max_body_bytes"
	toolSurfaceAgentsHeartbeatMaxWakesPerCyclePath             = "agents.heartbeat.max_wakes_per_cycle"
	toolSurfaceAgentsHeartbeatMinIntervalPath                  = "agents.heartbeat.min_interval"
	toolSurfaceAgentsHeartbeatSessionHealthHookMinIntervalPath = "agents.heartbeat.session_health_hook_min_interval"
	toolSurfaceAgentsHeartbeatSessionHealthStaleAfterPath      = "agents.heartbeat.session_health_stale_after"
	toolSurfaceAgentsHeartbeatWakeCooldownPath                 = "agents.heartbeat.wake_cooldown"
	toolSurfaceAgentsHeartbeatWakeEventRetentionPath           = "agents.heartbeat.wake_event_retention"
	toolSurfaceAgentsSoulContextProjectionBytesPath            = "agents.soul.context_projection_bytes"
	toolSurfaceAgentsSoulMaxBodyBytesPath                      = "agents.soul.max_body_bytes"
	toolSurfaceCommandKey                                      = "command"
	toolSurfaceDefaultKey                                      = "default"
	toolSurfaceDaemonSubprocessHealthEscalationThresholdPath   = "daemon.subprocess_health_escalation_threshold"
	toolSurfaceDefaultsAgentPath                               = "defaults.agent"
	toolSurfaceEventKey                                        = "event"
	toolSurfaceExtractorKey                                    = "extractor"
	toolSurfaceHooksKey                                        = "hooks"
	toolSurfaceMarketplaceKey                                  = "marketplace"
	toolSurfaceMCPServersKey                                   = "mcp_servers"
	toolSurfaceMemoryDreamGatesMinScorePath                    = "memory.dream.gates.min_score"
	toolSurfaceMemoryProviderTimeoutPath                       = "memory.provider.timeout"
	toolSurfaceMemoryRecallWeightsBm25UnicodePath              = "memory.recall.weights.bm25_unicode"
	toolSurfaceNetworkMaxReplayAgePath                         = "network.max_replay_age"
	toolSurfacePermissionsKey                                  = "permissions"
	toolSurfaceToolsDefaultMaxResultBytesPath                  = "tools.default_max_result_bytes"
	toolSurfaceToolsArtifactsMaxAgePath                        = "tools.artifacts.max_age"
	toolSurfaceToolsArtifactsMaxBytesPath                      = "tools.artifacts.max_bytes"
	toolSurfaceToolsArtifactsMaxCountPath                      = "tools.artifacts.max_count"
	toolSurfaceWebhookSecretRefKey                             = "webhook_secret_ref"
)

var (
	configToolDurationType = reflect.TypeFor[time.Duration]()

	agentMutableConfigKinds = mergeAgentMutableConfigKinds(map[string]ValueKind{
		toolSurfaceDefaultsAgentPath:                               ConfigValueString,
		"defaults.provider":                                        ConfigValueString,
		"defaults.sandbox":                                         ConfigValueString,
		"agents.soul.enabled":                                      ConfigValueBool,
		toolSurfaceAgentsSoulMaxBodyBytesPath:                      ConfigValueInt64,
		toolSurfaceAgentsSoulContextProjectionBytesPath:            ConfigValueInt64,
		"agents.heartbeat.enabled":                                 ConfigValueBool,
		toolSurfaceAgentsHeartbeatMaxBodyBytesPath:                 ConfigValueInt64,
		toolSurfaceAgentsHeartbeatContextProjectionBytesPath:       ConfigValueInt64,
		toolSurfaceAgentsHeartbeatMinIntervalPath:                  ConfigValueDuration,
		"agents.heartbeat.default_interval":                        ConfigValueDuration,
		toolSurfaceAgentsHeartbeatWakeCooldownPath:                 ConfigValueDuration,
		toolSurfaceAgentsHeartbeatMaxWakesPerCyclePath:             ConfigValueInt,
		"agents.heartbeat.active_session_only":                     ConfigValueBool,
		"agents.heartbeat.allow_active_hours_preferences":          ConfigValueBool,
		toolSurfaceAgentsHeartbeatWakeEventRetentionPath:           ConfigValueDuration,
		toolSurfaceAgentsHeartbeatSessionHealthStaleAfterPath:      ConfigValueDuration,
		toolSurfaceAgentsHeartbeatSessionHealthHookMinIntervalPath: ConfigValueDuration,
		"limits.max_concurrent_agents":                             ConfigValueInt,
		"session.limits.timeout":                                   ConfigValueDuration,
		"session.supervision.activity_heartbeat_interval":          ConfigValueDuration,
		"session.supervision.progress_notify_interval":             ConfigValueDuration,
		"session.supervision.prompt_deadline":                      ConfigValueDuration,
		"session.supervision.inactivity_warning_after":             ConfigValueDuration,
		"session.supervision.inactivity_timeout":                   ConfigValueDuration,
		"session.supervision.timeout_cancel_grace":                 ConfigValueDuration,
		"session.busy_input.default_mode":                          ConfigValueString,
		"session.busy_input.queue_cap":                             ConfigValueInt,
		"session.busy_input.max_text_bytes":                        ConfigValueInt,
		"session.compaction.enabled":                               ConfigValueBool,
		"session.compaction.pressure_threshold":                    ConfigValueFloat,
		"session.compaction.max_attempts_per_turn":                 ConfigValueInt,
		"session.compaction.failure_cooldown":                      ConfigValueDuration,
		toolSurfaceDaemonSubprocessHealthEscalationThresholdPath:   ConfigValueInt,
		"redact.enabled":                                           ConfigValueBool,
		"memory.enabled":                                           ConfigValueBool,
		"memory.controller.mode":                                   ConfigValueString,
		"memory.controller.max_latency":                            ConfigValueDuration,
		"memory.controller.default_op_on_fail":                     ConfigValueString,
		"memory.controller.policy.max_content_chars":               ConfigValueInt,
		"memory.controller.policy.max_writes_per_min":              ConfigValueInt,
		"memory.controller.policy.allow_origins":                   ConfigValueStringSlice,
		"memory.recall.top_k":                                      ConfigValueInt,
		"memory.recall.raw_candidates":                             ConfigValueInt,
		"memory.recall.fusion":                                     ConfigValueString,
		"memory.recall.include_already_surfaced":                   ConfigValueBool,
		"memory.recall.include_system":                             ConfigValueBool,
		toolSurfaceMemoryRecallWeightsBm25UnicodePath:              ConfigValueFloat,
		"memory.recall.weights.bm25_trigram":                       ConfigValueFloat,
		"memory.recall.weights.recency":                            ConfigValueFloat,
		"memory.recall.weights.recall_signal":                      ConfigValueFloat,
		"memory.recall.freshness.banner_after_days":                ConfigValueInt,
		"memory.recall.signals.queue_capacity":                     ConfigValueInt,
		"memory.recall.signals.worker_retry_max":                   ConfigValueInt,
		"memory.recall.signals.metrics_enabled":                    ConfigValueBool,
		"memory.decisions.prune_after_applied_days":                ConfigValueInt,
		"memory.decisions.keep_audit_summary":                      ConfigValueBool,
		"memory.decisions.max_post_content_bytes":                  ConfigValueInt64,
		"memory.extractor.mode":                                    ConfigValueString,
		"memory.extractor.throttle_turns":                          ConfigValueInt,
		"memory.extractor.deadline":                                ConfigValueDuration,
		"memory.extractor.sandbox_inbox_only":                      ConfigValueBool,
		"memory.extractor.queue.capacity":                          ConfigValueInt,
		configExtractorQueueCoalesceMaxPath:                        ConfigValueInt,
		"memory.dream.min_hours":                                   ConfigValueFloat,
		"memory.dream.min_sessions":                                ConfigValueInt,
		"memory.dream.debounce":                                    ConfigValueDuration,
		"memory.dream.prompt_version":                              ConfigValueString,
		"memory.dream.check_interval":                              ConfigValueDuration,
		"memory.dream.gates.min_unpromoted":                        ConfigValueInt,
		"memory.dream.gates.min_recall_count":                      ConfigValueInt,
		toolSurfaceMemoryDreamGatesMinScorePath:                    ConfigValueFloat,
		"memory.dream.scoring.recency_half_life_days":              ConfigValueInt,
		"memory.dream.scoring.weights.frequency":                   ConfigValueFloat,
		"memory.dream.scoring.weights.relevance":                   ConfigValueFloat,
		"memory.dream.scoring.weights.recency":                     ConfigValueFloat,
		"memory.dream.scoring.weights.freshness":                   ConfigValueFloat,
		"memory.session.ledger_format":                             ConfigValueString,
		"memory.session.events_purge_grace":                        ConfigValueDuration,
		"memory.session.cold_archive_days":                         ConfigValueInt,
		"memory.session.hard_delete_days":                          ConfigValueInt,
		"memory.session.max_archive_bytes":                         ConfigValueInt64,
		"memory.session.unbound_partition":                         ConfigValueString,
		"memory.daily.max_bytes":                                   ConfigValueInt64,
		"memory.daily.max_lines":                                   ConfigValueInt,
		"memory.daily.rotate_format":                               ConfigValueString,
		"memory.daily.dreaming_window":                             ConfigValueInt,
		"memory.daily.cold_archive_days":                           ConfigValueInt,
		"memory.daily.hard_delete_days":                            ConfigValueInt,
		"memory.daily.max_archive_bytes":                           ConfigValueInt64,
		"memory.daily.sweep_hour":                                  ConfigValueInt,
		"memory.file.max_lines":                                    ConfigValueInt,
		"memory.file.max_bytes":                                    ConfigValueInt64,
		"memory.provider.name":                                     ConfigValueString,
		toolSurfaceMemoryProviderTimeoutPath:                       ConfigValueDuration,
		"memory.provider.failure_threshold":                        ConfigValueInt,
		"memory.provider.cooldown":                                 ConfigValueDuration,
		"memory.workspace.auto_create":                             ConfigValueBool,
		"marketplace.catalog.ttl":                                  ConfigValueDuration,
		"marketplace.catalog.timeout":                              ConfigValueDuration,
		"skills.enabled":                                           ConfigValueBool,
		"skills.disabled_skills":                                   ConfigValueStringSlice,
		"skills.poll_interval":                                     ConfigValueDuration,
		"network.enabled":                                          ConfigValueBool,
		toolSurfaceNetworkMaxReplayAgePath:                         ConfigValueInt,
		"network.live.defaults.max_wakes":                          ConfigValueInt,
		"network.live.defaults.max_wake_wall_time":                 ConfigValueString,
		"network.live.defaults.max_total_wall_time":                ConfigValueString,
		"network.live.defaults.max_input_tokens":                   ConfigValueInt64,
		"network.live.defaults.max_output_tokens":                  ConfigValueInt64,
		"network.live.defaults.max_wake_depth":                     ConfigValueInt,
		"network.live.defaults.coalesce_window":                    ConfigValueString,
		networkLiveLimitsMaxWakesPath:                              ConfigValueInt,
		"network.live.limits.max_wake_wall_time":                   ConfigValueString,
		"network.live.limits.max_total_wall_time":                  ConfigValueString,
		"network.live.limits.max_input_tokens":                     ConfigValueInt64,
		"network.live.limits.max_output_tokens":                    ConfigValueInt64,
		"network.live.limits.max_wake_depth":                       ConfigValueInt,
		networkLiveLimitsMinCoalesceWindowPath:                     ConfigValueString,
		"network.live.limits.max_coalesce_window":                  ConfigValueString,
		toolSurfaceToolsDefaultMaxResultBytesPath:                  ConfigValueInt64,
		toolSurfaceToolsArtifactsMaxAgePath:                        ConfigValueDuration,
		toolSurfaceToolsArtifactsMaxBytesPath:                      ConfigValueInt64,
		toolSurfaceToolsArtifactsMaxCountPath:                      ConfigValueInt,
	}, roleMutableConfigKinds, automationToolPathKinds(), loopAndGoalToolPathKinds(), taskToolSurfaceMutableConfigKinds())
)

// RedactedConfigMap converts config to the same redacted map shape used by operator-facing CLI output.
func RedactedConfigMap(cfg *Config) map[string]any {
	node, ok := configNodeFromValue(reflect.ValueOf(cfg), "")
	if !ok {
		return map[string]any{}
	}
	values, ok := node.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return values
}

// FlattenConfigEntries returns deterministic flattened config entries.
func FlattenConfigEntries(configMap map[string]any) []Entry {
	entries := make([]Entry, 0)
	flattenConfigValue(&entries, "", configMap, false)
	sort.Slice(entries, func(i int, j int) bool {
		return entries[i].Path < entries[j].Path
	})
	return entries
}

// DiffConfigEntries returns sorted redacted differences between two effective entry sets.
func DiffConfigEntries(before []Entry, after []Entry) []DiffEntry {
	beforeByPath := make(map[string]Entry, len(before))
	for _, entry := range before {
		beforeByPath[entry.Path] = entry
	}
	afterByPath := make(map[string]Entry, len(after))
	for _, entry := range after {
		afterByPath[entry.Path] = entry
	}
	paths := make(map[string]struct{}, len(beforeByPath)+len(afterByPath))
	for path := range beforeByPath {
		paths[path] = struct{}{}
	}
	for path := range afterByPath {
		paths[path] = struct{}{}
	}

	diff := make([]DiffEntry, 0)
	for path := range paths {
		left, hasLeft := beforeByPath[path]
		right, hasRight := afterByPath[path]
		if hasLeft && hasRight && reflect.DeepEqual(left.Value, right.Value) && left.Redacted == right.Redacted {
			continue
		}
		entry := DiffEntry{Path: path}
		if hasLeft {
			entry.Before = left.Value
			entry.BeforeRedacted = left.Redacted
		}
		if hasRight {
			entry.After = right.Value
			entry.AfterRedacted = right.Redacted
		}
		diff = append(diff, entry)
	}
	sort.Slice(diff, func(i int, j int) bool {
		return diff[i].Path < diff[j].Path
	})
	return diff
}

// ParseDottedConfigPath parses a user-facing dotted config path.
func ParseDottedConfigPath(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("config: config path is required")
	}
	parts := strings.Split(trimmed, ".")
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return nil, fmt.Errorf("config: config path %q contains an empty segment", trimmed)
		}
	}
	return parts, nil
}

// ClassifyToolConfigPath applies the agent-facing mutable config policy.
func ClassifyToolConfigPath(path []string) (PathPolicy, error) {
	clean, err := normalizeMutationPath(path)
	if err != nil {
		return PathPolicy{}, err
	}
	policy := PathPolicy{Segments: clean, Kind: ConfigValueString}
	joined := strings.Join(clean, ".")
	if kind, ok := agentMutableConfigKinds[joined]; ok {
		policy.Kind = kind
		return policy, nil
	}
	if configPathIsSecret(clean) {
		policy.Denial = ConfigPathSecretForbidden
		return policy, nil
	}
	if configPathHasArraySyntax(clean) {
		policy.Denial = ConfigPathForbidden
		return policy, nil
	}
	if configPathIsTrustRoot(clean) {
		policy.Denial = ConfigPathTrustForbidden
		return policy, nil
	}
	if len(clean) == 3 && clean[0] == providersConfigKey {
		switch clean[2] {
		case toolSurfaceCommandKey,
			"auth_mode",
			"env_policy",
			"home_policy",
			"auth_status_command",
			"auth_login_command":
			policy.Kind = ConfigValueString
			return policy, nil
		case "session_mcp":
			policy.Kind = ConfigValueBool
			return policy, nil
		}
	}
	if len(clean) == 4 && clean[0] == providersConfigKey && clean[2] == toolSurfaceModelsKey {
		if clean[3] == toolSurfaceDefaultKey {
			policy.Kind = ConfigValueString
			return policy, nil
		}
	}
	policy.Denial = ConfigPathForbidden
	return policy, nil
}

// NormalizeToolConfigValue coerces a JSON-decoded tool value into a supported TOML value.
func NormalizeToolConfigValue(kind ValueKind, value any) (any, error) {
	switch kind {
	case ConfigValueString:
		typed, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("config: expected string value, got %T", value)
		}
		return typed, nil
	case ConfigValueBool:
		return coerceConfigBool(value)
	case ConfigValueInt:
		return coerceConfigInt(value)
	case ConfigValueInt64:
		return coerceConfigInt64(value)
	case ConfigValueFloat:
		return coerceConfigFloat(value)
	case ConfigValueDuration:
		raw, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("config: expected duration string, got %T", value)
		}
		trimmed := strings.TrimSpace(raw)
		if _, err := time.ParseDuration(trimmed); err != nil {
			return nil, fmt.Errorf("config: parse duration value %q: %w", raw, err)
		}
		return trimmed, nil
	case ConfigValueStringSlice:
		return coerceConfigStringSlice(value)
	case ConfigValueTable:
		table, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("config: expected object value, got %T", value)
		}
		return normalizeTreeValue(table)
	default:
		return nil, fmt.Errorf("config: unsupported config value kind %d", kind)
	}
}

// OverlayHookDeclarations returns config-backed hook declarations from one overlay target.
func OverlayHookDeclarations(target WriteTarget) ([]hookspkg.HookDecl, error) {
	if !target.isConfigTarget() {
		return nil, fmt.Errorf("config: write target %q is not a config overlay", target.Kind())
	}
	overlay, err := loadConfigOverlayFile(target.path)
	if err != nil {
		return nil, err
	}
	decls := make([]hookspkg.HookDecl, 0, len(overlay.Hooks.Declarations))
	for idx := range overlay.Hooks.Declarations {
		raw := &overlay.Hooks.Declarations[idx]
		decl, err := raw.toHookDecl(hookspkg.HookSourceConfig, "")
		if err != nil {
			return nil, fmt.Errorf("hooks.declarations[%d]: %w", idx, err)
		}
		decls = append(decls, decl)
	}
	return decls, nil
}

// HookDeclarationOverlayValues converts a hook declaration to TOML overlay values.
func HookDeclarationOverlayValues(decl hookspkg.HookDecl) map[string]any {
	values := map[string]any{
		toolSurfaceEventKey: string(decl.Event),
	}
	if decl.Enabled != nil {
		values[string(ToolsExternalDefaultEnabled)] = *decl.Enabled
	}
	if decl.Mode != "" {
		values["mode"] = string(decl.Mode)
	}
	if decl.Required {
		values["required"] = true
	}
	if decl.PrioritySet || decl.Priority != 0 {
		values["priority"] = decl.Priority
	}
	if decl.Timeout > 0 {
		values["timeout"] = decl.Timeout.String()
	}
	if matcher := hookMatcherOverlayValues(decl.Matcher); len(matcher) > 0 {
		values["matcher"] = matcher
	}
	if decl.Command != "" {
		values[toolSurfaceCommandKey] = decl.Command
	}
	if len(decl.Args) > 0 {
		values["args"] = append([]string(nil), decl.Args...)
	}
	if len(decl.Env) > 0 {
		values["env"] = mergeStringMaps(nil, decl.Env)
	}
	if decl.ExecutorKind != "" && decl.ExecutorKind != hookspkg.HookExecutorSubprocess {
		values["executor"] = map[string]any{"kind": string(decl.ExecutorKind)}
	}
	return values
}
