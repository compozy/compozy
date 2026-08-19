package config

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/loop/dsl"
	speedpkg "github.com/compozy/compozy/internal/speed"
)

func TestLoopsConfigShouldLoadDefaultsAndOverlays(t *testing.T) {
	t.Parallel()

	t.Run("Should load delivery and watch config defaults", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}

		cfg := DefaultWithHome(homePaths)

		assertLoopDefaultConfig(t, "delivery", cfg.Loops.Defaults.Delivery, loopDefaultWant{
			iterationCap:     50,
			noProgressWindow: 3,
			gateMaxRevisions: 10,
			budgetTokens:     0,
			budgetWallSec:    0,
			budgetOnExceeded: string(dsl.BudgetExceededHalt),
			fanOutWidth:      4,
		})
		assertLoopDefaultConfig(t, "watch", cfg.Loops.Defaults.Watch, loopDefaultWant{
			iterationCap:     0,
			noProgressWindow: 2,
			gateMaxRevisions: 0,
			budgetTokens:     0,
			budgetWallSec:    0,
			budgetOnExceeded: string(dsl.BudgetExceededHalt),
			fanOutWidth:      2,
		})
		assertLoopLifecycleDefaults(t, "delivery", cfg.Loops.Defaults.Delivery)
		assertLoopLifecycleDefaults(t, "watch", cfg.Loops.Defaults.Watch)
		if cfg.Loops.Breaker.Threshold != 5 || cfg.Loops.Breaker.ProbeInterval != "60s" {
			t.Fatalf("breaker defaults = %#v, want threshold 5 and probe interval 60s", cfg.Loops.Breaker)
		}
	})

	t.Run("Should apply global and workspace overlays with zero values preserved", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		workspaceRoot := t.TempDir()

		writeFile(t, homePaths.ConfigFile, `
[loops.defaults.delivery]
iteration_cap = 40
fan_out_width = 3

[loops.defaults.delivery.no_progress]
window = 2

[loops.defaults.delivery.gates]
max_revisions = 9

[loops.defaults.delivery.runtime_defaults.worker]
model = "global-worker"
speed = "fast"

[loops.defaults.delivery.runtime_defaults.judge]
model = "global-judge"

[[loops.defaults.delivery.runtime_rules]]
[loops.defaults.delivery.runtime_rules.match]
complexity = "high"
[loops.defaults.delivery.runtime_rules.runtime]
model = "global-complexity-model"

[loops.defaults.delivery.budget]
tokens = 100
wall_clock_sec = 60
on_exceeded = "escalate"

[loops.defaults.delivery.retry]
max_attempts = 0
backoff_base = "2s"
backoff_max = "45s"

[loops.defaults.delivery.requests]
expire_after = "72h"

[[loops.defaults.delivery.autopause]]
match = "class == 'transport'"
action = "pause"

[loops.defaults.watch]
iteration_cap = 1
fan_out_width = 1

[loops.defaults.watch.no_progress]
window = 1

[loops.defaults.watch.runtime_defaults.judge]
model = "global-watch-judge"

[loops.inputs.review-and-fix]
auto_commit = false
retries = 0
reviewer = ""
`)
		writeFile(t, filepath.Join(workspaceRoot, DirName, ConfigName), `
[loops.defaults.delivery]
iteration_cap = 0
fan_out_width = 2

[loops.defaults.delivery.budget]
tokens = 0
on_exceeded = "halt"

[loops.defaults.delivery.runtime_defaults.worker]
model = "workspace-worker"
speed = "normal"

[[loops.defaults.delivery.autopause]]
match = "attempt >= 2"
action = "pause"

[[loops.defaults.delivery.runtime_rules]]
[loops.defaults.delivery.runtime_rules.match]
type = "frontend"
[loops.defaults.delivery.runtime_rules.runtime]
model = "workspace-frontend-model"

[loops.defaults.watch]
fan_out_width = 5

[loops.defaults.watch.no_progress]
window = 4

[loops.defaults.watch.gates]
max_revisions = 7

[loops.defaults.watch.requests]
expire_after = "24h"

[loops.defaults.watch.runtime_defaults.judge]
model = "workspace-watch-judge"

[loops.inputs.review-and-fix]
auto_commit = true
fixer = ""
`)

		cfg, err := LoadForHome(homePaths, WithWorkspaceRoot(workspaceRoot))
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}

		assertLoopDefaultConfig(t, "delivery", cfg.Loops.Defaults.Delivery, loopDefaultWant{
			iterationCap:       0,
			noProgressWindow:   2,
			gateMaxRevisions:   9,
			budgetTokens:       0,
			budgetWallSec:      60,
			budgetOnExceeded:   string(dsl.BudgetExceededHalt),
			fanOutWidth:        2,
			workerModel:        "workspace-worker",
			workerSpeed:        speedpkg.SpeedNormal,
			judgeModel:         "global-judge",
			requestExpireAfter: "72h",
		})
		assertLoopDefaultConfig(t, "watch", cfg.Loops.Defaults.Watch, loopDefaultWant{
			iterationCap:       1,
			noProgressWindow:   4,
			gateMaxRevisions:   7,
			budgetTokens:       0,
			budgetWallSec:      0,
			budgetOnExceeded:   string(dsl.BudgetExceededHalt),
			fanOutWidth:        5,
			judgeModel:         "workspace-watch-judge",
			requestExpireAfter: "24h",
		})
		rules := cfg.Loops.Defaults.Delivery.RuntimeRules
		if len(rules) != 2 || rules[0].Match.Complexity != "high" ||
			rules[1].Match.Type != "frontend" || rules[1].Runtime.Model != "workspace-frontend-model" {
			t.Fatalf("delivery runtime rules = %#v, want ordered global/workspace rules", rules)
		}
		if got := cfg.Loops.Defaults.Delivery.Retry; got.MaxAttempts != 0 ||
			got.BackoffBase != "2s" || got.BackoffMax != "45s" {
			t.Fatalf("delivery retry = %#v, want explicit zero attempts with inherited bounds", got)
		}
		wantAutopause := []LoopAutopauseRule{
			{Match: "class == 'transport'", Action: "pause"},
			{Match: "attempt >= 2", Action: "pause"},
		}
		if got := cfg.Loops.Defaults.Delivery.Autopause; !reflect.DeepEqual(got, wantAutopause) {
			t.Fatalf("delivery autopause = %#v, want %#v", got, wantAutopause)
		}
		globalInputs, workspaceInputs := cfg.Loops.InputDefaultLayers("review-and-fix")
		if got, ok := workspaceInputs["auto_commit"]; !ok || got != true {
			t.Fatalf("workspace auto_commit = %#v/%v, want explicit true", got, ok)
		}
		if got, ok := globalInputs["retries"]; !ok || got != int64(0) {
			t.Fatalf("global retries = %#v/%v, want explicit int64(0)", got, ok)
		}
		if got, ok := globalInputs["reviewer"]; !ok || got != "" {
			t.Fatalf("global reviewer = %#v/%v, want explicit empty string", got, ok)
		}
		if got, ok := workspaceInputs["fixer"]; !ok || got != "" {
			t.Fatalf("workspace fixer = %#v/%v, want explicit empty string", got, ok)
		}
		if _, leaked := globalInputs["auto_commit"]; leaked {
			t.Fatalf("global inputs expose shadowed auto_commit: %#v", globalInputs)
		}
	})

	t.Run("Should reject retired model defaults with migration guidance", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		writeFile(t, homePaths.ConfigFile, `
[loops.defaults.delivery.model_defaults.worker]
model = "opus"
`)

		_, err = LoadForHome(homePaths)
		if err == nil || !strings.Contains(err.Error(), "model_defaults") ||
			!strings.Contains(err.Error(), "MIGRATION_GUIDE.md#per-task-runtime-selection") {
			t.Fatalf("LoadForHome() error = %v, want retired-key migration guidance", err)
		}
	})
}

