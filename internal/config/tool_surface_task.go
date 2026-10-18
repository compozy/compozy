package config

const (
	toolSurfaceTaskOrchestrationStatusProjectionQueueSizePath     = "task.orchestration.status_projection_queue_size"
	toolSurfaceTaskOrchestrationStatusProjectionTimeoutPath       = "task.orchestration.status_projection_timeout"
	toolSurfaceTaskOrchestrationContextBodyMaxBytesPath           = "task.orchestration.context_body_max_bytes"
	toolSurfaceTaskOrchestrationContextPriorAttemptsPath          = "task.orchestration.context_prior_attempts"
	toolSurfaceTaskOrchestrationContextRecentEventsPath           = "task.orchestration.context_recent_events"
	toolSurfaceTaskOrchestrationDefaultMaxRuntimePath             = "task.orchestration.default_max_runtime"
	toolSurfaceTaskOrchestrationDesignatedRunMaxPath              = "task.orchestration.designated_run_max"
	toolSurfaceTaskOrchestrationMaxActiveRunsPerWorkspacePath     = "task.orchestration.max_active_runs_per_workspace"
	toolSurfaceTaskOrchestrationProfileDefaultCoordinatorModePath = "task.orchestration.profile.default_coordinator_mode"
	toolSurfaceTaskOrchestrationProfileDefaultWorkerModePath      = "task.orchestration.profile.default_worker_mode"
	toolSurfaceTaskOrchestrationProfileDefaultWorktreeModePath    = "task.orchestration.profile.default_worktree_mode"
	toolSurfaceTaskOrchestrationReviewDefaultPolicyPath           = "task.orchestration.review.default_policy"
	toolSurfaceTaskOrchestrationReviewFailurePolicyPath           = "task.orchestration.review.failure_policy"
	toolSurfaceTaskOrchestrationReviewMaxReviewAttemptsPath       = "task.orchestration.review.max_review_attempts"
	toolSurfaceTaskOrchestrationReviewMaxRoundsPath               = "task.orchestration.review.max_rounds"
	reviewMissingWorkItemBytesPath                                = "task.orchestration.review.missing_work_item_max_bytes"
	toolSurfaceTaskOrchestrationReviewMissingWorkMaxItemsPath     = "task.orchestration.review.missing_work_max_items"
	reviewNextGuidanceBytesPath                                   = "task.orchestration.review." +
		"next_round_guidance_max_bytes"
	toolSurfaceTaskOrchestrationReviewRapidTerminalLimitPath  = "task.orchestration.review.rapid_terminal_limit"
	toolSurfaceTaskOrchestrationReviewRapidTerminalWindowPath = "task.orchestration.review.rapid_terminal_window"
	toolSurfaceTaskOrchestrationReviewReasonMaxBytesPath      = "task.orchestration.review.reason_max_bytes"
	toolSurfaceTaskOrchestrationReviewReviewTextMaxBytesPath  = "task.orchestration.review.review_text_max_bytes"
	toolSurfaceTaskOrchestrationReviewTimeoutPath             = "task.orchestration.review.timeout"
	toolSurfaceTaskOrchestrationSchedulerBadTickCooldownPath  = "task.orchestration.scheduler_bad_tick_cooldown"
	toolSurfaceTaskOrchestrationSchedulerBadTickThresholdPath = "task.orchestration.scheduler_bad_tick_threshold"
	toolSurfaceTaskOrchestrationSpawnFailureLimitPath         = "task.orchestration.spawn_failure_limit"
	toolSurfaceTaskOrchestrationSummaryMaxBytesPath           = "task.orchestration.summary_max_bytes"
	toolSurfaceTaskRecoveryAllowAgentForcePath                = "task.recovery.allow_agent_force"
)

func taskToolSurfaceMutableConfigKinds() map[string]ValueKind {
	return map[string]ValueKind{
		toolSurfaceTaskOrchestrationStatusProjectionQueueSizePath:     ConfigValueInt,
		toolSurfaceTaskOrchestrationStatusProjectionTimeoutPath:       ConfigValueDuration,
		toolSurfaceTaskOrchestrationSummaryMaxBytesPath:               ConfigValueInt,
		toolSurfaceTaskOrchestrationContextBodyMaxBytesPath:           ConfigValueInt,
		toolSurfaceTaskOrchestrationContextPriorAttemptsPath:          ConfigValueInt,
		toolSurfaceTaskOrchestrationContextRecentEventsPath:           ConfigValueInt,
		toolSurfaceTaskOrchestrationSpawnFailureLimitPath:             ConfigValueInt,
		toolSurfaceTaskOrchestrationSchedulerBadTickThresholdPath:     ConfigValueInt,
		toolSurfaceTaskOrchestrationSchedulerBadTickCooldownPath:      ConfigValueDuration,
		toolSurfaceTaskOrchestrationDefaultMaxRuntimePath:             ConfigValueDuration,
		toolSurfaceTaskOrchestrationDesignatedRunMaxPath:              ConfigValueInt,
		toolSurfaceTaskOrchestrationMaxActiveRunsPerWorkspacePath:     ConfigValueInt,
		toolSurfaceTaskOrchestrationProfileDefaultCoordinatorModePath: ConfigValueString,
		toolSurfaceTaskOrchestrationProfileDefaultWorkerModePath:      ConfigValueString,
		toolSurfaceTaskOrchestrationProfileDefaultWorktreeModePath:    ConfigValueString,
		"task.orchestration.profile.allow_task_provider_override":     ConfigValueBool,
		toolSurfaceTaskOrchestrationReviewDefaultPolicyPath:           ConfigValueString,
		toolSurfaceTaskOrchestrationReviewMaxRoundsPath:               ConfigValueInt,
		toolSurfaceTaskOrchestrationReviewMaxReviewAttemptsPath:       ConfigValueInt,
		toolSurfaceTaskOrchestrationReviewTimeoutPath:                 ConfigValueDuration,
		toolSurfaceTaskOrchestrationReviewRapidTerminalWindowPath:     ConfigValueDuration,
		toolSurfaceTaskOrchestrationReviewRapidTerminalLimitPath:      ConfigValueInt,
		toolSurfaceTaskOrchestrationReviewMissingWorkMaxItemsPath:     ConfigValueInt,
		reviewMissingWorkItemBytesPath:                                ConfigValueInt,
		toolSurfaceTaskOrchestrationReviewReasonMaxBytesPath:          ConfigValueInt,
		toolSurfaceTaskOrchestrationReviewReviewTextMaxBytesPath:      ConfigValueInt,
		reviewNextGuidanceBytesPath:                                   ConfigValueInt,
		toolSurfaceTaskOrchestrationReviewFailurePolicyPath:           ConfigValueString,
		toolSurfaceTaskRecoveryAllowAgentForcePath:                    ConfigValueBool,
	}
}
