package config

import "time"

type taskOverlay struct {
	Orchestration taskOrchestrationOverlay `toml:"orchestration"`
	Recovery      taskRecoveryOverlay      `toml:"recovery"`
}

type taskRecoveryOverlay struct {
	AllowAgentForce *bool `toml:"allow_agent_force"`
}

type taskOrchestrationOverlay struct {
	SummaryMaxBytes           *int                            `toml:"summary_max_bytes"`
	ContextBodyMaxBytes       *int                            `toml:"context_body_max_bytes"`
	ContextPriorAttempts      *int                            `toml:"context_prior_attempts"`
	ContextRecentEvents       *int                            `toml:"context_recent_events"`
	SpawnFailureLimit         *int                            `toml:"spawn_failure_limit"`
	SchedulerBadTickThreshold *int                            `toml:"scheduler_bad_tick_threshold"`
	SchedulerBadTickCooldown  *time.Duration                  `toml:"scheduler_bad_tick_cooldown"`
	DefaultMaxRuntime         *time.Duration                  `toml:"default_max_runtime"`
	BridgeNotificationTimeout *time.Duration                  `toml:"bridge_notification_timeout"`
	DesignatedRunMax          *int                            `toml:"designated_run_max"`
	MaxActiveRunsPerWorkspace *int                            `toml:"max_active_runs_per_workspace"`
	ActionRunTimeout          *time.Duration                  `toml:"action_run_timeout"`
	NetworkStatusQueueSize    *int                            `toml:"network_status_queue_size"`
	NetworkStatusTimeout      *time.Duration                  `toml:"network_status_timeout"`
	Profile                   taskOrchestrationProfileOverlay `toml:"profile"`
	Review                    taskOrchestrationReviewOverlay  `toml:"review"`
}

type taskOrchestrationProfileOverlay struct {
	DefaultCoordinatorMode    *string `toml:"default_coordinator_mode"`
	DefaultWorkerMode         *string `toml:"default_worker_mode"`
	DefaultSandboxMode        *string `toml:"default_sandbox_mode"`
	AllowTaskProviderOverride *bool   `toml:"allow_task_provider_override"`
	AllowTaskSandboxNone      *bool   `toml:"allow_task_sandbox_none"`
}

type taskOrchestrationReviewOverlay struct {
	DefaultPolicy             *string        `toml:"default_policy"`
	MaxRounds                 *int           `toml:"max_rounds"`
	MaxReviewAttempts         *int           `toml:"max_review_attempts"`
	Timeout                   *time.Duration `toml:"timeout"`
	RapidTerminalWindow       *time.Duration `toml:"rapid_terminal_window"`
	RapidTerminalLimit        *int           `toml:"rapid_terminal_limit"`
	MissingWorkMaxItems       *int           `toml:"missing_work_max_items"`
	MissingWorkItemMaxBytes   *int           `toml:"missing_work_item_max_bytes"`
	ReasonMaxBytes            *int           `toml:"reason_max_bytes"`
	ReviewTextMaxBytes        *int           `toml:"review_text_max_bytes"`
	NextRoundGuidanceMaxBytes *int           `toml:"next_round_guidance_max_bytes"`
	FailurePolicy             *string        `toml:"failure_policy"`
}

func (o taskOverlay) Apply(dst *TaskConfig) {
	o.Orchestration.Apply(&dst.Orchestration)
	o.Recovery.Apply(&dst.Recovery)
}

func (o taskRecoveryOverlay) Apply(dst *TaskRecoveryConfig) {
	if o.AllowAgentForce != nil {
		dst.AllowAgentForce = *o.AllowAgentForce
	}
}