func TestLoopsConfigShouldRejectWriteTimeInvalidDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		path      []string
		value     any
		wantError string
	}{
		{
			name:      "Should reject an unsupported runtime speed",
			path:      []string{"loops", "defaults", "delivery", "runtime_defaults", "worker", "speed"},
			value:     "turbo",
			wantError: "loops.defaults.delivery.runtime_defaults.worker.speed",
		},
		{
			name:      "Should reject a negative delivery fan out window",
			path:      []string{"loops", "defaults", "delivery", "fan_out_width"},
			value:     -1,
			wantError: "loops.defaults.delivery.fan_out_width",
		},
		{
			name:      "Should reject watch no-progress above compile-time ceiling",
			path:      []string{"loops", "defaults", "watch", "no_progress", "window"},
			value:     loopDefaultsMaxNoProgressWindow + 1,
			wantError: "loops.defaults.watch.no_progress.window",
		},
		{
			name:      "Should reject gate revisions above compile-time ceiling",
			path:      []string{"loops", "defaults", "delivery", "gates", "max_revisions"},
			value:     dsl.GateMaxRevisionsCeiling + 1,
			wantError: "loops.defaults.delivery.gates.max_revisions",
		},
		{
			name:      "Should reject invalid budget on exceeded policy",
			path:      []string{"loops", "defaults", "delivery", "budget", "on_exceeded"},
			value:     "ignore",
			wantError: "loops.defaults.delivery.budget.on_exceeded",
		},
		{
			name:      "Should reject retry attempts above the lifecycle ceiling",
			path:      []string{"loops", "defaults", "delivery", "retry", "max_attempts"},
			value:     11,
			wantError: "loops.defaults.delivery.retry.max_attempts",
		},
		{
			name:      "Should reject retry backoff bounds in descending order",
			path:      []string{"loops", "defaults", "watch", "retry", "backoff_base"},
			value:     "31s",
			wantError: "loops.defaults.watch.retry.backoff_base",
		},
		{
			name:      "Should reject zero wait admission interval",
			path:      []string{"loops", "defaults", "delivery", "waits", "admission_retry_interval"},
			value:     "0s",
			wantError: "loops.defaults.delivery.waits.admission_retry_interval",
		},
		{
			name:      "Should reject zero request expiry",
			path:      []string{"loops", "defaults", "delivery", "requests", "expire_after"},
			value:     "0s",
			wantError: "loops.defaults.delivery.requests.expire_after",
		},
		{
			name:      "Should reject malformed request expiry",
			path:      []string{"loops", "defaults", "watch", "requests", "expire_after"},
			value:     "eventually",
			wantError: "loops.defaults.watch.requests.expire_after",
		},
		{
			name:      "Should reject a non-positive global breaker threshold",
			path:      []string{"loops", "breaker", "threshold"},
			value:     0,
			wantError: "loops.breaker.threshold",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
			if err != nil {
				t.Fatalf("ResolveHomePathsFrom() error = %v", err)
			}
			target, err := ResolveConfigWriteTarget(homePaths, "", WriteScopeGlobal)
			if err != nil {
				t.Fatalf("ResolveConfigWriteTarget() error = %v", err)
			}

			_, err = EditConfigOverlay(homePaths, "", target, func(editor *OverlayEditor) error {
				return editor.SetValue(tt.path, tt.value)
			})
			if err == nil {
				t.Fatal("EditConfigOverlay() error = nil, want validation failure")
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("EditConfigOverlay() error = %v, want path %q", err, tt.wantError)
			}
			var validationError ValidationError
			if !errors.As(err, &validationError) || validationError.Path != tt.wantError {
				t.Fatalf("EditConfigOverlay() error = %#v, want ValidationError path %q", err, tt.wantError)
			}
		})
	}
}

