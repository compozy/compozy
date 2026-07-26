package lifecycle

import "testing"

func TestClassifyPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		path          string
		wantLifecycle Lifecycle
		wantDiffClass DiffClass
		wantErr       bool
	}{
		{
			name:          "Should classify disabled skills as live",
			path:          "skills.disabled_skills",
			wantLifecycle: Live,
			wantDiffClass: DiffClassLive,
		},
		{
			name:          "Should classify provider model changes as live",
			path:          "providers.codex.models",
			wantLifecycle: Live,
			wantDiffClass: DiffClassLive,
		},
		{
			name:          "Should classify provider model descendants as live",
			path:          "providers.codex.models.curated",
			wantLifecycle: Live,
			wantDiffClass: DiffClassLive,
		},
		{
			name:          "Should classify provider descendants as restart required",
			path:          "providers.codex.command",
			wantLifecycle: RestartRequired,
			wantDiffClass: DiffClassRestartRequired,
		},
		{
			name:          "Should classify sandbox descendants as session rebind",
			path:          "sandboxes.daytona-dev.backend",
			wantLifecycle: SessionRebind,
			wantDiffClass: DiffClassSessionRebind,
		},
		{
			name:          "Should classify reload timeout as live",
			path:          "daemon.reload_timeouts.providers",
			wantLifecycle: Live,
			wantDiffClass: DiffClassLive,
		},
		{
			name:          "Should classify daemon memory reports as restart required",
			path:          "daemon.memory_report_interval",
			wantLifecycle: RestartRequired,
			wantDiffClass: DiffClassRestartRequired,
		},
		{
			name:          "Should classify subprocess health escalation as restart required",
			path:          "daemon.subprocess_health_escalation_threshold",
			wantLifecycle: RestartRequired,
			wantDiffClass: DiffClassRestartRequired,
		},
		{
			name:          "Should classify clarification timeout as restart required",
			path:          "tools.clarify.timeout",
			wantLifecycle: RestartRequired,
			wantDiffClass: DiffClassRestartRequired,
		},
		{
			name:          "Should classify tool artifact retention as restart required",
			path:          "tools.artifacts.max_age",
			wantLifecycle: RestartRequired,
			wantDiffClass: DiffClassRestartRequired,
		},
		{
			name:          "Should classify marketplace catalog changes as live",
			path:          "marketplace.catalog.timeout",
			wantLifecycle: Live,
			wantDiffClass: DiffClassLive,
		},
		{
			name:          "Should classify extension side-load policy as live",
			path:          "extensions.marketplace.allow_unverified",
			wantLifecycle: Live,
			wantDiffClass: DiffClassLive,
		},
		{
			name:          "Should classify the complete roles section as live",
			path:          "roles",
			wantLifecycle: Live,
			wantDiffClass: DiffClassLive,
		},
		{
			name:          "Should classify role changes as live",
			path:          "roles.dream.model",
			wantLifecycle: Live,
			wantDiffClass: DiffClassLive,
		},
		{
			name:          "Should keep extension registry changes restart required",
			path:          "extensions.marketplace.registry",
			wantLifecycle: RestartRequired,
			wantDiffClass: DiffClassRestartRequired,
		},
		{
			name:          "Should classify redaction changes as restart required",
			path:          "redact.enabled",
			wantLifecycle: RestartRequired,
			wantDiffClass: DiffClassRestartRequired,
		},
		{
			name:          "Should classify Goal config as restart required",
			path:          "goals.context_nudge_ratio",
			wantLifecycle: RestartRequired,
			wantDiffClass: DiffClassRestartRequired,
		},
		{
			name:          "Should classify task orchestration config as restart required",
			path:          "task.orchestration.max_active_runs_per_workspace",
			wantLifecycle: RestartRequired,
			wantDiffClass: DiffClassRestartRequired,
		},
		{
			name:          "Should classify network availability as live",
			path:          "network.enabled",
			wantLifecycle: Live,
			wantDiffClass: DiffClassLive,
		},
		{
			name:          "Should classify network Live bounds as restart required",
			path:          "network.live.defaults.max_wakes",
			wantLifecycle: RestartRequired,
			wantDiffClass: DiffClassRestartRequired,
		},
		{
			name:          "Should classify window-manager behavior as live",
			path:          "window_manager.snap.repeat_ratios",
			wantLifecycle: Live,
			wantDiffClass: DiffClassLive,
		},
		{
			name:    "Should reject unknown paths",
			path:    "unknown.path",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ClassifyPath(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatal("ClassifyPath() error = nil, want non-nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ClassifyPath() error = %v", err)
			}
			if got.Lifecycle != tt.wantLifecycle {
				t.Fatalf("ClassifyPath().Lifecycle = %q, want %q", got.Lifecycle, tt.wantLifecycle)
			}
			if got.DiffClass != tt.wantDiffClass {
				t.Fatalf("ClassifyPath().DiffClass = %q, want %q", got.DiffClass, tt.wantDiffClass)
			}
		})
	}
}

func TestClassifyPaths(t *testing.T) {
	t.Parallel()

	t.Run("Should default empty mutations to live", func(t *testing.T) {
		t.Parallel()

		gotLifecycle, gotDiffClass, err := ClassifyPaths(nil)
		if err != nil {
			t.Fatalf("ClassifyPaths() error = %v", err)
		}
		if gotLifecycle != Live || gotDiffClass != DiffClassLive {
			t.Fatalf(
				"ClassifyPaths() = (%q, %q), want (%q, %q)",
				gotLifecycle,
				gotDiffClass,
				Live,
				DiffClassLive,
			)
		}
	})

	t.Run("Should let restart required dominate mixed changes", func(t *testing.T) {
		t.Parallel()

		gotLifecycle, gotDiffClass, err := ClassifyPaths([]string{
			"providers.codex.models",
			"providers.codex.command",
		})
		if err != nil {
			t.Fatalf("ClassifyPaths() error = %v", err)
		}
		if gotLifecycle != RestartRequired || gotDiffClass != DiffClassRestartRequired {
			t.Fatalf(
				"ClassifyPaths() = (%q, %q), want (%q, %q)",
				gotLifecycle,
				gotDiffClass,
				RestartRequired,
				DiffClassRestartRequired,
			)
		}
	})
}

func TestNextActionForLifecycle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		lifecycle Lifecycle
		status    Status
		want      NextAction
	}{
		{
			name:      "Should retry failed applies",
			lifecycle: Live,
			status:    StatusFailed,
			want:      NextActionRetry,
		},
		{
			name:      "Should request daemon restart for blocked restart required applies",
			lifecycle: RestartRequired,
			status:    StatusBlocked,
			want:      NextActionRestartDaemon,
		},
		{
			name:      "Should request new sessions for applied session rebind changes",
			lifecycle: SessionRebind,
			status:    StatusApplied,
			want:      NextActionNewSession,
		},
		{
			name:      "Should return none for live applied changes",
			lifecycle: Live,
			status:    StatusApplied,
			want:      NextActionNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := NextActionForLifecycle(tt.lifecycle, tt.status); got != tt.want {
				t.Fatalf("NextActionForLifecycle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDiffClassForRoot(t *testing.T) {
	t.Parallel()

	t.Run("Should classify the window-manager settings section as live", func(t *testing.T) {
		t.Parallel()

		if got, want := DiffClassForRoot("window-manager"), DiffClassLive; got != want {
			t.Fatalf("DiffClassForRoot(window-manager) = %q, want %q", got, want)
		}
	})
}
