package config

import (
	"cmp"
	"errors"
	"fmt"

	"reflect"
	"slices"

	"strings"
	"time"

	hookspkg "github.com/compozy/compozy/internal/hooks"
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
	toolSurfaceAuthLoginCommandKey                             = "auth_login_command"
	toolSurfaceCommandKey                                      = "command"
	toolSurfaceDefaultKey                                      = "default"
	toolSurfaceDaemonSubprocessHealthEscalationThresholdPath   = "daemon.subprocess_health_escalation_threshold"
	toolSurfaceDefaultsAgentPath                               = "defaults.agent"
	toolSurfaceEventKey                                        = "event"
	toolSurfaceHooksKey                                        = "hooks"
	toolSurfaceMarketplaceKey                                  = "marketplace"
	toolSurfaceMCPServersKey                                   = "mcp_servers"
	toolSurfacePermissionsKey                                  = "permissions"
	toolSurfaceToolsDefaultMaxResultBytesPath                  = "tools.default_max_result_bytes"
	toolSurfaceToolsArtifactsMaxAgePath                        = "tools.artifacts.max_age"
	toolSurfaceToolsArtifactsMaxBytesPath                      = "tools.artifacts.max_bytes"
	toolSurfaceToolsArtifactsMaxCountPath                      = "tools.artifacts.max_count"
	toolSurfaceToolsClarifyTimeoutPath                         = "tools.clarify.timeout"
	toolSurfaceWebhookSecretRefKey                             = "webhook_secret_ref"
)

// toolSurfaceToolsClarifyTimeoutSegments splits the boot-scoped clarify
// timeout path for scope checks without repeating its segments.
var toolSurfaceToolsClarifyTimeoutSegments = strings.Split(toolSurfaceToolsClarifyTimeoutPath, ".")

// mcpStdioConfigSegments prefixes the boot-scoped stdio MCP process reuse keys.
var mcpStdioConfigSegments = []string{"mcp", "stdio"}

// mcpStatusProbeTimeoutSegments names the boot-scoped MCP status probe deadline.
var mcpStatusProbeTimeoutSegments = []string{"mcp", "status_probe_timeout"}

var sessionAuthRecoverySegments = []string{"session", "auth_recovery"}

