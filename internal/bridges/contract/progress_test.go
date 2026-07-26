// Suite: bridge progress wire contract
// Invariant: progress payloads are closed, one-line, redacted, and resolve deterministic defaults.
// Boundary IN: provider-facing progress payloads and persisted delivery_defaults JSON.
// Boundary OUT: daemon event projection and provider rendering, owned by their integration suites.
package contract

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestToolProgressContract(t *testing.T) {
	t.Parallel()

	t.Run("Should expose and validate every progress phase", func(t *testing.T) {
		t.Parallel()

		if got, want := strings.Join(ToolProgressPhaseValues(), ","), "started,completed,failed"; got != want {
			t.Fatalf("ToolProgressPhaseValues() = %q, want %q", got, want)
		}

		tests := []struct {
			name  string
			phase ToolProgressPhase
			want  ToolProgressPhase
		}{
			{name: "Should normalize the started phase", phase: " STARTED ", want: ToolProgressPhaseStarted},
			{name: "Should normalize the completed phase", phase: " Completed ", want: ToolProgressPhaseCompleted},
			{name: "Should normalize the failed phase", phase: " failed ", want: ToolProgressPhaseFailed},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				if got := test.phase.Normalize(); got != test.want {
					t.Fatalf("ToolProgressPhase(%q).Normalize() = %q, want %q", test.phase, got, test.want)
				}
				if err := test.phase.Validate(); err != nil {
					t.Fatalf("ToolProgressPhase(%q).Validate() error = %v", test.phase, err)
				}
			})
		}

		invalid := []struct {
			name  string
			phase ToolProgressPhase
		}{
			{name: "Should reject an empty progress phase", phase: ""},
			{name: "Should reject an unsupported progress phase", phase: "waiting"},
		}
		for _, test := range invalid {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				if err := test.phase.Validate(); err == nil {
					t.Fatalf("ToolProgressPhase(%q).Validate() error = nil", test.phase)
				}
			})
		}
	})

	t.Run("Should accept a complete redacted one-line progress payload", func(t *testing.T) {
		t.Parallel()

		progress := ToolProgress{
			ToolCallID: " call-1 ",
			ToolID:     " agh__task_read ",
			Phase:      " STARTED ",
			Label:      " Reading task ",
			Preview:    " task-42 ",
			Emoji:      " 🔧 ",
			DurationMS: 125,
			Index:      1,
		}
		if err := progress.Validate(); err != nil {
			t.Fatalf("ToolProgress.Validate() error = %v", err)
		}
		normalized := NormalizeToolProgress(progress)
		if normalized.ToolCallID != "call-1" || normalized.ToolID != "agh__task_read" ||
			normalized.Phase != ToolProgressPhaseStarted || normalized.Label != "Reading task" ||
			normalized.Preview != "task-42" || normalized.Emoji != "🔧" {
			t.Fatalf("NormalizeToolProgress() = %#v, want canonical progress", normalized)
		}
		if progress.ToolCallID != " call-1 " || progress.Label != " Reading task " {
			t.Fatalf("NormalizeToolProgress() mutated source = %#v", progress)
		}
	})

	t.Run("Should reject malformed or secret-bearing progress payloads", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name   string
			mutate func(*ToolProgress)
		}{
			{name: "Should require a tool call id", mutate: func(v *ToolProgress) { v.ToolCallID = "" }},
			{name: "Should require a tool id", mutate: func(v *ToolProgress) { v.ToolID = "" }},
			{name: "Should require a supported phase", mutate: func(v *ToolProgress) { v.Phase = "waiting" }},
			{name: "Should reject a negative duration", mutate: func(v *ToolProgress) { v.DurationMS = -1 }},
			{name: "Should require a positive index", mutate: func(v *ToolProgress) { v.Index = 0 }},
			{name: "Should reject a multiline label", mutate: func(v *ToolProgress) { v.Label = "Reading\ntask" }},
			{name: "Should reject a multiline preview", mutate: func(v *ToolProgress) { v.Preview = "first\rsecond" }},
			{name: "Should reject a multiline emoji", mutate: func(v *ToolProgress) { v.Emoji = "🔧\nicon" }},
			{name: "Should reject a multiline error", mutate: func(v *ToolProgress) { v.Error = "failed\nretry" }},
			{
				name:   "Should reject a secret-bearing preview",
				mutate: func(v *ToolProgress) { v.Preview = "api_key=sk-contract-secret" },
			},
			{
				name:   "Should reject a secret-bearing error",
				mutate: func(v *ToolProgress) { v.Error = "password=hunter2" },
			},
			{
				name:   "Should reject an oversized error",
				mutate: func(v *ToolProgress) { v.Error = strings.Repeat("é", maxToolProgressErrorRunes+1) },
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				progress := ToolProgress{
					ToolCallID: "call-1",
					ToolID:     "agh__task_read",
					Phase:      ToolProgressPhaseStarted,
					Label:      "Reading task",
					Preview:    "task-42",
					Emoji:      "🔧",
					Index:      1,
				}
				test.mutate(&progress)
				if err := progress.Validate(); err == nil {
					t.Fatalf("ToolProgress.Validate(%s) error = nil", test.name)
				}
			})
		}
	})
}

