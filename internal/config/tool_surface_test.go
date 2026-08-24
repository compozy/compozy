package config

import (
	"fmt"
	"strings"
	"testing"
	"time"

	hookspkg "github.com/compozy/compozy/internal/hooks"
)

func TestToolConfigPathPolicy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		path     string
		denial   PathDenial
		kind     ValueKind
		redacted bool
	}{
		{
			name: "Should allow desktop update check mutation",
			path: "app.update_check",
			kind: ConfigValueBool,
		},
		{
			name: "Should allow desktop update interval mutation",
			path: appUpdateCheckIntervalPath,
			kind: ConfigValueDuration,
		},
		{
			name: "Should allow default agent mutation",
			path: "defaults.agent",
			kind: ConfigValueString,
		},
		{
			name: "Should allow concurrent agent limit mutation",
			path: "limits.max_concurrent_agents",
			kind: ConfigValueInt,
		},
		{
			name: "Should allow automatic title enablement mutation",
			path: "roles.auto_title.enabled",
			kind: ConfigValueBool,
		},
		{
			name: "Should allow redaction enablement mutation",
			path: "redact.enabled",
			kind: ConfigValueBool,
		},
		{
			name: "Should allow automation suggestion pending cap mutation",
			path: "automation.suggestions.pending_cap",
			kind: ConfigValueInt,
		},
		{
			name: "Should allow session compaction enablement mutation",
			path: "session.compaction.enabled",
			kind: ConfigValueBool,
		},
		{
			name: "Should allow session compaction pressure mutation",
			path: "session.compaction.pressure_threshold",
			kind: ConfigValueFloat,
		},
		{
			name: "Should allow session compaction attempt cap mutation",
			path: "session.compaction.max_attempts_per_turn",
			kind: ConfigValueInt,
		},
		{
			name: "Should allow session compaction cooldown mutation",
			path: "session.compaction.failure_cooldown",
			kind: ConfigValueDuration,
		},
		{
			name: "Should allow session attachment max file bytes mutation",
			path: "session.attachments.max_file_bytes",
			kind: ConfigValueInt64,
		},
		{
			name: "Should allow session attachment prompt count mutation",
			path: "session.attachments.max_files_per_prompt",
			kind: ConfigValueInt,
		},
		{
			name: "Should allow session attachment allowed mime mutation",
			path: "session.attachments.allowed_mime",
			kind: ConfigValueStringSlice,
		},
		{
			name: "Should allow session attachment retention count mutation",
			path: "session.attachments.retention.max_count",
			kind: ConfigValueInt,
		},
		{
			name: "Should allow session attachment retention bytes mutation",
			path: "session.attachments.retention.max_bytes",
			kind: ConfigValueInt64,
		},
		{
			name: "Should allow session attachment retention age mutation",
			path: "session.attachments.retention.max_age",
			kind: ConfigValueDuration,
		},
		{
			name: "Should allow soul enabled mutation",
			path: "agents.soul.enabled",
			kind: ConfigValueBool,
		},
		{
			name: "Should allow soul max body limit mutation",
			path: "agents.soul.max_body_bytes",
			kind: ConfigValueInt64,
		},
		{
			name: "Should allow soul context projection mutation",
			path: "agents.soul.context_projection_bytes",
			kind: ConfigValueInt64,
		},
		{
			name: "Should allow heartbeat enabled mutation",
			path: "agents.heartbeat.enabled",
			kind: ConfigValueBool,
		},
		{
			name: "Should allow heartbeat max body limit mutation",
			path: "agents.heartbeat.max_body_bytes",
			kind: ConfigValueInt64,
		},
		{
			name: "Should allow heartbeat context projection mutation",
			path: "agents.heartbeat.context_projection_bytes",
			kind: ConfigValueInt64,
		},
		{
			name: "Should allow heartbeat min interval mutation",
			path: "agents.heartbeat.min_interval",
			kind: ConfigValueDuration,
		},
		{
			name: "Should allow heartbeat default interval mutation",
			path: "agents.heartbeat.default_interval",
			kind: ConfigValueDuration,
		},
		{
			name: "Should allow heartbeat wake cooldown mutation",
			path: "agents.heartbeat.wake_cooldown",
			kind: ConfigValueDuration,
		},
		{
			name: "Should allow heartbeat max wakes mutation",
			path: "agents.heartbeat.max_wakes_per_cycle",
			kind: ConfigValueInt,
		},
		{
			name: "Should allow heartbeat active session only mutation",
			path: "agents.heartbeat.active_session_only",
			kind: ConfigValueBool,
		},
		{
			name: "Should allow heartbeat active hours preference mutation",
			path: "agents.heartbeat.allow_active_hours_preferences",
			kind: ConfigValueBool,
		},
		{
			name: "Should allow heartbeat wake event retention mutation",
			path: "agents.heartbeat.wake_event_retention",
			kind: ConfigValueDuration,
		},
		{
			name: "Should allow heartbeat stale health mutation",
			path: "agents.heartbeat.session_health_stale_after",
			kind: ConfigValueDuration,
		},
		{
			name: "Should allow heartbeat health hook interval mutation",
			path: "agents.heartbeat.session_health_hook_min_interval",
			kind: ConfigValueDuration,
		},
		{
			name: "Should allow task orchestration summary budget mutation",
			path: "task.orchestration.summary_max_bytes",
			kind: ConfigValueInt,
		},
		{
			name: "Should allow task orchestration context budget mutation",
			path: "task.orchestration.context_body_max_bytes",
			kind: ConfigValueInt,
		},
		{
			name: "Should allow task orchestration prior attempts mutation",
			path: "task.orchestration.context_prior_attempts",
			kind: ConfigValueInt,
		},
		{
			name: "Should allow task orchestration recent events mutation",
			path: "task.orchestration.context_recent_events",
			kind: ConfigValueInt,
		},
		{
			name: "Should allow task orchestration spawn failure limit mutation",
			path: "task.orchestration.spawn_failure_limit",
			kind: ConfigValueInt,
		},
		{
			name: "Should allow task orchestration scheduler threshold mutation",
			path: "task.orchestration.scheduler_bad_tick_threshold",
			kind: ConfigValueInt,
		},
		{
			name: "Should allow task orchestration scheduler cooldown mutation",
			path: "task.orchestration.scheduler_bad_tick_cooldown",
			kind: ConfigValueDuration,
		},
		{
			name: "Should allow task orchestration runtime watchdog mutation",
			path: "task.orchestration.default_max_runtime",
			kind: ConfigValueDuration,
		},
		{
			name: "Should allow task orchestration workspace active run cap mutation",
			path: "task.orchestration.max_active_runs_per_workspace",
			kind: ConfigValueInt,
		},
		{
			name: "Should allow task orchestration coordinator mode mutation",
			path: "task.orchestration.profile.default_coordinator_mode",
			kind: ConfigValueString,
		},
		{
			name: "Should allow task orchestration worker mode mutation",
			path: "task.orchestration.profile.default_worker_mode",
			kind: ConfigValueString,
		},
		{
			name: "Should allow task orchestration sandbox mode mutation",
			path: "task.orchestration.profile.default_sandbox_mode",
			kind: ConfigValueString,
		},
		{
			name: "Should allow task orchestration worktree mode mutation",
			path: "task.orchestration.profile.default_worktree_mode",
			kind: ConfigValueString,
		},
		{
			name: "Should allow worktree root mutation",
			path: worktreeRootConfigPath,
			kind: ConfigValueString,
		},
		{
			name: "Should allow worktree branch namespace mutation",
			path: worktreeNamespaceConfigPath,
			kind: ConfigValueString,
		},
		{
			name: "Should allow worktree copy list mutation",
			path: worktreeCopyListConfigPath,
			kind: ConfigValueStringSlice,
		},
		{
			name: "Should allow worktree setup command mutation",
			path: worktreeSetupCommandPath,
			kind: ConfigValueString,
		},
		{
			name: "Should allow worktree setup timeout mutation",
			path: worktreeSetupTimeoutPath,
			kind: ConfigValueDuration,
		},
		{
			name: "Should allow worktree discovery cache mutation",
			path: worktreeDiscoveryTTLPath,
			kind: ConfigValueDuration,
		},
		{
			name: "Should allow task orchestration provider override gate mutation",
			path: "task.orchestration.profile.allow_task_provider_override",
			kind: ConfigValueBool,
		},
		{
			name: "Should allow task orchestration sandbox none gate mutation",
			path: "task.orchestration.profile.allow_task_sandbox_none",
			kind: ConfigValueBool,
		},
		{
			name: "Should allow task review default policy mutation",
			path: "task.orchestration.review.default_policy",
			kind: ConfigValueString,
		},
		{
			name: "Should allow task review max rounds mutation",
			path: "task.orchestration.review.max_rounds",
			kind: ConfigValueInt,
		},
		{
			name: "Should allow task review attempts mutation",
			path: "task.orchestration.review.max_review_attempts",
			kind: ConfigValueInt,
		},
		{
			name: "Should allow task review timeout mutation",
			path: "task.orchestration.review.timeout",
			kind: ConfigValueDuration,
		},
		{
			name: "Should allow task review rapid terminal window mutation",
			path: "task.orchestration.review.rapid_terminal_window",
			kind: ConfigValueDuration,
		},
		{
			name: "Should allow task review rapid terminal limit mutation",
			path: "task.orchestration.review.rapid_terminal_limit",
			kind: ConfigValueInt,
		},
		{
			name: "Should allow task review missing work item count mutation",
			path: "task.orchestration.review.missing_work_max_items",
			kind: ConfigValueInt,
		},
		{
			name: "Should allow task review missing work item byte mutation",
			path: "task.orchestration.review.missing_work_item_max_bytes",
			kind: ConfigValueInt,
		},
		{
			name: "Should allow task review reason budget mutation",
			path: "task.orchestration.review.reason_max_bytes",
			kind: ConfigValueInt,
		},
		{
			name: "Should allow task review text budget mutation",
			path: "task.orchestration.review.review_text_max_bytes",
			kind: ConfigValueInt,
		},
		{
			name: "Should allow task review guidance budget mutation",
			path: "task.orchestration.review.next_round_guidance_max_bytes",
			kind: ConfigValueInt,
		},
		{
			name: "Should allow task review failure policy mutation",
			path: "task.orchestration.review.failure_policy",
			kind: ConfigValueString,
		},
		{
			name: "Should allow memory controller policy origins mutation",
			path: "memory.controller.policy.allow_origins",
			kind: ConfigValueStringSlice,
		},
		{
			name: "Should allow memory recall scoring mutation",
			path: "memory.recall.weights.bm25_unicode",
			kind: ConfigValueFloat,
		},
		{
			name: "Should allow memory extractor queue mutation",
			path: "memory.extractor.queue.coalesce_max",
			kind: ConfigValueInt,
		},
		{
			name: "Should allow memory dream gate mutation",
			path: "memory.dream.gates.min_score",
			kind: ConfigValueFloat,
		},
		{
			name: "Should allow memory provider timeout mutation",
			path: "memory.provider.timeout",
			kind: ConfigValueDuration,
		},
		{
			name: "Should allow full roles section mutation",
			path: "roles",
			kind: ConfigValueTable,
		},
		{
			name: "Should allow dream role model mutation",
			path: "roles.dream.model",
			kind: ConfigValueString,
		},
		{
			name: "Should allow memory controller role timeout mutation",
			path: "roles.memory_controller.timeout",
			kind: ConfigValueDuration,
		},
		{
			name: "Should allow Marketplace catalog TTL mutation",
			path: "marketplace.catalog.ttl",
			kind: ConfigValueDuration,
		},
		{
			name: "Should allow Marketplace catalog timeout mutation",
			path: "marketplace.catalog.timeout",
			kind: ConfigValueDuration,
		},
		{
			name: "Should allow shell session sort mutation",
			path: "shell.sessions.sort",
			kind: ConfigValueString,
		},
		{
			name: "Should allow shell session scope mutation",
			path: "shell.sessions.scope",
			kind: ConfigValueString,
		},
		{
			name: "Should allow subprocess health escalation threshold mutation",
			path: "daemon.subprocess_health_escalation_threshold",
			kind: ConfigValueInt,
		},
		{
			name:     "Should allow write-only provider login command mutation",
			path:     "providers.claude.auth_login_command",
			kind:     ConfigValueString,
			redacted: true,
		},
		{
			name:   "Should reject daemon socket trust root",
			path:   "daemon.socket",
			denial: ConfigPathTrustForbidden,
		},
		{
			name:   "Should reject HTTP port trust root",
			path:   "http.port",
			denial: ConfigPathTrustForbidden,
		},
		{
			name:   "Should reject provider secret binding",
			path:   "providers.claude.credential_slots[0].secret_ref",
			denial: ConfigPathSecretForbidden,
		},
		{
			name:   "Should reject MCP auth secret path",
			path:   "mcp_servers[0].env.TOKEN",
			denial: ConfigPathSecretForbidden,
		},
		{
			name:   "Should reject sandbox runtime root trust path",
			path:   "sandboxes.default.runtime_root",
			denial: ConfigPathTrustForbidden,
		},
		{
			name:   "Should reject provider command trust root",
			path:   "providers.claude.command",
			denial: ConfigPathTrustForbidden,
		},
		{
			name:   "Should reject memory global dir trust root",
			path:   "memory.global_dir",
			denial: ConfigPathTrustForbidden,
		},
		{
			name:   "Should reject memory extractor inbox trust root",
			path:   "memory.extractor.inbox_path",
			denial: ConfigPathTrustForbidden,
		},
		{
			name:   "Should reject memory session ledger trust root",
			path:   "memory.session.ledger_root",
			denial: ConfigPathTrustForbidden,
		},
		{
			name:   "Should reject informational workspace TOML path",
			path:   "memory.workspace.toml_path",
			denial: ConfigPathTrustForbidden,
		},
		{
			name:   "Should reject removed network port path",
			path:   "network.port",
			denial: ConfigPathForbidden,
		},
		{
			name:   "Should reject removed dream agent path",
			path:   "memory.dream.agent",
			denial: ConfigPathForbidden,
		},
		{
			name:   "Should reject removed dream enabled path",
			path:   "memory.dream.enabled",
			denial: ConfigPathForbidden,
		},
		{
			name:   "Should reject removed extractor model path",
			path:   "memory.extractor.model",
			denial: ConfigPathForbidden,
		},
		{
			name:   "Should reject removed extractor enabled path",
			path:   "memory.extractor.enabled",
			denial: ConfigPathForbidden,
		},
		{
			name:   "Should reject removed controller LLM model path",
			path:   "memory.controller.llm.model",
			denial: ConfigPathForbidden,
		},
		{
			name:   "Should reject removed recall signal metrics path",
			path:   "memory.recall.signals.metrics_enabled",
			denial: ConfigPathForbidden,
		},
		{
			name:   "Should reject removed automatic title path",
			path:   "session.auto_title_enabled",
			denial: ConfigPathForbidden,
		},
		{
			name:   "Should reject tool policy trust root",
			path:   "tools.policy.external_default",
			denial: ConfigPathTrustForbidden,
		},
		{
			name:   "Should reject extension trust root",
			path:   "extensions.sources.github.enabled",
			denial: ConfigPathTrustForbidden,
		},
		{
			name:   "Should reject Marketplace catalog feed trust root",
			path:   "marketplace.catalog.base_url",
			denial: ConfigPathTrustForbidden,
		},
		{
			name:   "Should reject hook declarations through config tools",
			path:   "hooks.declarations",
			denial: ConfigPathTrustForbidden,
		},
		{
			name: "Should allow delivery retry attempts",
			path: "loops.defaults.delivery.retry.max_attempts",
			kind: ConfigValueInt,
		},
		{
			name: "Should allow delivery retry base",
			path: "loops.defaults.delivery.retry.backoff_base",
			kind: ConfigValueDuration,
		},
		{
			name: "Should allow delivery retry maximum",
			path: "loops.defaults.delivery.retry.backoff_max",
			kind: ConfigValueDuration,
		},
		{
			name: "Should allow delivery liveness window",
			path: "loops.defaults.delivery.liveness.silence_window",
			kind: ConfigValueDuration,
		},
		{
			name: "Should allow delivery resume streak",
			path: "loops.defaults.delivery.resume.death_streak_limit",
			kind: ConfigValueInt,
		},
		{
			name: "Should allow delivery predicate cost",
			path: "loops.defaults.delivery.predicates.cost_limit",
			kind: ConfigValueUint64,
		},
		{
			name: "Should allow delivery wait attempts",
			path: "loops.defaults.delivery.waits.admission_attempts",
			kind: ConfigValueInt,
		},
		{
			name: "Should allow delivery wait retry interval",
			path: "loops.defaults.delivery.waits.admission_retry_interval",
			kind: ConfigValueDuration,
		},
		{
			name: "Should allow delivery admission horizon",
			path: "loops.defaults.delivery.admission.tombstone_horizon",
			kind: ConfigValueDuration,
		},
		{
			name: "Should allow watch retry attempts",
			path: "loops.defaults.watch.retry.max_attempts",
			kind: ConfigValueInt,
		},
		{
			name: "Should allow watch retry base",
			path: "loops.defaults.watch.retry.backoff_base",
			kind: ConfigValueDuration,
		},
		{
			name: "Should allow watch retry maximum",
			path: "loops.defaults.watch.retry.backoff_max",
			kind: ConfigValueDuration,
		},
		{
			name: "Should allow watch liveness window",
			path: "loops.defaults.watch.liveness.silence_window",
			kind: ConfigValueDuration,
		},
		{
			name: "Should allow watch resume streak",
			path: "loops.defaults.watch.resume.death_streak_limit",
			kind: ConfigValueInt,
		},
		{
			name: "Should allow watch predicate cost",
			path: "loops.defaults.watch.predicates.cost_limit",
			kind: ConfigValueUint64,
		},
		{
			name: "Should allow watch wait attempts",
			path: "loops.defaults.watch.waits.admission_attempts",
			kind: ConfigValueInt,
		},
		{
			name: "Should allow watch wait retry interval",
			path: "loops.defaults.watch.waits.admission_retry_interval",
			kind: ConfigValueDuration,
		},
		{
			name: "Should allow watch admission horizon",
			path: "loops.defaults.watch.admission.tombstone_horizon",
			kind: ConfigValueDuration,
		},
		{
			name: "Should allow global loop breaker threshold",
			path: "loops.breaker.threshold",
			kind: ConfigValueInt,
		},
		{
			name: "Should allow global loop breaker probe interval",
			path: "loops.breaker.probe_interval",
			kind: ConfigValueDuration,
		},
		{
			name:   "Should reject delivery autopause mutation",
			path:   "loops.defaults.delivery.autopause",
			denial: ConfigPathForbidden,
		},
		{
			name:   "Should reject watch autopause mutation",
			path:   "loops.defaults.watch.autopause",
			denial: ConfigPathForbidden,
		},
		{
			name:   "Should reject per-kind breaker threshold",
			path:   "loops.defaults.delivery.breaker.threshold",
			denial: ConfigPathForbidden,
		},
		{
			name:   "Should reject per-kind breaker probe interval",
			path:   "loops.defaults.watch.breaker.probe_interval",
			denial: ConfigPathForbidden,
		},
		{
			name:   "Should reject unknown mutable path",
			path:   "unknown.value",
			denial: ConfigPathForbidden,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			path, err := ParseDottedConfigPath(tc.path)
			if err != nil {
				t.Fatalf("ParseDottedConfigPath() error = %v", err)
			}
			policy, err := ClassifyToolConfigPath(path)
			if err != nil {
				t.Fatalf("ClassifyToolConfigPath() error = %v", err)
			}
			if policy.Denial != tc.denial {
				t.Fatalf("PathPolicy.Denial = %q, want %q", policy.Denial, tc.denial)
			}
			if tc.denial == ConfigPathAllowed && policy.Kind != tc.kind {
				t.Fatalf("PathPolicy.Kind = %d, want %d", policy.Kind, tc.kind)
			}
			if policy.Redacted != tc.redacted {
				t.Fatalf("PathPolicy.Redacted = %t, want %t", policy.Redacted, tc.redacted)
			}
		})
	}
}