var (
	configToolDurationType = reflect.TypeFor[time.Duration]()

	agentMutableConfigKinds = mergeAgentMutableConfigKinds(map[string]ValueKind{
		"provider_usage.enabled":                                   ConfigValueBool,
		"provider_usage.refresh_interval":                          ConfigValueDuration,
		"subagents.result_max_chars":                               ConfigValueInt,
		"app.update_check":                                         ConfigValueBool,
		appUpdateCheckIntervalPath:                                 ConfigValueDuration,
		toolSurfaceDefaultsAgentPath:                               ConfigValueString,
		"user_scope.work_dir":                                      ConfigValueString,
		"defaults.provider":                                        ConfigValueString,
		configDefaultsModelPath:                                    ConfigValueString,
		configDefaultsReasoningEffortPath:                          ConfigValueString,
		"agents.always_on.rotation_threshold":                      ConfigValueFloat,
		"agents.always_on.unattended_turn_limit":                   ConfigValueInt,
		"agents.always_on.max_active_instances":                    ConfigValueInt,
		"agents.always_on.max_proposals":                           ConfigValueInt,
		"agents.always_on.triage_timeout":                          ConfigValueDuration,
		"agents.always_on.approval_timeout":                        ConfigValueDuration,
		"agents.soul.enabled":                                      ConfigValueBool,
		toolSurfaceAgentsSoulMaxBodyBytesPath:                      ConfigValueInt64,
		toolSurfaceAgentsSoulContextProjectionBytesPath:            ConfigValueInt64,
		"agents.heartbeat.sources.scripts_enabled":                 ConfigValueBool,
		"agents.heartbeat.sources.timeout":                         ConfigValueDuration,
		"agents.heartbeat.sources.max_concurrent_polls":            ConfigValueInt,
		"agents.heartbeat.sources.max_per_agent":                   ConfigValueInt,
		"agents.heartbeat.sources.max_items":                       ConfigValueInt,
		"agents.heartbeat.sources.max_output_bytes":                ConfigValueInt,
		"agents.heartbeat.sources.max_state_bytes":                 ConfigValueInt,
		"agents.heartbeat.sources.failure_threshold":               ConfigValueInt,
		"agents.heartbeat.sources.env_allow":                       ConfigValueStringSlice,
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
		"agents.heartbeat.quiet_backoff_cap":                       ConfigValueDuration,
		"agents.heartbeat.max_interval":                            ConfigValueDuration,
		"agents.heartbeat.max_concurrent_triage":                   ConfigValueInt,
		"agents.heartbeat.fatigue_limit":                           ConfigValueInt,
		toolSurfaceAgentsHeartbeatSessionHealthStaleAfterPath:      ConfigValueDuration,
		toolSurfaceAgentsHeartbeatSessionHealthHookMinIntervalPath: ConfigValueDuration,
		"limits.max_concurrent_agents":                             ConfigValueInt,
		"session.limits.timeout":                                   ConfigValueDuration,
		"session.stop.cooperative_grace":                           ConfigValueDuration,
		"daemon.shutdown_grace":                                    ConfigValueDuration,
		"session.restart.auto_continue":                            ConfigValueBool,
		"session.restart.max_attempts":                             ConfigValueInt,
		"session.restart.window":                                   ConfigValueDuration,
		"session.auth_recovery.enabled":                            ConfigValueBool,
		"session.auth_recovery.window":                             ConfigValueDuration,
		"session.auth_recovery.initial_delay":                      ConfigValueDuration,
		"session.auth_recovery.max_delay":                          ConfigValueDuration,
		"session.supervision.activity_heartbeat_interval":          ConfigValueDuration,
		"session.supervision.progress_notify_interval":             ConfigValueDuration,
		"session.supervision.prompt_deadline":                      ConfigValueDuration,
		"session.supervision.inactivity_warning_after":             ConfigValueDuration,
		"session.supervision.inactivity_timeout":                   ConfigValueDuration,
		"session.supervision.quiet_after":                          ConfigValueDuration,
		"session.supervision.stop_grace":                           ConfigValueDuration,
		"session.supervision.idle_release_after":                   ConfigValueDuration,
		"session.supervision.timeout_cancel_grace":                 ConfigValueDuration,
		"session.busy_input.default_mode":                          ConfigValueString,
		"session.busy_input.queue_cap":                             ConfigValueInt,
		"session.busy_input.max_text_bytes":                        ConfigValueInt,
		"session.messaging.rate_limit_messages":                    ConfigValueInt,
		"session.messaging.rate_limit_window":                      ConfigValueDuration,
		"session.messaging.reply_watch_ttl":                        ConfigValueDuration,
		"session.messaging.queue_promote_after":                    ConfigValueDuration,
		toolSurfaceDaemonSubprocessHealthEscalationThresholdPath:   ConfigValueInt,
		daemonLoadAgentNicenessPath:                                ConfigValueInt,
		daemonLoadAdmissionThreshold:                               ConfigValueFloat,
		daemonLoadAdmissionMaxWaitPath:                             ConfigValueDuration,
		"redact.enabled":                                           ConfigValueBool,
		"marketplace.catalog.ttl":                                  ConfigValueDuration,
		"marketplace.catalog.timeout":                              ConfigValueDuration,
		"skills.enabled":                                           ConfigValueBool,
		"skills.sources":                                           ConfigValueStringSlice,
		"skills.custom_sources":                                    ConfigValueStringSlice,
		"skills.disabled_skills":                                   ConfigValueStringSlice,
		"skills.poll_interval":                                     ConfigValueDuration,
		"shell.sessions.sort":                                      ConfigValueString,
		"shell.sessions.group_by":                                  ConfigValueString,
		"shell.sessions.scope":                                     ConfigValueString,
		"cmd_palette.fallback_targets":                             ConfigValueStringSlice,
		"cmd_palette.personalization":                              ConfigValueBool,
		toolSurfaceToolsDefaultMaxResultBytesPath:                  ConfigValueInt64,
		toolSurfaceToolsArtifactsMaxAgePath:                        ConfigValueDuration,
		toolSurfaceToolsArtifactsMaxBytesPath:                      ConfigValueInt64,
		toolSurfaceToolsArtifactsMaxCountPath:                      ConfigValueInt,
		toolSurfaceToolsClarifyTimeoutPath:                         ConfigValueDuration,
	}, roleMutableConfigKinds, automationToolPathKinds(), loopAndGoalToolPathKinds(),
		taskToolSurfaceMutableConfigKinds(), worktreeToolSurfaceMutableConfigKinds(),
		sessionAttachmentsToolPathKinds())
)