func (o taskOrchestrationOverlay) Apply(dst *TaskOrchestrationConfig) {
	if o.SummaryMaxBytes != nil {
		dst.SummaryMaxBytes = *o.SummaryMaxBytes
	}
	if o.ContextBodyMaxBytes != nil {
		dst.ContextBodyMaxBytes = *o.ContextBodyMaxBytes
	}
	if o.ContextPriorAttempts != nil {
		dst.ContextPriorAttempts = *o.ContextPriorAttempts
	}
	if o.ContextRecentEvents != nil {
		dst.ContextRecentEvents = *o.ContextRecentEvents
	}
	if o.SpawnFailureLimit != nil {
		dst.SpawnFailureLimit = *o.SpawnFailureLimit
	}
	if o.SchedulerBadTickThreshold != nil {
		dst.SchedulerBadTickThreshold = *o.SchedulerBadTickThreshold
	}
	if o.SchedulerBadTickCooldown != nil {
		dst.SchedulerBadTickCooldown = *o.SchedulerBadTickCooldown
	}
	if o.DefaultMaxRuntime != nil {
		dst.DefaultMaxRuntime = *o.DefaultMaxRuntime
	}
	if o.BridgeNotificationTimeout != nil {
		dst.BridgeNotificationTimeout = *o.BridgeNotificationTimeout
	}
	if o.DesignatedRunMax != nil {
		dst.DesignatedRunMax = *o.DesignatedRunMax
	}
	if o.MaxActiveRunsPerWorkspace != nil {
		dst.MaxActiveRunsPerWorkspace = *o.MaxActiveRunsPerWorkspace
	}
	if o.ActionRunTimeout != nil {
		dst.ActionRunTimeout = *o.ActionRunTimeout
	}
	if o.NetworkStatusQueueSize != nil {
		dst.NetworkStatusQueueSize = *o.NetworkStatusQueueSize
	}
	if o.NetworkStatusTimeout != nil {
		dst.NetworkStatusTimeout = *o.NetworkStatusTimeout
	}
	o.Profile.Apply(&dst.Profile)
	o.Review.Apply(&dst.Review)
}

func (o taskOrchestrationProfileOverlay) Apply(dst *TaskOrchestrationProfileConfig) {
	if o.DefaultCoordinatorMode != nil {
		dst.DefaultCoordinatorMode = *o.DefaultCoordinatorMode
	}
	if o.DefaultWorkerMode != nil {
		dst.DefaultWorkerMode = *o.DefaultWorkerMode
	}
	if o.DefaultSandboxMode != nil {
		dst.DefaultSandboxMode = *o.DefaultSandboxMode
	}
	if o.AllowTaskProviderOverride != nil {
		dst.AllowTaskProviderOverride = *o.AllowTaskProviderOverride
	}
	if o.AllowTaskSandboxNone != nil {
		dst.AllowTaskSandboxNone = *o.AllowTaskSandboxNone
	}
}

func (o taskOrchestrationReviewOverlay) Apply(dst *TaskOrchestrationReviewConfig) {
	if o.DefaultPolicy != nil {
		dst.DefaultPolicy = *o.DefaultPolicy
	}
	if o.MaxRounds != nil {
		dst.MaxRounds = *o.MaxRounds
	}
	if o.MaxReviewAttempts != nil {
		dst.MaxReviewAttempts = *o.MaxReviewAttempts
	}
	if o.Timeout != nil {
		dst.Timeout = *o.Timeout
	}
	if o.RapidTerminalWindow != nil {
		dst.RapidTerminalWindow = *o.RapidTerminalWindow
	}
	if o.RapidTerminalLimit != nil {
		dst.RapidTerminalLimit = *o.RapidTerminalLimit
	}
	if o.MissingWorkMaxItems != nil {
		dst.MissingWorkMaxItems = *o.MissingWorkMaxItems
	}
	if o.MissingWorkItemMaxBytes != nil {
		dst.MissingWorkItemMaxBytes = *o.MissingWorkItemMaxBytes
	}
	if o.ReasonMaxBytes != nil {
		dst.ReasonMaxBytes = *o.ReasonMaxBytes
	}
	if o.ReviewTextMaxBytes != nil {
		dst.ReviewTextMaxBytes = *o.ReviewTextMaxBytes
	}
	if o.NextRoundGuidanceMaxBytes != nil {
		dst.NextRoundGuidanceMaxBytes = *o.NextRoundGuidanceMaxBytes
	}
	if o.FailurePolicy != nil {
		dst.FailurePolicy = *o.FailurePolicy
	}
}