func TestProgressConfigContract(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize progress defaults and verbose preview behavior", func(t *testing.T) {
		t.Parallel()

		defaults := NormalizeProgressConfig(ProgressConfig{
			ToolProgress: " ALL ", Grouping: " SEPARATE ", Typing: true, Reactions: true,
		})
		if defaults.ToolProgress != ProgressModeAll || defaults.Grouping != ProgressGroupingSeparate ||
			!defaults.Typing || !defaults.Reactions || defaults.PreviewLimit != defaultProgressPreviewLimit ||
			defaults.EditInterval != defaultProgressEditInterval {
			t.Fatalf("NormalizeProgressConfig(defaults) = %#v, want canonical defaults", defaults)
		}

		verbose := NormalizeProgressConfig(ProgressConfig{
			ToolProgress: ProgressModeVerbose, Grouping: ProgressGroupingAccumulate,
		})
		if verbose.PreviewLimit != 0 || verbose.EditInterval != defaultProgressEditInterval {
			t.Fatalf(
				"NormalizeProgressConfig(verbose) = %#v, want unlimited preview and default edit interval",
				verbose,
			)
		}
	})

	t.Run("Should validate every progress mode and grouping", func(t *testing.T) {
		t.Parallel()

		modes := []struct {
			name string
			mode ProgressMode
			want ProgressMode
		}{
			{name: "Should normalize the off mode", mode: " OFF ", want: ProgressModeOff},
			{name: "Should normalize the new mode", mode: " New ", want: ProgressModeNew},
			{name: "Should normalize the all mode", mode: " all ", want: ProgressModeAll},
			{name: "Should normalize the verbose mode", mode: " VERBOSE ", want: ProgressModeVerbose},
		}
		for _, test := range modes {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				if got := test.mode.Normalize(); got != test.want {
					t.Fatalf("ProgressMode(%q).Normalize() = %q, want %q", test.mode, got, test.want)
				}
				if err := test.mode.Validate(); err != nil {
					t.Fatalf("ProgressMode(%q).Validate() error = %v", test.mode, err)
				}
			})
		}

		groupings := []struct {
			name     string
			grouping ProgressGrouping
			want     ProgressGrouping
		}{
			{name: "Should normalize accumulate grouping", grouping: " ACCUMULATE ", want: ProgressGroupingAccumulate},
			{name: "Should normalize separate grouping", grouping: " Separate ", want: ProgressGroupingSeparate},
		}
		for _, test := range groupings {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				if got := test.grouping.Normalize(); got != test.want {
					t.Fatalf("ProgressGrouping(%q).Normalize() = %q, want %q", test.grouping, got, test.want)
				}
				if err := test.grouping.Validate(); err != nil {
					t.Fatalf("ProgressGrouping(%q).Validate() error = %v", test.grouping, err)
				}
			})
		}
	})

	t.Run("Should reject invalid progress configuration fields", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name   string
			config ProgressConfig
		}{
			{name: "Should require a progress mode", config: ProgressConfig{Grouping: ProgressGroupingAccumulate}},
			{
				name:   "Should reject an unsupported progress mode",
				config: ProgressConfig{ToolProgress: "sometimes", Grouping: ProgressGroupingAccumulate},
			},
			{name: "Should require a progress grouping", config: ProgressConfig{ToolProgress: ProgressModeAll}},
			{
				name:   "Should reject an unsupported progress grouping",
				config: ProgressConfig{ToolProgress: ProgressModeAll, Grouping: "threaded"},
			},
			{
				name: "Should reject a negative preview limit",
				config: ProgressConfig{
					ToolProgress: ProgressModeAll,
					Grouping:     ProgressGroupingAccumulate,
					PreviewLimit: -1,
				},
			},
			{
				name: "Should reject a negative edit interval",
				config: ProgressConfig{
					ToolProgress: ProgressModeAll,
					Grouping:     ProgressGroupingAccumulate,
					EditInterval: -time.Second,
				},
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				if err := test.config.Validate(); err == nil {
					t.Fatalf("ProgressConfig.Validate(%s) error = nil", test.name)
				}
			})
		}
	})

	t.Run("Should resolve instance overrides and shipped platform defaults", func(t *testing.T) {
		t.Parallel()

		t.Run("Should prefer an explicit instance override", func(t *testing.T) {
			t.Parallel()

			instance := &BridgeInstance{
				Platform: "slack",
				DeliveryDefaults: json.RawMessage(
					`{"progress":{"tool_progress":"all","grouping":"separate","typing":true,"reactions":false}}`,
				),
			}
			override := ResolveProgressConfig(instance, "")
			if override.ToolProgress != ProgressModeAll || override.Grouping != ProgressGroupingSeparate ||
				!override.Typing || override.Reactions || override.PreviewLimit != defaultProgressPreviewLimit ||
				override.EditInterval != defaultProgressEditInterval {
				t.Fatalf("ResolveProgressConfig(instance) = %#v, want effective explicit override", override)
			}
		})

		t.Run("Should leave verbose preview unlimited", func(t *testing.T) {
			t.Parallel()

			verbose := ResolveProgressConfig(&BridgeInstance{DeliveryDefaults: json.RawMessage(
				`{"progress":{"tool_progress":"verbose","grouping":"accumulate","typing":false,"reactions":false}}`,
			)}, "unknown")
			if verbose.PreviewLimit != 0 || verbose.EditInterval != defaultProgressEditInterval {
				t.Fatalf(
					"ResolveProgressConfig(verbose) = %#v, want unlimited preview and default edit interval",
					verbose,
				)
			}
		})

		enabledPlatforms := []string{"slack", "telegram", "discord"}
		for _, platform := range enabledPlatforms {
			t.Run("Should enable shipped affordances for "+platform, func(t *testing.T) {
				t.Parallel()

				got := ResolveProgressConfig(nil, platform)
				if got.ToolProgress != ProgressModeNew || got.Grouping != ProgressGroupingAccumulate ||
					!got.Typing || !got.Reactions || got.PreviewLimit != defaultProgressPreviewLimit ||
					got.EditInterval != defaultProgressEditInterval {
					t.Fatalf("ResolveProgressConfig(nil, %q) = %#v, want shipped enabled defaults", platform, got)
				}
			})
		}

		disabledPlatforms := []string{"teams", "whatsapp", "gchat", "github", "linear", "unknown"}
		for _, platform := range disabledPlatforms {
			t.Run("Should keep progress disabled for "+platform, func(t *testing.T) {
				t.Parallel()

				got := ResolveProgressConfig(nil, platform)
				if got.ToolProgress != ProgressModeOff || got.Grouping != ProgressGroupingAccumulate ||
					got.Typing || got.Reactions {
					t.Fatalf("ResolveProgressConfig(nil, %q) = %#v, want disabled defaults", platform, got)
				}
			})
		}

		t.Run("Should fall back to the instance platform when an override is malformed", func(t *testing.T) {
			t.Parallel()

			malformed := &BridgeInstance{
				Platform:         "telegram",
				DeliveryDefaults: json.RawMessage(`{"progress":{"tool_progress":"bad"}}`),
			}
			fallback := ResolveProgressConfig(malformed, "")
			if fallback.ToolProgress != ProgressModeNew || !fallback.Typing || !fallback.Reactions {
				t.Fatalf("ResolveProgressConfig(malformed override) = %#v, want platform fallback", fallback)
			}
		})
	})
}