func TestToolConfigPathPolicyShouldExposeOnlyCurrentNetworkPaths(t *testing.T) {
	t.Parallel()

	allowed := map[string]ValueKind{
		"network.live.defaults.max_wakes":           ConfigValueInt,
		"network.live.defaults.max_wake_wall_time":  ConfigValueString,
		"network.live.defaults.max_total_wall_time": ConfigValueString,
		"network.live.defaults.max_input_tokens":    ConfigValueInt64,
		"network.live.defaults.max_output_tokens":   ConfigValueInt64,
		"network.live.defaults.max_wake_depth":      ConfigValueInt,
		"network.live.defaults.coalesce_window":     ConfigValueString,
		networkLiveLimitsMaxWakesPath:               ConfigValueInt,
		"network.live.limits.max_wake_wall_time":    ConfigValueString,
		"network.live.limits.max_total_wall_time":   ConfigValueString,
		"network.live.limits.max_input_tokens":      ConfigValueInt64,
		"network.live.limits.max_output_tokens":     ConfigValueInt64,
		"network.live.limits.max_wake_depth":        ConfigValueInt,
		networkLiveLimitsMinCoalesceWindowPath:      ConfigValueString,
		"network.live.limits.max_coalesce_window":   ConfigValueString,
	}
	for pathValue, wantKind := range allowed {
		t.Run("Should allow "+pathValue, func(t *testing.T) {
			t.Parallel()

			path, err := ParseDottedConfigPath(pathValue)
			if err != nil {
				t.Fatalf("ParseDottedConfigPath() error = %v", err)
			}
			policy, err := ClassifyToolConfigPath(path)
			if err != nil {
				t.Fatalf("ClassifyToolConfigPath() error = %v", err)
			}
			if policy.Denial != ConfigPathAllowed {
				t.Fatalf("PathPolicy.Denial = %q, want allowed", policy.Denial)
			}
			if policy.Kind != wantKind {
				t.Fatalf("PathPolicy.Kind = %d, want %d", policy.Kind, wantKind)
			}
		})
	}

	removed := []string{
		"network.default_channel",
		"network.port",
		"network.max_payload",
		"network.activation_top_k",
		"network.digest_flush_interval",
		"network.digest_max_envelopes",
		"network.response_guidance_max_bytes",
		"network.delivery_structured_body_max_bytes",
	}
	for _, pathValue := range removed {
		t.Run("Should reject "+pathValue, func(t *testing.T) {
			t.Parallel()

			path, err := ParseDottedConfigPath(pathValue)
			if err != nil {
				t.Fatalf("ParseDottedConfigPath() error = %v", err)
			}
			policy, err := ClassifyToolConfigPath(path)
			if err != nil {
				t.Fatalf("ClassifyToolConfigPath() error = %v", err)
			}
			if policy.Denial != ConfigPathForbidden {
				t.Fatalf("PathPolicy.Denial = %q, want %q", policy.Denial, ConfigPathForbidden)
			}
		})
	}
}

