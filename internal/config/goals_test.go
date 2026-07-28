package config

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGoalsConfigShouldLoadDefaultsAndOverlays(t *testing.T) {
	t.Parallel()

	t.Run("Should load Goal defaults", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}

		cfg := DefaultWithHome(homePaths)
		if cfg.Goals.MaxTurns != 20 {
			t.Fatalf("Goals.MaxTurns = %d, want 20", cfg.Goals.MaxTurns)
		}
		if cfg.Goals.ContextNudgeRatio != 0.8 {
			t.Fatalf("Goals.ContextNudgeRatio = %v, want 0.8", cfg.Goals.ContextNudgeRatio)
		}
		if cfg.Goals.OutboxBatchSize != 50 {
			t.Fatalf("Goals.OutboxBatchSize = %d, want 50", cfg.Goals.OutboxBatchSize)
		}
		if cfg.Goals.OutboxPollInterval != 100*time.Millisecond {
			t.Fatalf("Goals.OutboxPollInterval = %s, want 100ms", cfg.Goals.OutboxPollInterval)
		}
	})

	t.Run("Should apply global and workspace overlays with zero ratio preserved", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		workspaceRoot := t.TempDir()
		writeFile(t, homePaths.ConfigFile, `
[goals]
max_turns = 12
context_nudge_ratio = 0.6
outbox_batch_size = 40
outbox_poll_interval = "250ms"
`)
		writeFile(t, filepath.Join(workspaceRoot, DirName, ConfigName), `
[goals]
max_turns = 7
context_nudge_ratio = 0.0
outbox_batch_size = 25
outbox_poll_interval = "50ms"
`)

		cfg, err := LoadForHome(homePaths, WithWorkspaceRoot(workspaceRoot))
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		if cfg.Goals.MaxTurns != 7 {
			t.Fatalf("Goals.MaxTurns = %d, want workspace value 7", cfg.Goals.MaxTurns)
		}
		if cfg.Goals.ContextNudgeRatio != 0 {
			t.Fatalf("Goals.ContextNudgeRatio = %v, want explicit zero", cfg.Goals.ContextNudgeRatio)
		}
		if cfg.Goals.OutboxBatchSize != 25 || cfg.Goals.OutboxPollInterval != 50*time.Millisecond {
			t.Fatalf(
				"Goals outbox config = %d/%s, want 25/50ms",
				cfg.Goals.OutboxBatchSize,
				cfg.Goals.OutboxPollInterval,
			)
		}
	})
}

func TestGoalsConfigShouldRejectWriteTimeInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		path      []string
		value     any
		wantError string
	}{
		{
			name:      "Should reject zero max turns",
			path:      []string{"goals", "max_turns"},
			value:     0,
			wantError: goalMaxTurnsPath,
		},
		{
			name:      "Should reject negative context nudge ratio",
			path:      []string{"goals", "context_nudge_ratio"},
			value:     -0.1,
			wantError: goalContextNudgeRatioPath,
		},
		{
			name:      "Should reject context nudge ratio above one",
			path:      []string{"goals", "context_nudge_ratio"},
			value:     1.1,
			wantError: goalContextNudgeRatioPath,
		},
		{
			name:      "Should reject zero outbox batch size",
			path:      []string{"goals", "outbox_batch_size"},
			value:     0,
			wantError: goalOutboxBatchSizePath,
		},
		{
			name:      "Should reject outbox batch size above the store limit",
			path:      []string{"goals", "outbox_batch_size"},
			value:     201,
			wantError: goalOutboxBatchSizePath,
		},
		{
			name:      "Should reject non-positive outbox poll interval",
			path:      []string{"goals", "outbox_poll_interval"},
			value:     "0s",
			wantError: goalOutboxPollIntervalPath,
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

func TestGoalsConfigShouldExposeAgentMutableToolPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path []string
		kind ValueKind
	}{
		{
			name: "Should allow Goal max turns",
			path: []string{"goals", "max_turns"},
			kind: ConfigValueInt,
		},
		{
			name: "Should allow Goal context nudge ratio",
			path: []string{"goals", "context_nudge_ratio"},
			kind: ConfigValueFloat,
		},
		{
			name: "Should allow Goal outbox batch size",
			path: []string{"goals", "outbox_batch_size"},
			kind: ConfigValueInt,
		},
		{
			name: "Should allow Goal outbox poll interval",
			path: []string{"goals", "outbox_poll_interval"},
			kind: ConfigValueDuration,
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