func TestLoopsConfigShouldAcceptLargeFanOutWidth(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	target, err := ResolveConfigWriteTarget(homePaths, "", WriteScopeGlobal)
	if err != nil {
		t.Fatalf("ResolveConfigWriteTarget() error = %v", err)
	}
	_, err = EditConfigOverlay(homePaths, "", target, func(editor *OverlayEditor) error {
		return editor.SetValue([]string{"loops", "defaults", "delivery", "fan_out_width"}, 500)
	})
	if err != nil {
		t.Fatalf("EditConfigOverlay() error = %v", err)
	}
	loaded, err := LoadForHome(homePaths)
	if err != nil {
		t.Fatalf("LoadForHome() error = %v", err)
	}
	if got, want := loaded.Loops.Defaults.Delivery.FanOutWidth, 500; got != want {
		t.Fatalf("FanOutWidth = %d, want %d", got, want)
	}
}

func TestLoopsConfigShouldRejectInvalidAutopauseWithoutMutatingRules(t *testing.T) {
	t.Parallel()

	valid := LoopAutopauseRule{Match: "class == 'transport' && attempt >= 2", Action: "pause"}
	tests := []struct {
		name        string
		invalid     LoopAutopauseRule
		wantPath    string
		wantMessage string
	}{
		{
			name:        "Should reject an expression that does not compile",
			invalid:     LoopAutopauseRule{Match: "unknown == true", Action: "pause"},
			wantPath:    "loops.defaults.delivery.autopause[1].match",
			wantMessage: "undeclared reference",
		},
		{
			name:        "Should reject a non boolean expression",
			invalid:     LoopAutopauseRule{Match: "attempt", Action: "pause"},
			wantPath:    "loops.defaults.delivery.autopause[1].match",
			wantMessage: "must return bool",
		},
		{
			name:        "Should reject an unsupported action",
			invalid:     LoopAutopauseRule{Match: "attempt >= 2", Action: "resume"},
			wantPath:    "loops.defaults.delivery.autopause[1].action",
			wantMessage: "must be pause",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := DefaultLoopsConfig()
			cfg.Defaults.Delivery.Autopause = []LoopAutopauseRule{valid, tt.invalid}
			before := append([]LoopAutopauseRule(nil), cfg.Defaults.Delivery.Autopause...)

			err := cfg.Validate()
			if err == nil {
				t.Fatal("Validate() error = nil, want invalid autopause rule")
			}
			var validationError ValidationError
			if !errors.As(err, &validationError) || validationError.Code != loopAutopauseRuleInvalidCode ||
				validationError.Path != tt.wantPath || !strings.Contains(validationError.Message, tt.wantMessage) {
				t.Fatalf(
					"Validate() error = %#v, want code %q path %q containing %q",
					err,
					loopAutopauseRuleInvalidCode,
					tt.wantPath,
					tt.wantMessage,
				)
			}
			if got := cfg.Defaults.Delivery.Autopause; !reflect.DeepEqual(got, before) {
				t.Fatalf("Validate() mutated rules = %#v, want %#v", got, before)
			}
		})
	}
}