func TestNormalizeToolConfigValue(t *testing.T) {
	t.Run("Should normalize supported kinds and reject malformed values", func(t *testing.T) {
		t.Parallel()

		boolValue, err := NormalizeToolConfigValue(ConfigValueBool, "true")
		if err != nil {
			t.Fatalf("NormalizeToolConfigValue(bool) error = %v", err)
		}
		if boolValue != true {
			t.Fatalf("NormalizeToolConfigValue(bool) = %#v, want true", boolValue)
		}

		intValue, err := NormalizeToolConfigValue(ConfigValueInt, float64(7))
		if err != nil {
			t.Fatalf("NormalizeToolConfigValue(int) error = %v", err)
		}
		if intValue != 7 {
			t.Fatalf("NormalizeToolConfigValue(int) = %#v, want 7", intValue)
		}

		int64Value, err := NormalizeToolConfigValue(ConfigValueInt64, "922337203685477580")
		if err != nil {
			t.Fatalf("NormalizeToolConfigValue(int64) error = %v", err)
		}
		if int64Value != int64(922337203685477580) {
			t.Fatalf("NormalizeToolConfigValue(int64) = %#v, want int64", int64Value)
		}

		uint64Value, err := NormalizeToolConfigValue(ConfigValueUint64, "18446744073709551615")
		if err != nil {
			t.Fatalf("NormalizeToolConfigValue(uint64) error = %v", err)
		}
		if uint64Value != uint64(18446744073709551615) {
			t.Fatalf("NormalizeToolConfigValue(uint64) = %#v, want max uint64", uint64Value)
		}
		invalidValues := []struct {
			value       any
			wantMessage string
		}{
			{value: "-1", wantMessage: `parse unsigned integer value "-1"`},
			{value: "18446744073709551616", wantMessage: "value out of range"},
			{value: float64(1<<53) + 2, wantMessage: "expected unsigned integer value"},
		}
		for _, invalid := range invalidValues {
			if _, err := NormalizeToolConfigValue(ConfigValueUint64, invalid.value); err == nil ||
				!strings.Contains(err.Error(), invalid.wantMessage) {
				t.Fatalf(
					"NormalizeToolConfigValue(uint64 %#v) error = %v, want message containing %q",
					invalid.value,
					err,
					invalid.wantMessage,
				)
			}
		}

		floatValue, err := NormalizeToolConfigValue(ConfigValueFloat, "1.25")
		if err != nil {
			t.Fatalf("NormalizeToolConfigValue(float) error = %v", err)
		}
		if floatValue != 1.25 {
			t.Fatalf("NormalizeToolConfigValue(float) = %#v, want 1.25", floatValue)
		}

		durationValue, err := NormalizeToolConfigValue(ConfigValueDuration, "5s")
		if err != nil {
			t.Fatalf("NormalizeToolConfigValue(duration) error = %v", err)
		}
		if durationValue != "5s" {
			t.Fatalf("NormalizeToolConfigValue(duration) = %#v, want 5s", durationValue)
		}

		value, err := NormalizeToolConfigValue(ConfigValueStringSlice, []any{"codex", "claude"})
		if err != nil {
			t.Fatalf("NormalizeToolConfigValue(string slice) error = %v", err)
		}
		values, ok := value.([]string)
		if !ok || len(values) != 2 || values[0] != "codex" || values[1] != "claude" {
			t.Fatalf("NormalizeToolConfigValue(string slice) = %#v, want two strings", value)
		}
		tableValue, err := NormalizeToolConfigValue(ConfigValueTable, map[string]any{
			"auto_title": map[string]any{
				"fallback_chain": []any{map[string]any{"provider": "codex", "model": "gpt-5-mini"}},
			},
		})
		if err != nil {
			t.Fatalf("NormalizeToolConfigValue(table) error = %v", err)
		}
		table, ok := tableValue.(map[string]any)
		if !ok || table["auto_title"] == nil {
			t.Fatalf("NormalizeToolConfigValue(table) = %#v, want normalized object", tableValue)
		}
		loopRuntime, err := NormalizeToolConfigValue(ConfigValueLoopInput, map[string]any{
			"provider":  "codex",
			"model":     "gpt-5.6",
			"reasoning": "high",
		})
		if err != nil {
			t.Fatalf("NormalizeToolConfigValue(loop runtime) error = %v", err)
		}
		runtimeTable, ok := loopRuntime.(map[string]any)
		if !ok || runtimeTable["provider"] != "codex" || runtimeTable["reasoning"] != "high" {
			t.Fatalf("NormalizeToolConfigValue(loop runtime) = %#v, want runtime object", loopRuntime)
		}

		if _, err := NormalizeToolConfigValue(ConfigValueDuration, "not-a-duration"); err == nil {
			t.Fatal("NormalizeToolConfigValue(invalid duration) error = nil, want non-nil")
		}
		if _, err := NormalizeToolConfigValue(ConfigValueInt, float64(1.5)); err == nil {
			t.Fatal("NormalizeToolConfigValue(non-integral int) error = nil, want non-nil")
		}
	})
}

