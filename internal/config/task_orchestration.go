package config

import (
	"time"
)

const (
	// MaxTaskOrchestrationRuntime is the largest accepted runtime watchdog budget.
	MaxTaskOrchestrationRuntime = 24 * time.Hour
	// DefaultTaskDesignatedRunMax is the default bounded sibling-run fan-out size.
	DefaultTaskDesignatedRunMax = 5
	// MaxTaskDesignatedRunMax is the largest accepted sibling-run fan-out size.
	MaxTaskDesignatedRunMax = 5
	// DefaultTaskMaxActiveRunsPerWorkspace bounds simultaneous task execution per workspace.
	DefaultTaskMaxActiveRunsPerWorkspace = 16
	// DefaultTaskActionRunTimeout bounds action-node execution when a node omits timeout.
	DefaultTaskActionRunTimeout = 30 * time.Minute
	// DefaultTaskNetworkStatusQueueSize is the default observer queue depth.
	DefaultTaskNetworkStatusQueueSize = 64
	// DefaultTaskNetworkStatusTimeout is the default per-event observer timeout.
	DefaultTaskNetworkStatusTimeout = 5 * time.Second

	TaskCoordinatorModeInherit = "inherit"
	TaskCoordinatorModeGuided  = "guided"
	TaskWorkerModeInherit      = "inherit"
	TaskSandboxModeInherit     = "inherit"
	TaskSandboxModeNone        = "none"
	TaskReviewPolicyNone       = "none"
	TaskReviewPolicyOnSuccess  = "on_success"
	TaskReviewPolicyOnFailure  = "on_failure"
	TaskReviewPolicyAlways     = "always"
	TaskReviewFailureBlockTask = "block_task"
	TaskReviewFailureFailTask  = "fail_task"
)

// TaskConfig controls task runtime behavior.
type TaskConfig struct {
	Orchestration TaskOrchestrationConfig `toml:"orchestration"`
	Recovery      TaskRecoveryConfig      `toml:"recovery"`
}

// TaskRecoveryConfig controls task-run recovery verbs.
type TaskRecoveryConfig struct {
	AllowAgentForce bool `toml:"allow_agent_force"`
}

// TaskOrchestrationConfig controls bounded task orchestration behavior.
type TaskOrchestrationConfig struct {
	SummaryMaxBytes           int                            `toml:"summary_max_bytes"`
	ContextBodyMaxBytes       int                            `toml:"context_body_max_bytes"`
	ContextPriorAttempts      int                            `toml:"context_prior_attempts"`
	ContextRecentEvents       int                            `toml:"context_recent_events"`
	SpawnFailureLimit         int                            `toml:"spawn_failure_limit"`
	SchedulerBadTickThreshold int                            `toml:"scheduler_bad_tick_threshold"`
	SchedulerBadTickCooldown  time.Duration                  `toml:"scheduler_bad_tick_cooldown"`
	DefaultMaxRuntime         time.Duration                  `toml:"default_max_runtime"`
	BridgeNotificationTimeout time.Duration                  `toml:"bridge_notification_timeout"`
	DesignatedRunMax          int                            `toml:"designated_run_max"`
	MaxActiveRunsPerWorkspace int                            `toml:"max_active_runs_per_workspace"`
	ActionRunTimeout          time.Duration                  `toml:"action_run_timeout"`
	NetworkStatusQueueSize    int                            `toml:"network_status_queue_size"`
	NetworkStatusTimeout      time.Duration                  `toml:"network_status_timeout"`
	Profile                   TaskOrchestrationProfileConfig `toml:"profile"`
	Review                    TaskOrchestrationReviewConfig  `toml:"review"`
}

// TaskOrchestrationProfileConfig controls task execution profile defaults and gates.
type TaskOrchestrationProfileConfig struct {
	DefaultCoordinatorMode    string `toml:"default_coordinator_mode"`
	DefaultWorkerMode         string `toml:"default_worker_mode"`
	DefaultSandboxMode        string `toml:"default_sandbox_mode"`
	AllowTaskProviderOverride bool   `toml:"allow_task_provider_override"`
	AllowTaskSandboxNone      bool   `toml:"allow_task_sandbox_none"`
}

// TaskOrchestrationReviewConfig controls task review gate defaults and bounds.
type TaskOrchestrationReviewConfig struct {
	DefaultPolicy             string        `toml:"default_policy"`
	MaxRounds                 int           `toml:"max_rounds"`
	MaxReviewAttempts         int           `toml:"max_review_attempts"`
	Timeout                   time.Duration `toml:"timeout"`
	RapidTerminalWindow       time.Duration `toml:"rapid_terminal_window"`
	RapidTerminalLimit        int           `toml:"rapid_terminal_limit"`
	MissingWorkMaxItems       int           `toml:"missing_work_max_items"`
	MissingWorkItemMaxBytes   int           `toml:"missing_work_item_max_bytes"`
	ReasonMaxBytes            int           `toml:"reason_max_bytes"`
	ReviewTextMaxBytes        int           `toml:"review_text_max_bytes"`
	NextRoundGuidanceMaxBytes int           `toml:"next_round_guidance_max_bytes"`
	FailurePolicy             string        `toml:"failure_policy"`
}

// DefaultTaskConfig returns built-in task runtime defaults.
func DefaultTaskConfig() TaskConfig {
	return TaskConfig{
		Orchestration: TaskOrchestrationConfig{
			SummaryMaxBytes:           4096,
			ContextBodyMaxBytes:       8192,
			ContextPriorAttempts:      5,
			ContextRecentEvents:       50,
			SpawnFailureLimit:         5,
			SchedulerBadTickThreshold: 6,
			SchedulerBadTickCooldown:  5 * time.Minute,
			DefaultMaxRuntime:         0,
			BridgeNotificationTimeout: 10 * time.Second,
			DesignatedRunMax:          DefaultTaskDesignatedRunMax,
			MaxActiveRunsPerWorkspace: DefaultTaskMaxActiveRunsPerWorkspace,
			ActionRunTimeout:          DefaultTaskActionRunTimeout,
			NetworkStatusQueueSize:    DefaultTaskNetworkStatusQueueSize,
			NetworkStatusTimeout:      DefaultTaskNetworkStatusTimeout,
			Profile: TaskOrchestrationProfileConfig{
				DefaultCoordinatorMode:    TaskCoordinatorModeInherit,
				DefaultWorkerMode:         TaskWorkerModeInherit,
				DefaultSandboxMode:        TaskSandboxModeInherit,
				AllowTaskProviderOverride: true,
				AllowTaskSandboxNone:      true,
			},
			Review: TaskOrchestrationReviewConfig{
				DefaultPolicy:             TaskReviewPolicyNone,
				MaxRounds:                 3,
				MaxReviewAttempts:         2,
				Timeout:                   20 * time.Minute,
				RapidTerminalWindow:       2 * time.Minute,
				RapidTerminalLimit:        3,
				MissingWorkMaxItems:       20,
				MissingWorkItemMaxBytes:   512,
				ReasonMaxBytes:            2048,
				ReviewTextMaxBytes:        12000,
				NextRoundGuidanceMaxBytes: 4096,
				FailurePolicy:             TaskReviewFailureBlockTask,
			},
		},
		Recovery: TaskRecoveryConfig{
			AllowAgentForce: true,
		},
	}
}