func TestNormalizeDeliveryDefaultsJSONContract(t *testing.T) {
	t.Parallel()

	t.Run("Should canonicalize progress and delivery mode while preserving provider strings", func(t *testing.T) {
		t.Parallel()

		got, err := NormalizeDeliveryDefaultsJSON(json.RawMessage(
			` {"progress":{"tool_progress":" ALL ","grouping":" Separate ","typing":true,"reactions":false},"parse_mode":"MarkdownV2","thread_id":"thread-1","mode":" DIRECT_SEND "} `,
		))
		if err != nil {
			t.Fatalf("NormalizeDeliveryDefaultsJSON() error = %v", err)
		}
		want := `{"mode":"direct-send","parse_mode":"MarkdownV2","progress":{"tool_progress":"all","grouping":"separate","typing":true,"reactions":false},"thread_id":"thread-1"}`
		if string(got) != want {
			t.Fatalf("NormalizeDeliveryDefaultsJSON() = %s, want %s", got, want)
		}
	})

	t.Run("Should normalize empty and null defaults to nil", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			raw  json.RawMessage
		}{
			{name: "Should normalize absent defaults to nil", raw: nil},
			{name: "Should normalize whitespace defaults to nil", raw: json.RawMessage("  ")},
			{name: "Should normalize JSON null defaults to nil", raw: json.RawMessage(" null ")},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				got, err := NormalizeDeliveryDefaultsJSON(test.raw)
				if err != nil || got != nil {
					t.Fatalf("NormalizeDeliveryDefaultsJSON(%q) = (%s, %v), want nil, nil", test.raw, got, err)
				}
			})
		}
	})

	t.Run("Should reject malformed or wrongly typed delivery defaults", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			raw  json.RawMessage
		}{
			{name: "Should reject malformed JSON", raw: json.RawMessage(`{`)},
			{name: "Should reject a non-object payload", raw: json.RawMessage(`[]`)},
			{name: "Should reject a non-string provider field", raw: json.RawMessage(`{"parse_mode":1}`)},
			{name: "Should reject an unsupported delivery mode", raw: json.RawMessage(`{"mode":"broadcast"}`)},
			{
				name: "Should reject an unsupported progress mode",
				raw: json.RawMessage(
					`{"progress":{"tool_progress":"log","grouping":"accumulate","typing":false,"reactions":false}}`,
				),
			},
			{
				name: "Should reject an unsupported grouping",
				raw: json.RawMessage(
					`{"progress":{"tool_progress":"all","grouping":"bubble","typing":false,"reactions":false}}`,
				),
			},
			{
				name: "Should reject non-boolean typing",
				raw: json.RawMessage(
					`{"progress":{"tool_progress":"all","grouping":"accumulate","typing":"yes","reactions":false}}`,
				),
			},
			{
				name: "Should reject non-boolean reactions",
				raw: json.RawMessage(
					`{"progress":{"tool_progress":"all","grouping":"accumulate","typing":false,"reactions":1}}`,
				),
			},
			{
				name: "Should reject unknown progress fields",
				raw: json.RawMessage(
					`{"progress":{"tool_progress":"all","grouping":"accumulate","typing":false,"reactions":false,"raw_args":true}}`,
				),
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				if _, err := NormalizeDeliveryDefaultsJSON(test.raw); err == nil {
					t.Fatalf("NormalizeDeliveryDefaultsJSON(%s) error = nil", test.raw)
				}
			})
		}
	})
}