func TestRedactedConfigMapEntriesAndDiff(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	cfg := DefaultWithHome(homePaths)
	cfg.Defaults.Agent = "planner"
	cfg.Roles.Coordinator.Enabled = true
	cfg.Sandboxes["dev"] = SandboxProfile{
		Backend: "local",
		Env: map[string]string{
			"TOKEN": "secret",
		},
	}
	cfg.Providers["private"] = ProviderConfig{
		Command:      "provider-acp",
		AuthMode:     ProviderAuthModeNativeCLI,
		AuthLoginCmd: "provider login --token raw-login-secret",
	}
	cfg.Loops.Inputs = LoopInputDefaults{
		"release": {
			"settings": map[string]any{
				"mode": "safe", "password": "loop-secret", "nested": map[string]any{"api_token": "nested-secret"},
			},
		},
	}

	configMap := RedactedConfigMap(&cfg)
	entries := FlattenConfigEntries(configMap)
	agent, ok := EntryByPath(entries, "defaults.agent")
	if !ok || agent.Value != "planner" {
		t.Fatalf("EntryByPath(defaults.agent) = %#v/%v, want planner", agent, ok)
	}
	coordinatorEnabled, ok := EntryByPath(entries, "roles.coordinator.enabled")
	if !ok || coordinatorEnabled.Value != true {
		t.Fatalf(
			"EntryByPath(roles.coordinator.enabled) = %#v/%v, want true",
			coordinatorEnabled,
			ok,
		)
	}
	if nested, exists := EntryByPath(entries, "roles.coordinator.roleconfig.enabled"); exists {
		t.Fatalf("embedded coordinator path leaked as %#v", nested)
	}
	soulEnabled, ok := EntryByPath(entries, "agents.soul.enabled")
	if !ok || soulEnabled.Value != true {
		t.Fatalf("EntryByPath(agents.soul.enabled) = %#v/%v, want true", soulEnabled, ok)
	}
	soulMaxBody, ok := EntryByPath(entries, "agents.soul.max_body_bytes")
	if !ok || soulMaxBody.Value != int64(32768) {
		t.Fatalf("EntryByPath(agents.soul.max_body_bytes) = %#v/%v, want 32768", soulMaxBody, ok)
	}
	heartbeatMinInterval, ok := EntryByPath(entries, "agents.heartbeat.min_interval")
	if !ok || heartbeatMinInterval.Value != "5m0s" {
		t.Fatalf(
			"EntryByPath(agents.heartbeat.min_interval) = %#v/%v, want 5m0s",
			heartbeatMinInterval,
			ok,
		)
	}
	heartbeatMaxWakes, ok := EntryByPath(entries, "agents.heartbeat.max_wakes_per_cycle")
	if !ok || heartbeatMaxWakes.Value != int64(25) {
		t.Fatalf(
			"EntryByPath(agents.heartbeat.max_wakes_per_cycle) = %#v/%v, want 25",
			heartbeatMaxWakes,
			ok,
		)
	}
	env, ok := EntryByPath(entries, "sandboxes.dev.env.TOKEN")
	if !ok || env.Value != RedactedValue() || !env.Redacted {
		t.Fatalf("EntryByPath(env) = %#v/%v, want redacted env", env, ok)
	}
	login, ok := EntryByPath(entries, "providers.private.auth_login_command")
	if !ok || login.Value != RedactedValue() || !login.Redacted {
		t.Fatalf("EntryByPath(auth_login_command) = %#v/%v, want redacted login command", login, ok)
	}
	if configText := fmt.Sprint(configMap); strings.Contains(configText, "raw-login-secret") {
		t.Fatalf("RedactedConfigMap leaked login command: %s", configText)
	}
	for _, path := range []string{
		"loops.inputs.release.settings.password",
		"loops.inputs.release.settings.nested.api_token",
	} {
		entry, exists := EntryByPath(entries, path)
		if !exists || entry.Value != RedactedValue() || !entry.Redacted {
			t.Fatalf("EntryByPath(%s) = %#v/%v, want redacted Loop input member", path, entry, exists)
		}
	}
	mode, exists := EntryByPath(entries, "loops.inputs.release.settings.mode")
	if !exists || mode.Value != "safe" || mode.Redacted {
		t.Fatalf("Loop input mode = %#v/%v, want visible safe value", mode, exists)
	}

	before := FlattenConfigEntries(RedactedConfigMap(&Config{Defaults: DefaultsConfig{Agent: DefaultAgentName}}))
	diff := DiffConfigEntries(before, entries)
	if len(diff) == 0 {
		t.Fatal("DiffConfigEntries() returned no differences")
	}
	found := false
	for _, entry := range diff {
		if entry.Path == "defaults.agent" && entry.After == "planner" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("DiffConfigEntries() = %#v, want defaults.agent change", diff)
	}
}

