package config

import (
	"fmt"
	"time"
)

// Validate ensures task config is safe to consume.
func (c TaskConfig) Validate() error {
	return c.Orchestration.Validate("task.orchestration")
}

// Validate ensures task orchestration config is safe to consume.
func (c TaskOrchestrationConfig) Validate(path string) error {
	if err := c.validateContext(path); err != nil {
		return err
	}
	if err := c.validateScheduler(path); err != nil {
		return err
	}
	if err := c.validateRuntime(path); err != nil {
		return err
	}
	if err := c.Profile.Validate(path + ".profile"); err != nil {
		return err
	}
	return c.Review.Validate(path + ".review")
}

func (c TaskOrchestrationConfig) validateContext(path string) error {
	if c.SummaryMaxBytes <= 0 {
		return fmt.Errorf("%s.summary_max_bytes must be positive: %d", path, c.SummaryMaxBytes)
	}
	if c.ContextBodyMaxBytes <= 0 {
		return fmt.Errorf("%s.context_body_max_bytes must be positive: %d", path, c.ContextBodyMaxBytes)
	}
	if c.ContextPriorAttempts < 0 {
		return fmt.Errorf("%s.context_prior_attempts must be >= 0: %d", path, c.ContextPriorAttempts)
	}
	if c.ContextRecentEvents < 0 {
		return fmt.Errorf("%s.context_recent_events must be >= 0: %d", path, c.ContextRecentEvents)
	}
	if c.SpawnFailureLimit <= 0 {
		return fmt.Errorf("%s.spawn_failure_limit must be positive: %d", path, c.SpawnFailureLimit)
	}
	return nil
}

func (c TaskOrchestrationConfig) validateScheduler(path string) error {
	if c.SchedulerBadTickThreshold <= 0 {
		return fmt.Errorf(
			"%s.scheduler_bad_tick_threshold must be positive: %d",
			path,
			c.SchedulerBadTickThreshold,
		)
	}
	if err := validateWholeSecondDuration(
		path+".scheduler_bad_tick_cooldown",
		c.SchedulerBadTickCooldown,
		false,
	); err != nil {
		return err
	}
	if err := validateWholeSecondDuration(path+".default_max_runtime", c.DefaultMaxRuntime, true); err != nil {
		return err
	}
	if c.DefaultMaxRuntime > MaxTaskOrchestrationRuntime {
		return fmt.Errorf(
			"%s.default_max_runtime must be <= %s: %s",
			path,
			MaxTaskOrchestrationRuntime,
			c.DefaultMaxRuntime,
		)
	}
	return nil
}

func (c TaskOrchestrationConfig) validateRuntime(path string) error {
	if err := validateWholeSecondDuration(
		path+".bridge_notification_timeout",
		c.BridgeNotificationTimeout,
		false,
	); err != nil {
		return err
	}
	if c.DesignatedRunMax <= 0 || c.DesignatedRunMax > MaxTaskDesignatedRunMax {
		return fmt.Errorf(
			"%s.designated_run_max must be between 1 and %d: %d",
			path,
			MaxTaskDesignatedRunMax,
			c.DesignatedRunMax,
		)
	}
	if c.MaxActiveRunsPerWorkspace < 0 {
		return fmt.Errorf(
			"%s.max_active_runs_per_workspace must be zero or positive: %d",
			path,
			c.MaxActiveRunsPerWorkspace,
		)
	}
	if err := validateWholeSecondDuration(path+".action_run_timeout", c.ActionRunTimeout, false); err != nil {
		return err
	}
	if c.ActionRunTimeout > MaxTaskOrchestrationRuntime {
		return fmt.Errorf(
			"%s.action_run_timeout must be <= %s: %s",
			path,
			MaxTaskOrchestrationRuntime,
			c.ActionRunTimeout,
		)
	}
	if c.NetworkStatusQueueSize <= 0 {
		return fmt.Errorf("%s.network_status_queue_size must be positive: %d", path, c.NetworkStatusQueueSize)
	}
	return validateWholeSecondDuration(path+".network_status_timeout", c.NetworkStatusTimeout, false)
}

