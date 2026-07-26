package config

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/loop/dsl"
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

[loops.defaults.delivery.model_defaults]
worker = "global-worker"
judge = "global-judge"

[loops.defaults.delivery.budget]
tokens = 100
wall_clock_sec = 60
on_exceeded = "escalate"

[loops.defaults.watch]
iteration_cap = 1
fan_out_width = 1

[loops.defaults.watch.no_progress]
window = 1

[loops.defaults.watch.model_defaults]
judge = "global-watch-judge"
`)
		writeFile(t, filepath.Join(workspaceRoot, DirName, ConfigName), `
[loops.defaults.delivery]
iteration_cap = 0
fan_out_width = 2

[loops.defaults.delivery.budget]
tokens = 0
on_exceeded = "halt"

[loops.defaults.delivery.model_defaults]
worker = "workspace-worker"

[loops.defaults.watch]
fan_out_width = 5

[loops.defaults.watch.no_progress]
window = 4

[loops.defaults.watch.gates]
max_revisions = 7

[loops.defaults.watch.model_defaults]
judge = "workspace-watch-judge"
`)

		cfg, err := LoadForHome(homePaths, WithWorkspaceRoot(workspaceRoot))
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}

		assertLoopDefaultConfig(t, "delivery", cfg.Loops.Defaults.Delivery, loopDefaultWant{
			iterationCap:     0,
			noProgressWindow: 2,
			gateMaxRevisions: 9,
			budgetTokens:     0,
			budgetWallSec:    60,
			budgetOnExceeded: string(dsl.BudgetExceededHalt),
			fanOutWidth:      2,
			workerModel:      "workspace-worker",
			judgeModel:       "global-judge",
		})
		assertLoopDefaultConfig(t, "watch", cfg.Loops.Defaults.Watch, loopDefaultWant{
			iterationCap:     1,
			noProgressWindow: 4,
			gateMaxRevisions: 7,
			budgetTokens:     0,
			budgetWallSec:    0,
			budgetOnExceeded: string(dsl.BudgetExceededHalt),
			fanOutWidth:      5,
			judgeModel:       "workspace-watch-judge",
		})
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
			name:      "Should reject delivery fan out above compile-time ceiling",
			path:      []string{"loops", "defaults", "delivery", "fan_out_width"},
			value:     loopDefaultsMaxFanoutWidth + 1,
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
			name: "Should allow delivery worker model defaults",
			path: []string{"loops", "defaults", "delivery", "model_defaults", "worker"},
			kind: ConfigValueString,
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
	iterationCap     int
	noProgressWindow int
	gateMaxRevisions int
	budgetTokens     int
	budgetWallSec    int
	budgetOnExceeded string
	fanOutWidth      int
	workerModel      string
	judgeModel       string
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
	if got.ModelDefaults.Worker != want.workerModel {
		t.Fatalf("%s ModelDefaults.Worker = %q, want %q", label, got.ModelDefaults.Worker, want.workerModel)
	}
	if got.ModelDefaults.Judge != want.judgeModel {
		t.Fatalf("%s ModelDefaults.Judge = %q, want %q", label, got.ModelDefaults.Judge, want.judgeModel)
	}
}