func TestLoopsConfigShouldExposeAgentMutableToolPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path []string
		kind ValueKind
	}{
		{
			name: "Should allow delivery fan out defaults",
			path: []string{"loops", "defaults", "delivery", "fan_out_width"},
			kind: ConfigValueInt,
		},
		{
			name: "Should allow watch budget policy defaults",
			path: []string{"loops", "defaults", "watch", "budget", "on_exceeded"},
			kind: ConfigValueString,
		},
		{
			name: "Should allow delivery worker runtime defaults",
			path: []string{"loops", "defaults", "delivery", "runtime_defaults", "worker", "model"},
			kind: ConfigValueString,
		},
		{
			name: "Should allow delivery worker speed defaults",
			path: []string{"loops", "defaults", "delivery", "runtime_defaults", "worker", "speed"},
			kind: ConfigValueString,
		},
		{
			name: "Should allow delivery judge speed defaults",
			path: []string{"loops", "defaults", "delivery", "runtime_defaults", "judge", "speed"},
			kind: ConfigValueString,
		},
		{
			name: "Should allow watch worker speed defaults",
			path: []string{"loops", "defaults", "watch", "runtime_defaults", "worker", "speed"},
			kind: ConfigValueString,
		},
		{
			name: "Should allow watch judge speed defaults",
			path: []string{"loops", "defaults", "watch", "runtime_defaults", "judge", "speed"},
			kind: ConfigValueString,
		},
		{
			name: "Should allow request expiry defaults",
			path: []string{"loops", "defaults", "delivery", "requests", "expire_after"},
			kind: ConfigValueString,
		},
		{
			name: "Should allow dynamic per Loop input defaults",
			path: []string{"loops", "inputs", "review-and-fix", "auto_commit"},
			kind: ConfigValueLoopInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			policy, err := ClassifyToolConfigPath(tt.path)
			if err != nil {
				t.Fatalf("ClassifyToolConfigPath() error = %v", err)
			}
			if policy.Denial != ConfigPathAllowed {
				t.Fatalf("ClassifyToolConfigPath() denial = %q, want allowed", policy.Denial)
			}
			if policy.Kind != tt.kind {
				t.Fatalf("ClassifyToolConfigPath() kind = %v, want %v", policy.Kind, tt.kind)
			}
		})
	}
}