// Validate ensures task execution profile defaults are recognized.
func (c TaskOrchestrationProfileConfig) Validate(path string) error {
	switch c.DefaultCoordinatorMode {
	case TaskCoordinatorModeInherit, TaskCoordinatorModeGuided:
	default:
		return fmt.Errorf(
			"%s.default_coordinator_mode must be %q or %q: %q",
			path,
			TaskCoordinatorModeInherit,
			TaskCoordinatorModeGuided,
			c.DefaultCoordinatorMode,
		)
	}
	if c.DefaultWorkerMode != TaskWorkerModeInherit {
		return fmt.Errorf("%s.default_worker_mode must be %q: %q", path, TaskWorkerModeInherit, c.DefaultWorkerMode)
	}
	switch c.DefaultSandboxMode {
	case TaskSandboxModeInherit, TaskSandboxModeNone:
	default:
		return fmt.Errorf(
			"%s.default_sandbox_mode must be %q or %q: %q",
			path,
			TaskSandboxModeInherit,
			TaskSandboxModeNone,
			c.DefaultSandboxMode,
		)
	}
	if c.DefaultSandboxMode == TaskSandboxModeNone && !c.AllowTaskSandboxNone {
		return fmt.Errorf("%s.default_sandbox_mode %q requires allow_task_sandbox_none", path, TaskSandboxModeNone)
	}
	return nil
}

// Validate ensures review gate defaults are bounded.
func (c TaskOrchestrationReviewConfig) Validate(path string) error {
	switch c.DefaultPolicy {
	case TaskReviewPolicyNone, TaskReviewPolicyOnSuccess, TaskReviewPolicyOnFailure, TaskReviewPolicyAlways:
	default:
		return fmt.Errorf("%s.default_policy is invalid: %q", path, c.DefaultPolicy)
	}
	if c.MaxRounds <= 0 {
		return fmt.Errorf("%s.max_rounds must be positive: %d", path, c.MaxRounds)
	}
	if c.MaxReviewAttempts <= 0 {
		return fmt.Errorf("%s.max_review_attempts must be positive: %d", path, c.MaxReviewAttempts)
	}
	if err := validateWholeSecondDuration(path+".timeout", c.Timeout, false); err != nil {
		return err
	}
	if err := validateWholeSecondDuration(path+".rapid_terminal_window", c.RapidTerminalWindow, false); err != nil {
		return err
	}
	if c.RapidTerminalLimit <= 0 {
		return fmt.Errorf("%s.rapid_terminal_limit must be positive: %d", path, c.RapidTerminalLimit)
	}
	if c.MissingWorkMaxItems <= 0 {
		return fmt.Errorf("%s.missing_work_max_items must be positive: %d", path, c.MissingWorkMaxItems)
	}
	if c.MissingWorkItemMaxBytes <= 0 {
		return fmt.Errorf("%s.missing_work_item_max_bytes must be positive: %d", path, c.MissingWorkItemMaxBytes)
	}
	if c.ReasonMaxBytes <= 0 {
		return fmt.Errorf("%s.reason_max_bytes must be positive: %d", path, c.ReasonMaxBytes)
	}
	if c.ReviewTextMaxBytes <= 0 {
		return fmt.Errorf("%s.review_text_max_bytes must be positive: %d", path, c.ReviewTextMaxBytes)
	}
	if c.NextRoundGuidanceMaxBytes <= 0 {
		return fmt.Errorf("%s.next_round_guidance_max_bytes must be positive: %d", path, c.NextRoundGuidanceMaxBytes)
	}
	switch c.FailurePolicy {
	case TaskReviewFailureBlockTask, TaskReviewFailureFailTask:
	default:
		return fmt.Errorf("%s.failure_policy is invalid: %q", path, c.FailurePolicy)
	}
	return nil
}

func validateWholeSecondDuration(path string, value time.Duration, allowZero bool) error {
	if value < 0 {
		return fmt.Errorf("%s must be >= 0: %s", path, value)
	}
	if value == 0 {
		if allowZero {
			return nil
		}
		return fmt.Errorf("%s must be positive: %s", path, value)
	}
	if value%time.Second != 0 {
		return fmt.Errorf("%s must use whole-second precision: %s", path, value)
	}
	return nil
}