func TestConfigOverlayHookDeclarationsAndValues(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	target, err := ResolveConfigWriteTarget(homePaths, "", WriteScopeUser, "")
	if err != nil {
		t.Fatalf("ResolveConfigWriteTarget() error = %v", err)
	}

	enabled := true
	readOnly := true
	decl := hookspkg.HookDecl{
		Name:        "tool-audit",
		Event:       hookspkg.HookToolPreCall,
		Source:      hookspkg.HookSourceConfig,
		Mode:        hookspkg.HookModeSync,
		Required:    true,
		Priority:    42,
		PrioritySet: true,
		Timeout:     2 * time.Second,
		Enabled:     &enabled,
		Matcher: hookspkg.HookMatcher{
			AgentName:    "general",
			ToolReadOnly: &readOnly,
		},
		Command: "/bin/echo",
		Args:    []string{"audit"},
		Env: map[string]string{
			"PHASE": "test",
		},
	}
	if _, err := EditConfigOverlay(homePaths, "", target, func(editor *OverlayEditor) error {
		return editor.UpsertArrayTableItem(
			[]string{"hooks", "declarations"},
			"name",
			decl.Name,
			HookDeclarationOverlayValues(decl),
		)
	}); err != nil {
		t.Fatalf("EditConfigOverlay(hook) error = %v", err)
	}

	decls, err := OverlayHookDeclarations(target)
	if err != nil {
		t.Fatalf("OverlayHookDeclarations() error = %v", err)
	}
	if len(decls) != 1 {
		t.Fatalf("len(OverlayHookDeclarations()) = %d, want 1", len(decls))
	}
	got := decls[0]
	if got.Name != decl.Name ||
		got.Event != decl.Event ||
		got.Mode != decl.Mode ||
		!got.Required ||
		got.Priority != decl.Priority ||
		got.Timeout != decl.Timeout ||
		got.Command != decl.Command ||
		got.Env["PHASE"] != "test" ||
		got.Enabled == nil ||
		!*got.Enabled ||
		got.Matcher.ToolReadOnly == nil ||
		!*got.Matcher.ToolReadOnly {
		t.Fatalf("OverlayHookDeclarations() = %#v, want round-tripped hook", got)
	}
}