// RedactedConfigMap converts config to the same redacted map shape used by operator-facing CLI output.
func RedactedConfigMap(cfg *Config) map[string]any {
	// Reflection avoids a duplicate TOML schema on the administrative path measured by BenchmarkRedactedConfigMap.
	node, ok := configNodeFromValue(reflect.ValueOf(cfg), "")
	if !ok {
		return map[string]any{}
	}
	values, ok := node.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	addQuietCompatibilityView(cfg, values)
	return values
}

// FlattenConfigEntries returns deterministic flattened config entries.
func FlattenConfigEntries(configMap map[string]any) []Entry {
	entries := make([]Entry, 0)
	flattenConfigValue(&entries, "", configMap, false)
	slices.SortFunc(entries, func(left, right Entry) int {
		return cmp.Compare(left.Path, right.Path)
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
	slices.SortFunc(diff, func(left, right DiffEntry) int {
		return cmp.Compare(left.Path, right.Path)
	})
	return diff
}

// configPathAliasPrefixes maps a v0.5.0 table prefix to its replacement. Removal: v0.5.2.
var configPathAliasPrefixes = [][2][]string{
	{{AgentsDirName, "stays_on"}, {AgentsDirName, "always_on"}},
}

// ConfigPathAliasWarning is the deprecation text for the v0.5.0 agents.stays_on paths. Removal: v0.5.2.
const ConfigPathAliasWarning = "agents.stays_on is deprecated; use agents.always_on (removed in v0.5.2)"

// CanonicalConfigPath rewrites a deprecated table prefix to its replacement. deprecated reports
// that the old prefix was used so the boundary can warn. Removal: v0.5.2.
func CanonicalConfigPath(path []string) (canonical []string, deprecated bool) {
	for _, alias := range configPathAliasPrefixes {
		if configPathHasPrefix(path, alias[0]) {
			return append(slices.Clone(alias[1]), path[len(alias[0]):]...), true
		}
	}
	return path, false
}

// CanonicalDottedConfigPath is CanonicalConfigPath for a dotted path. Removal: v0.5.2.
func CanonicalDottedConfigPath(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	canonical, deprecated := CanonicalConfigPath(strings.Split(trimmed, "."))
	if !deprecated {
		return raw, false
	}
	return strings.Join(canonical, "."), true
}

// LogConfigPathAlias records the daemon-side deprecation for one aliased key. Removal: v0.5.2.
func LogConfigPathAlias(path []string) {
	DeprecationLogger().Warn(ConfigPathAliasWarning, "keys", strings.Join(path, "."))
}

// configPathAliasTwin returns the other spelling of an aliased path, so an unset clears both
// tables in one overlay file. Removal: v0.5.2.
func configPathAliasTwin(path []string) ([]string, bool) {
	for _, alias := range configPathAliasPrefixes {
		switch {
		case configPathHasPrefix(path, alias[0]):
			return append(slices.Clone(alias[1]), path[len(alias[0]):]...), true
		case configPathHasPrefix(path, alias[1]):
			return append(slices.Clone(alias[0]), path[len(alias[1]):]...), true
		}
	}
	return nil, false
}

// configPathHasPrefix reports whether path is prefix itself or a key under it, so the whole
// aliased table resolves the same way as its keys.
func configPathHasPrefix(path []string, prefix []string) bool {
	return len(path) >= len(prefix) && slices.Equal(path[:len(prefix)], prefix)
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
	clean, deprecated := CanonicalConfigPath(clean)
	policy := PathPolicy{Segments: clean, Kind: ConfigValueString, DeprecatedAlias: deprecated}
	if kind, ok := sessionEnvMutationKind(clean); ok {
		policy.Kind = kind
		return policy, nil
	}
	if len(clean) == 4 && clean[0] == "agents" && clean[1] == "heartbeat" && clean[2] == "triage_models" {
		policy.Kind = ConfigValueString
		return policy, nil
	}
	joined := strings.Join(clean, ".")
	if kind, ok := agentMutableConfigKinds[joined]; ok {
		policy.Kind = kind
		return policy, nil
	}
	if len(clean) == 4 && clean[0] == LoopsConfigKey && clean[1] == LoopInputsConfigKey {
		policy.Kind = ConfigValueLoopInput
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
	if providerUsageToolPath(clean) {
		policy.Kind = ConfigValueBool
		return policy, nil
	}
	if providerPolicy, ok := providerMutableConfigPolicy(clean); ok {
		return providerPolicy, nil
	}
	policy.Denial = ConfigPathForbidden
	return policy, nil
}

// NormalizeToolConfigValue coerces a JSON-decoded tool value into a supported TOML value.
func NormalizeToolConfigValue(kind ValueKind, value any) (any, error) {
	switch kind {
	case ConfigValueSessionEnv, ConfigValueSessionEnvBinding:
		return normalizeSessionEnvMutation(kind, value)
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
	case ConfigValueUint64:
		return coerceConfigUint64(value)
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
	case ConfigValueACPOptions:
		return normalizeToolACPOptions(value)
	case ConfigValueScalar, ConfigValueLoopInput:
		return normalizeScalarConfigValue(kind, value)
	default:
		return nil, fmt.Errorf("config: unsupported config value kind %d", kind)
	}
}

func normalizeScalarConfigValue(kind ValueKind, value any) (any, error) {
	if kind == ConfigValueLoopInput {
		if table, ok := value.(map[string]any); ok {
			return normalizeTreeValue(table)
		}
	}
	switch value.(type) {
	case string, bool:
		return value, nil
	}
	if normalized, ok := normalizeIntegerValue(value); ok {
		return normalized, nil
	}
	if normalized, ok := normalizeUnsignedValue(value); ok {
		return normalized, nil
	}
	if normalized, ok := normalizeFloatValue(value); ok {
		return normalized, nil
	}
	if kind == ConfigValueLoopInput {
		return nil, fmt.Errorf("config: expected string, boolean, number, or object value, got %T", value)
	}
	return nil, fmt.Errorf("config: expected string, boolean, or number value, got %T", value)
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

func providerMutableConfigPolicy(clean []string) (PathPolicy, bool) {
	policy := PathPolicy{Segments: clean, Kind: ConfigValueString}
	if len(clean) == 3 && clean[0] == providersConfigKey {
		switch clean[2] {
		case toolSurfaceCommandKey,
			"auth_mode",
			"env_policy",
			"home_policy",
			"steer_capability",
			"session_mcp_transport",
			"auth_status_command":
			policy.Kind = ConfigValueString
			return policy, true
		case toolSurfaceAuthLoginCommandKey:
			policy.Kind = ConfigValueString
			policy.Redacted = true
			return policy, true
		case "session_mcp":
			policy.Kind = ConfigValueBool
			return policy, true
		}
	}
	if len(clean) == 4 && clean[0] == providersConfigKey && clean[2] == toolSurfaceModelsKey {
		if clean[3] == toolSurfaceDefaultKey {
			policy.Kind = ConfigValueString
			return policy, true
		}
	}
	return PathPolicy{}, false
}