type loopDefaultWant struct {
	iterationCap       int
	noProgressWindow   int
	gateMaxRevisions   int
	budgetTokens       int
	budgetWallSec      int
	budgetOnExceeded   string
	fanOutWidth        int
	workerModel        string
	workerSpeed        speedpkg.Speed
	judgeModel         string
	requestExpireAfter string
}

func assertLoopDefaultConfig(t *testing.T, label string, got LoopDefaultConfig, want loopDefaultWant) {
	t.Helper()

	if got.IterationCap != want.iterationCap {
		t.Fatalf("%s IterationCap = %d, want %d", label, got.IterationCap, want.iterationCap)
	}
	if got.NoProgress.Window != want.noProgressWindow {
		t.Fatalf("%s NoProgress.Window = %d, want %d", label, got.NoProgress.Window, want.noProgressWindow)
	}
	if got.Gates.MaxRevisions != want.gateMaxRevisions {
		t.Fatalf("%s Gates.MaxRevisions = %d, want %d", label, got.Gates.MaxRevisions, want.gateMaxRevisions)
	}
	if got.Budget.Tokens != want.budgetTokens {
		t.Fatalf("%s Budget.Tokens = %d, want %d", label, got.Budget.Tokens, want.budgetTokens)
	}
	if got.Budget.WallClockSec != want.budgetWallSec {
		t.Fatalf("%s Budget.WallClockSec = %d, want %d", label, got.Budget.WallClockSec, want.budgetWallSec)
	}
	if got.Budget.OnExceeded != want.budgetOnExceeded {
		t.Fatalf("%s Budget.OnExceeded = %q, want %q", label, got.Budget.OnExceeded, want.budgetOnExceeded)
	}
	if got.FanOutWidth != want.fanOutWidth {
		t.Fatalf("%s FanOutWidth = %d, want %d", label, got.FanOutWidth, want.fanOutWidth)
	}
	if got.RuntimeDefaults.Worker.Model != want.workerModel {
		t.Fatalf(
			"%s RuntimeDefaults.Worker.Model = %q, want %q",
			label,
			got.RuntimeDefaults.Worker.Model,
			want.workerModel,
		)
	}
	if got.RuntimeDefaults.Worker.Speed != want.workerSpeed {
		t.Fatalf(
			"%s RuntimeDefaults.Worker.Speed = %q, want %q",
			label,
			got.RuntimeDefaults.Worker.Speed,
			want.workerSpeed,
		)
	}
	if got.Requests.ExpireAfter != want.requestExpireAfter {
		t.Fatalf(
			"%s Requests.ExpireAfter = %q, want %q",
			label,
			got.Requests.ExpireAfter,
			want.requestExpireAfter,
		)
	}
	if got.RuntimeDefaults.Judge.Model != want.judgeModel {
		t.Fatalf(
			"%s RuntimeDefaults.Judge.Model = %q, want %q",
			label,
			got.RuntimeDefaults.Judge.Model,
			want.judgeModel,
		)
	}
}

func assertLoopLifecycleDefaults(t *testing.T, label string, got LoopDefaultConfig) {
	t.Helper()

	if got.Retry != (LoopRetryDefaultConfig{MaxAttempts: 3, BackoffBase: "1s", BackoffMax: "30s"}) {
		t.Fatalf("%s Retry = %#v, want shipped lifecycle defaults", label, got.Retry)
	}
	if got.Liveness.SilenceWindow != "30m" || got.Resume.DeathStreakLimit != 3 ||
		got.Predicates.CostLimit != 10000 || got.Waits.AdmissionAttempts != 3 ||
		got.Waits.AdmissionRetryInterval != "60s" || got.Admission.TombstoneHorizon != "168h" {
		t.Fatalf("%s lifecycle defaults = %#v, want shipped values", label, got)
	}
	if got.Autopause == nil || len(got.Autopause) != 0 {
		t.Fatalf("%s Autopause = %#v, want owned empty list", label, got.Autopause)
	}
}
