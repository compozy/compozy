package config

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTaskOrchestrationConfigDefaultsAndValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should include built in task orchestration defaults", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}

		cfg := DefaultWithHome(homePaths)
		orchestration := cfg.Task.Orchestration
		if got, want := orchestration.SummaryMaxBytes, 4096; got != want {
			t.Fatalf("DefaultWithHome() Task.Orchestration.SummaryMaxBytes = %d, want %d", got, want)
		}
		if got, want := orchestration.ContextBodyMaxBytes, 8192; got != want {
			t.Fatalf("DefaultWithHome() Task.Orchestration.ContextBodyMaxBytes = %d, want %d", got, want)
		}
		if got, want := orchestration.ContextPriorAttempts, 5; got != want {
			t.Fatalf("DefaultWithHome() Task.Orchestration.ContextPriorAttempts = %d, want %d", got, want)
		}
		if got, want := orchestration.ContextRecentEvents, 50; got != want {
			t.Fatalf("DefaultWithHome() Task.Orchestration.ContextRecentEvents = %d, want %d", got, want)
		}
		if got, want := orchestration.SpawnFailureLimit, 5; got != want {
			t.Fatalf("DefaultWithHome() Task.Orchestration.SpawnFailureLimit = %d, want %d", got, want)
		}
		if got, want := orchestration.SchedulerBadTickThreshold, 6; got != want {
			t.Fatalf("DefaultWithHome() Task.Orchestration.SchedulerBadTickThreshold = %d, want %d", got, want)
		}
		if got, want := orchestration.SchedulerBadTickCooldown, 5*time.Minute; got != want {
			t.Fatalf("DefaultWithHome() Task.Orchestration.SchedulerBadTickCooldown = %s, want %s", got, want)
		}
		if got := orchestration.DefaultMaxRuntime; got != 0 {
			t.Fatalf("DefaultWithHome() Task.Orchestration.DefaultMaxRuntime = %s, want disabled", got)
		}
		if got, want := orchestration.BridgeNotificationTimeout, 10*time.Second; got != want {
			t.Fatalf("DefaultWithHome() Task.Orchestration.BridgeNotificationTimeout = %s, want %s", got, want)
		}
		if got, want := orchestration.DesignatedRunMax, DefaultTaskDesignatedRunMax; got != want {
			t.Fatalf("DefaultWithHome() Task.Orchestration.DesignatedRunMax = %d, want %d", got, want)
		}
		if got, want := orchestration.MaxActiveRunsPerWorkspace, DefaultTaskMaxActiveRunsPerWorkspace; got != want {
			t.Fatalf("DefaultWithHome() Task.Orchestration.MaxActiveRunsPerWorkspace = %d, want %d", got, want)
		}
		if got, want := orchestration.ActionRunTimeout, DefaultTaskActionRunTimeout; got != want {
			t.Fatalf("DefaultWithHome() Task.Orchestration.ActionRunTimeout = %s, want %s", got, want)
		}
		if got, want := orchestration.NetworkStatusQueueSize, DefaultTaskNetworkStatusQueueSize; got != want {
			t.Fatalf("DefaultWithHome() Task.Orchestration.NetworkStatusQueueSize = %d, want %d", got, want)
		}
		if got, want := orchestration.NetworkStatusTimeout, DefaultTaskNetworkStatusTimeout; got != want {
			t.Fatalf("DefaultWithHome() Task.Orchestration.NetworkStatusTimeout = %s, want %s", got, want)
		}
		if got, want := orchestration.Profile.DefaultCoordinatorMode, TaskCoordinatorModeInherit; got != want {
			t.Fatalf("DefaultWithHome() profile DefaultCoordinatorMode = %q, want %q", got, want)
		}
		if got, want := orchestration.Profile.DefaultWorkerMode, TaskWorkerModeInherit; got != want {
			t.Fatalf("DefaultWithHome() profile DefaultWorkerMode = %q, want %q", got, want)
		}
		if got, want := orchestration.Profile.DefaultSandboxMode, TaskSandboxModeInherit; got != want {
			t.Fatalf("DefaultWithHome() profile DefaultSandboxMode = %q, want %q", got, want)
		}
		if !orchestration.Profile.AllowTaskProviderOverride {
			t.Fatal("DefaultWithHome() profile AllowTaskProviderOverride = false, want true")
		}
		if !orchestration.Profile.AllowTaskSandboxNone {
			t.Fatal("DefaultWithHome() profile AllowTaskSandboxNone = false, want true")
		}
		if !cfg.Task.Recovery.AllowAgentForce {
			t.Fatal("DefaultWithHome() recovery AllowAgentForce = false, want true")
		}
		if got, want := orchestration.Review.DefaultPolicy, TaskReviewPolicyNone; got != want {
			t.Fatalf("DefaultWithHome() review DefaultPolicy = %q, want %q", got, want)
		}
		if got, want := orchestration.Review.MaxRounds, 3; got != want {
			t.Fatalf("DefaultWithHome() review MaxRounds = %d, want %d", got, want)
		}
		if got, want := orchestration.Review.MaxReviewAttempts, 2; got != want {
			t.Fatalf("DefaultWithHome() review MaxReviewAttempts = %d, want %d", got, want)
		}
		if got, want := orchestration.Review.Timeout, 20*time.Minute; got != want {
			t.Fatalf("DefaultWithHome() review Timeout = %s, want %s", got, want)
		}
		if got, want := orchestration.Review.RapidTerminalWindow, 2*time.Minute; got != want {
			t.Fatalf("DefaultWithHome() review RapidTerminalWindow = %s, want %s", got, want)
		}
		if got, want := orchestration.Review.RapidTerminalLimit, 3; got != want {
			t.Fatalf("DefaultWithHome() review RapidTerminalLimit = %d, want %d", got, want)
		}
		if got, want := orchestration.Review.MissingWorkMaxItems, 20; got != want {
			t.Fatalf("DefaultWithHome() review MissingWorkMaxItems = %d, want %d", got, want)
		}
		if got, want := orchestration.Review.MissingWorkItemMaxBytes, 512; got != want {
			t.Fatalf("DefaultWithHome() review MissingWorkItemMaxBytes = %d, want %d", got, want)
		}
		if got, want := orchestration.Review.ReasonMaxBytes, 2048; got != want {
			t.Fatalf("DefaultWithHome() review ReasonMaxBytes = %d, want %d", got, want)
		}
		if got, want := orchestration.Review.ReviewTextMaxBytes, 12000; got != want {
			t.Fatalf("DefaultWithHome() review ReviewTextMaxBytes = %d, want %d", got, want)
		}
		if got, want := orchestration.Review.NextRoundGuidanceMaxBytes, 4096; got != want {
			t.Fatalf("DefaultWithHome() review NextRoundGuidanceMaxBytes = %d, want %d", got, want)
		}
		if got, want := orchestration.Review.FailurePolicy, TaskReviewFailureBlockTask; got != want {
			t.Fatalf("DefaultWithHome() review FailurePolicy = %q, want %q", got, want)
		}
	})

	base := DefaultTaskConfig()
	tests := []struct {
		name    string
		mutate  func(*TaskConfig)
		wantErr string
	}{
		{
			name:    "Should reject zero summary limit",
			mutate:  func(cfg *TaskConfig) { cfg.Orchestration.SummaryMaxBytes = 0 },
			wantErr: "task.orchestration.summary_max_bytes",
		},
		{
			name:    "Should reject zero context body limit",
			mutate:  func(cfg *TaskConfig) { cfg.Orchestration.ContextBodyMaxBytes = 0 },
			wantErr: "task.orchestration.context_body_max_bytes",
		},
		{
			name:    "Should reject negative prior attempts",
			mutate:  func(cfg *TaskConfig) { cfg.Orchestration.ContextPriorAttempts = -1 },
			wantErr: "task.orchestration.context_prior_attempts",
		},
		{
			name:    "Should reject negative recent events",
			mutate:  func(cfg *TaskConfig) { cfg.Orchestration.ContextRecentEvents = -1 },
			wantErr: "task.orchestration.context_recent_events",
		},
		{
			name:    "Should reject zero spawn failure limit",
			mutate:  func(cfg *TaskConfig) { cfg.Orchestration.SpawnFailureLimit = 0 },
			wantErr: "task.orchestration.spawn_failure_limit",
		},
		{
			name:    "Should reject zero scheduler bad tick threshold",
			mutate:  func(cfg *TaskConfig) { cfg.Orchestration.SchedulerBadTickThreshold = 0 },
			wantErr: "task.orchestration.scheduler_bad_tick_threshold",
		},
		{
			name:    "Should reject zero scheduler cooldown",
			mutate:  func(cfg *TaskConfig) { cfg.Orchestration.SchedulerBadTickCooldown = 0 },
			wantErr: "task.orchestration.scheduler_bad_tick_cooldown",
		},
		{
			name:    "Should reject fractional default runtime",
			mutate:  func(cfg *TaskConfig) { cfg.Orchestration.DefaultMaxRuntime = 1500 * time.Millisecond },
			wantErr: "task.orchestration.default_max_runtime",
		},
		{
			name:    "Should reject runtime above watchdog maximum",
			mutate:  func(cfg *TaskConfig) { cfg.Orchestration.DefaultMaxRuntime = 25 * time.Hour },
			wantErr: "task.orchestration.default_max_runtime",
		},
		{
			name:    "Should reject zero bridge notification timeout",
			mutate:  func(cfg *TaskConfig) { cfg.Orchestration.BridgeNotificationTimeout = 0 },
			wantErr: "task.orchestration.bridge_notification_timeout",
		},
		{
			name:    "Should reject designated run max above the configured cap",
			mutate:  func(cfg *TaskConfig) { cfg.Orchestration.DesignatedRunMax = MaxTaskDesignatedRunMax + 1 },
			wantErr: "task.orchestration.designated_run_max",
		},
		{
			name:    "Should reject negative workspace active run cap",
			mutate:  func(cfg *TaskConfig) { cfg.Orchestration.MaxActiveRunsPerWorkspace = -1 },
			wantErr: "task.orchestration.max_active_runs_per_workspace",
		},
		{
			name:    "Should reject zero action run timeout",
			mutate:  func(cfg *TaskConfig) { cfg.Orchestration.ActionRunTimeout = 0 },
			wantErr: "task.orchestration.action_run_timeout",
		},
		{
			name:    "Should reject fractional action run timeout",
			mutate:  func(cfg *TaskConfig) { cfg.Orchestration.ActionRunTimeout = 1500 * time.Millisecond },
			wantErr: "task.orchestration.action_run_timeout",
		},
		{
			name:    "Should reject action run timeout above watchdog maximum",
			mutate:  func(cfg *TaskConfig) { cfg.Orchestration.ActionRunTimeout = 25 * time.Hour },
			wantErr: "task.orchestration.action_run_timeout",
		},
		{
			name:    "Should reject zero network status queue size",
			mutate:  func(cfg *TaskConfig) { cfg.Orchestration.NetworkStatusQueueSize = 0 },
			wantErr: "task.orchestration.network_status_queue_size",
		},
		{
			name:    "Should reject fractional network status timeout",
			mutate:  func(cfg *TaskConfig) { cfg.Orchestration.NetworkStatusTimeout = 1500 * time.Millisecond },
			wantErr: "task.orchestration.network_status_timeout",
		},
		{
			name: "Should reject unknown coordinator default mode",
			mutate: func(cfg *TaskConfig) {
				cfg.Orchestration.Profile.DefaultCoordinatorMode = "dedicated"
			},
			wantErr: "task.orchestration.profile.default_coordinator_mode",
		},
		{
			name: "Should reject unknown worker default mode",
			mutate: func(cfg *TaskConfig) {
				cfg.Orchestration.Profile.DefaultWorkerMode = "custom"
			},
			wantErr: "task.orchestration.profile.default_worker_mode",
		},
		{
			name: "Should reject sandbox none default when gate is disabled",
			mutate: func(cfg *TaskConfig) {
				cfg.Orchestration.Profile.DefaultSandboxMode = TaskSandboxModeNone
				cfg.Orchestration.Profile.AllowTaskSandboxNone = false
			},
			wantErr: "task.orchestration.profile.default_sandbox_mode",
		},
		{
			name: "Should reject unknown review policy",
			mutate: func(cfg *TaskConfig) {
				cfg.Orchestration.Review.DefaultPolicy = "manual"
			},
			wantErr: "task.orchestration.review.default_policy",
		},
		{
			name: "Should reject zero review rounds",
			mutate: func(cfg *TaskConfig) {
				cfg.Orchestration.Review.MaxRounds = 0
			},
			wantErr: "task.orchestration.review.max_rounds",
		},
		{
			name: "Should reject zero review attempts",
			mutate: func(cfg *TaskConfig) {
				cfg.Orchestration.Review.MaxReviewAttempts = 0
			},
			wantErr: "task.orchestration.review.max_review_attempts",
		},
		{
			name: "Should reject zero review timeout",
			mutate: func(cfg *TaskConfig) {
				cfg.Orchestration.Review.Timeout = 0
			},
			wantErr: "task.orchestration.review.timeout",
		},
		{
			name: "Should reject fractional rapid terminal window",
			mutate: func(cfg *TaskConfig) {
				cfg.Orchestration.Review.RapidTerminalWindow = 1500 * time.Millisecond
			},
			wantErr: "task.orchestration.review.rapid_terminal_window",
		},
		{
			name: "Should reject zero rapid terminal limit",
			mutate: func(cfg *TaskConfig) {
				cfg.Orchestration.Review.RapidTerminalLimit = 0
			},
			wantErr: "task.orchestration.review.rapid_terminal_limit",
		},
		{
			name: "Should reject zero missing work item budget",
			mutate: func(cfg *TaskConfig) {
				cfg.Orchestration.Review.MissingWorkMaxItems = 0
			},
			wantErr: "task.orchestration.review.missing_work_max_items",
		},
		{
			name: "Should reject zero missing work item bytes",
			mutate: func(cfg *TaskConfig) {
				cfg.Orchestration.Review.MissingWorkItemMaxBytes = 0
			},
			wantErr: "task.orchestration.review.missing_work_item_max_bytes",
		},
		{
			name: "Should reject zero reason bytes",
			mutate: func(cfg *TaskConfig) {
				cfg.Orchestration.Review.ReasonMaxBytes = 0
			},
			wantErr: "task.orchestration.review.reason_max_bytes",
		},
		{
			name: "Should reject zero review text bytes",
			mutate: func(cfg *TaskConfig) {
				cfg.Orchestration.Review.ReviewTextMaxBytes = 0
			},
			wantErr: "task.orchestration.review.review_text_max_bytes",
		},
		{
			name: "Should reject zero next round guidance bytes",
			mutate: func(cfg *TaskConfig) {
				cfg.Orchestration.Review.NextRoundGuidanceMaxBytes = 0
			},
			wantErr: "task.orchestration.review.next_round_guidance_max_bytes",
		},
		{
			name: "Should reject unknown review failure policy",
			mutate: func(cfg *TaskConfig) {
				cfg.Orchestration.Review.FailurePolicy = "ignore"
			},
			wantErr: "task.orchestration.review.failure_policy",
		},
		{
			name: "Should accept guided coordinator and disabled default runtime",
			mutate: func(cfg *TaskConfig) {
				cfg.Orchestration.Profile.DefaultCoordinatorMode = TaskCoordinatorModeGuided
				cfg.Orchestration.Profile.DefaultSandboxMode = TaskSandboxModeNone
				cfg.Orchestration.Review.DefaultPolicy = TaskReviewPolicyAlways
				cfg.Orchestration.Review.FailurePolicy = TaskReviewFailureFailTask
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := base
			tt.mutate(&cfg)
			err := cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("Validate() error = nil, want non-nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadTaskOrchestrationConfig(t *testing.T) {
	t.Parallel()

	t.Run("Should merge global and workspace task orchestration overlays", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := t.TempDir()
		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}

		writeFile(t, homePaths.ConfigFile, `
[task.orchestration]
summary_max_bytes = 1000
context_body_max_bytes = 2000
context_prior_attempts = 3
context_recent_events = 30
spawn_failure_limit = 4
scheduler_bad_tick_threshold = 5
scheduler_bad_tick_cooldown = "4m"
default_max_runtime = "1h"
max_active_runs_per_workspace = 24
action_run_timeout = "20m"
network_status_queue_size = 17
network_status_timeout = "7s"

[task.recovery]
allow_agent_force = false

[task.orchestration.profile]
default_coordinator_mode = "guided"
default_worker_mode = "inherit"
default_sandbox_mode = "none"
allow_task_provider_override = false
allow_task_sandbox_none = true

[task.orchestration.review]
default_policy = "always"
max_rounds = 5
max_review_attempts = 4
timeout = "30m"
rapid_terminal_window = "3m"
rapid_terminal_limit = 4
missing_work_max_items = 10
missing_work_item_max_bytes = 256
reason_max_bytes = 1024
review_text_max_bytes = 6000
next_round_guidance_max_bytes = 2048
failure_policy = "fail_task"
`)
		writeFile(t, filepath.Join(workspaceRoot, DirName, ConfigName), `
[task.orchestration]
summary_max_bytes = 3000
default_max_runtime = "0s"
max_active_runs_per_workspace = 0
action_run_timeout = "45m"
network_status_timeout = "3s"

[task.recovery]
allow_agent_force = true

[task.orchestration.review]
default_policy = "on_success"
timeout = "10m"
`)

		cfg, err := LoadForHome(homePaths, WithWorkspaceRoot(workspaceRoot))
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}

		orchestration := cfg.Task.Orchestration
		if got, want := orchestration.SummaryMaxBytes, 3000; got != want {
			t.Fatalf("LoadForHome() SummaryMaxBytes = %d, want workspace override %d", got, want)
		}
		if got, want := orchestration.ContextBodyMaxBytes, 2000; got != want {
			t.Fatalf("LoadForHome() ContextBodyMaxBytes = %d, want global value %d", got, want)
		}
		if got := orchestration.DefaultMaxRuntime; got != 0 {
			t.Fatalf("LoadForHome() DefaultMaxRuntime = %s, want workspace disabled runtime", got)
		}
		if got := orchestration.MaxActiveRunsPerWorkspace; got != 0 {
			t.Fatalf("LoadForHome() MaxActiveRunsPerWorkspace = %d, want workspace-disabled cap", got)
		}
		if got, want := orchestration.ActionRunTimeout, 45*time.Minute; got != want {
			t.Fatalf("LoadForHome() ActionRunTimeout = %s, want workspace override %s", got, want)
		}
		if got, want := orchestration.NetworkStatusQueueSize, 17; got != want {
			t.Fatalf("LoadForHome() NetworkStatusQueueSize = %d, want global value %d", got, want)
		}
		if got, want := orchestration.NetworkStatusTimeout, 3*time.Second; got != want {
			t.Fatalf("LoadForHome() NetworkStatusTimeout = %s, want workspace override %s", got, want)
		}
		if got, want := orchestration.Profile.DefaultCoordinatorMode, TaskCoordinatorModeGuided; got != want {
			t.Fatalf("LoadForHome() DefaultCoordinatorMode = %q, want %q", got, want)
		}
		if got, want := orchestration.Profile.DefaultSandboxMode, TaskSandboxModeNone; got != want {
			t.Fatalf("LoadForHome() DefaultSandboxMode = %q, want %q", got, want)
		}
		if orchestration.Profile.AllowTaskProviderOverride {
			t.Fatal("LoadForHome() AllowTaskProviderOverride = true, want global false")
		}
		if got, want := orchestration.Review.DefaultPolicy, TaskReviewPolicyOnSuccess; got != want {
			t.Fatalf("LoadForHome() Review.DefaultPolicy = %q, want workspace override %q", got, want)
		}
		if got, want := orchestration.Review.Timeout, 10*time.Minute; got != want {
			t.Fatalf("LoadForHome() Review.Timeout = %s, want workspace override %s", got, want)
		}
		if got, want := orchestration.Review.MaxRounds, 5; got != want {
			t.Fatalf("LoadForHome() Review.MaxRounds = %d, want global value %d", got, want)
		}
		if got, want := orchestration.Review.FailurePolicy, TaskReviewFailureFailTask; got != want {
			t.Fatalf("LoadForHome() Review.FailurePolicy = %q, want global value %q", got, want)
		}
		if !cfg.Task.Recovery.AllowAgentForce {
			t.Fatal("LoadForHome() Recovery.AllowAgentForce = false, want workspace override true")
		}
	})

	t.Run("Should reject unknown task orchestration keys", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := t.TempDir()
		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		writeFile(t, homePaths.ConfigFile, `
[task.orchestration]
unknown = true
`)

		_, err = LoadForHome(homePaths, WithWorkspaceRoot(workspaceRoot))
		if err == nil {
			t.Fatal("LoadForHome() error = nil, want unknown-key failure")
		}
		if !strings.Contains(err.Error(), "task.orchestration.unknown") {
			t.Fatalf("LoadForHome() error = %v, want task.orchestration.unknown", err)
		}
	})

	t.Run("Should reject unknown task recovery keys", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := t.TempDir()
		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		writeFile(t, homePaths.ConfigFile, `
[task.recovery]
unknown = true
`)

		_, err = LoadForHome(homePaths, WithWorkspaceRoot(workspaceRoot))
		if err == nil {
			t.Fatal("LoadForHome() error = nil, want unknown-key failure")
		}
		if !strings.Contains(err.Error(), "task.recovery.unknown") {
			t.Fatalf("LoadForHome() error = %v, want task.recovery.unknown", err)
		}
	})
}
