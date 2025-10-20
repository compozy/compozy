package core

import (
	"time"

	"dario.cat/mergo"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// SuccessTransition defines the next task to execute after successful completion.
//
// **Usage**:
// - **Next**: ID of the task to execute next
// - **With**: Optional input parameters to pass to the next task
type SuccessTransition struct {
	// ID of the next task to execute
	// - **Example:** `"process-results"`, `"send-notification"`
	Next *string `json:"next,omitempty" yaml:"next,omitempty" mapstructure:"next,omitempty"`
	// Input parameters to pass to the next task
	// - **Supports:** Template expressions like `{ "data": "{{ .output.result }}" }`
	With *Input `json:"with,omitempty" yaml:"with,omitempty" mapstructure:"with,omitempty"`
}

// GetWith returns the With field of the transition
func (t *SuccessTransition) GetWith() *Input {
	return t.With
}

// AsMap converts the provider configuration to a map for template normalization
func (t *SuccessTransition) AsMap() (map[string]any, error) {
	return AsMapDefault(t)
}

// FromMap updates the provider configuration from a normalized map
func (t *SuccessTransition) FromMap(data any) error {
	config, err := FromMapDefault[SuccessTransition](data)
	if err != nil {
		return err
	}
	return mergo.Merge(t, config, mergo.WithOverride)
}

// ErrorTransition defines error handling behavior when a task fails.
//
// **Usage**:
// - **Next**: ID of the error handler task
// - **With**: Error context and recovery parameters
type ErrorTransition struct {
	// ID of the error handler task
	//
	// - **Example**: "handle-error", "retry-with-fallback"
	Next *string `json:"next,omitempty" yaml:"next,omitempty" mapstructure:"next,omitempty"`
	// Error context passed to the handler
	// Includes error details: { "error": "{{ .error }}", "attempt": "{{ .retryCount }}" }
	With *Input `json:"with,omitempty" yaml:"with,omitempty" mapstructure:"with,omitempty"`
}

// GetWith returns the With field of the transition
func (t *ErrorTransition) GetWith() *Input {
	return t.With
}

// AsMap converts the provider configuration to a map for template normalization
func (t *ErrorTransition) AsMap() (map[string]any, error) {
	return AsMapDefault(t)
}

// FromMap updates the provider configuration from a normalized map
func (t *ErrorTransition) FromMap(data any) error {
	config, err := FromMapDefault[ErrorTransition](data)
	if err != nil {
		return err
	}
	return mergo.Merge(t, config, mergo.WithOverride)
}

// RetryPolicyConfig defines automatic retry behavior for failed tasks.
//
// **Features**:
// - **Exponential backoff**: Gradually increases delay between retries
// - **Maximum attempts**: Prevents infinite retry loops
// - **Error filtering**: Skip retries for specific error types
type RetryPolicyConfig struct {
	// Initial delay before first retry
	// - **Default:** `"1s"`
	// - **Example:** `"500ms"`, `"2s"`, `"1m"`
	InitialInterval string `json:"initial_interval,omitempty"          yaml:"initial_interval,omitempty"          mapstructure:"initial_interval,omitempty"`
	// Multiplier for exponential backoff
	// - **Default:** `2.0` (doubles each time)
	// - **Example:** `1.5`, `2.0`, `3.0`
	BackoffCoefficient float64 `json:"backoff_coefficient,omitempty"       yaml:"backoff_coefficient,omitempty"       mapstructure:"backoff_coefficient,omitempty"`
	// Maximum retry attempts
	// - **Default:** `3`
	// - **Example:** `5` for critical operations
	MaximumAttempts int32 `json:"maximum_attempts,omitempty"          yaml:"maximum_attempts,omitempty"          mapstructure:"maximum_attempts,omitempty"`
	// Maximum delay between retries
	// - **Default:** `"1m"`
	// - **Example:** `"30s"`, `"5m"`, `"1h"`
	MaximumInterval string `json:"maximum_interval,omitempty"          yaml:"maximum_interval,omitempty"          mapstructure:"maximum_interval,omitempty"`
	// Error types that should not trigger retries
	// - **Example:** `["ValidationError", "AuthenticationError"]`
	NonRetryableErrorTypes []string `json:"non_retryable_error_types,omitempty" yaml:"non_retryable_error_types,omitempty" mapstructure:"non_retryable_error_types,omitempty"`
}

// GlobalOpts contains workflow execution options that can be configured at multiple levels.
//
// **Hierarchy**: Project → Workflow → Task (each level overrides the previous)
//
// **Features**:
// - **Error handling**: Define fallback behavior for failures
// - **Retry policies**: Automatic retry with exponential backoff
// - **Timeout controls**: Prevent hung tasks and enforce SLAs
type GlobalOpts struct {
	// Error handler configuration
	// Defines what happens when a task fails after all retries
	OnError *ErrorTransition `json:"on_error,omitempty"                  yaml:"on_error,omitempty"                  mapstructure:"on_error,omitempty"`
	// Retry configuration for transient failures
	// Automatically retries failed tasks with exponential backoff
	RetryPolicy *RetryPolicyConfig `json:"retry_policy,omitempty"              yaml:"retry_policy,omitempty"              mapstructure:"retry_policy,omitempty"`
	// Maximum time to wait for a task to start executing
	// Default: "1m"
	//
	// - **Example**: "30s", "5m", "1h"
	ScheduleToStartTimeout string `json:"schedule_to_start_timeout,omitempty" yaml:"schedule_to_start_timeout,omitempty" mapstructure:"schedule_to_start_timeout,omitempty"`
	// Maximum time for task execution once started
	// Default: "5m"
	//
	// - **Example**: "30s", "10m", "1h"
	StartToCloseTimeout string `json:"start_to_close_timeout,omitempty"    yaml:"start_to_close_timeout,omitempty"    mapstructure:"start_to_close_timeout,omitempty"`
	// Total timeout from scheduling to completion
	// Default: "6m"
	//
	// - **Example**: "1m", "15m", "2h"
	ScheduleToCloseTimeout string `json:"schedule_to_close_timeout,omitempty" yaml:"schedule_to_close_timeout,omitempty" mapstructure:"schedule_to_close_timeout,omitempty"`
	// Interval for task heartbeat signals
	// Used for long-running tasks to indicate progress
	//
	// - **Example**: "10s", "30s", "1m"
	HeartbeatTimeout string `json:"heartbeat_timeout,omitempty"         yaml:"heartbeat_timeout,omitempty"         mapstructure:"heartbeat_timeout,omitempty"`
}

// ResolvedActivityOptions contains the final resolved activity options
type ResolvedActivityOptions struct {
	ScheduleToStartTimeout string
	StartToCloseTimeout    string
	ScheduleToCloseTimeout string
	HeartbeatTimeout       string
	RetryPolicy            *RetryPolicyConfig
}

// -----------------------------------------------------------------------------
// Activity Options Resolution
// -----------------------------------------------------------------------------

// ResolveActivityOptions resolves activity options in hierarchical order:
// project -> workflow -> task, with each level overriding the previous
func ResolveActivityOptions(
	projectOpts, workflowOpts *GlobalOpts,
	taskOpts *GlobalOpts,
) *ResolvedActivityOptions {
	resolved := &ResolvedActivityOptions{
		RetryPolicy: &RetryPolicyConfig{},
	}
	// Start with defaults
	applyDefaultTimeouts(resolved)
	applyDefaultRetryPolicy(resolved.RetryPolicy)
	// Apply project-level options
	if projectOpts != nil {
		mergeTimeouts(resolved, projectOpts)
		if projectOpts.RetryPolicy != nil {
			mergeRetryPolicy(resolved.RetryPolicy, projectOpts.RetryPolicy)
		}
	}
	// Apply workflow-level options (overrides project)
	if workflowOpts != nil {
		mergeTimeouts(resolved, workflowOpts)
		if workflowOpts.RetryPolicy != nil {
			mergeRetryPolicy(resolved.RetryPolicy, workflowOpts.RetryPolicy)
		}
	}
	// Apply task-level options (overrides workflow and project)
	if taskOpts != nil {
		mergeTimeouts(resolved, taskOpts)
		if taskOpts.RetryPolicy != nil {
			mergeRetryPolicy(resolved.RetryPolicy, taskOpts.RetryPolicy)
		}
	}
	return resolved
}

// ToTemporalActivityOptions converts resolved options to temporal ActivityOptions
func (r *ResolvedActivityOptions) ToTemporalActivityOptions() workflow.ActivityOptions {
	opts := workflow.ActivityOptions{}
	r.setTimeouts(&opts)
	r.setDefaultTimeouts(&opts)
	r.setRetryPolicy(&opts)
	return opts
}

// setTimeouts sets timeout values from resolved options
func (r *ResolvedActivityOptions) setTimeouts(opts *workflow.ActivityOptions) {
	setTimeoutIfValid(r.ScheduleToStartTimeout, &opts.ScheduleToStartTimeout)
	setTimeoutIfValid(r.StartToCloseTimeout, &opts.StartToCloseTimeout)
	setTimeoutIfValid(r.ScheduleToCloseTimeout, &opts.ScheduleToCloseTimeout)
	setTimeoutIfValid(r.HeartbeatTimeout, &opts.HeartbeatTimeout)
}

// setDefaultTimeouts ensures at least one required timeout is set
func (r *ResolvedActivityOptions) setDefaultTimeouts(opts *workflow.ActivityOptions) {
	if opts.StartToCloseTimeout == 0 && opts.ScheduleToCloseTimeout == 0 {
		opts.StartToCloseTimeout = 5 * time.Minute
	}
}

// setRetryPolicy sets retry policy from resolved options
func (r *ResolvedActivityOptions) setRetryPolicy(opts *workflow.ActivityOptions) {
	if r.RetryPolicy == nil {
		return
	}
	retryPolicy := &temporal.RetryPolicy{}
	r.populateRetryPolicy(retryPolicy)
	opts.RetryPolicy = retryPolicy
}

// populateRetryPolicy fills retry policy fields from configuration
func (r *ResolvedActivityOptions) populateRetryPolicy(retryPolicy *temporal.RetryPolicy) {
	setTimeoutIfValid(r.RetryPolicy.InitialInterval, &retryPolicy.InitialInterval)
	setTimeoutIfValid(r.RetryPolicy.MaximumInterval, &retryPolicy.MaximumInterval)
	if r.RetryPolicy.BackoffCoefficient > 0 {
		retryPolicy.BackoffCoefficient = r.RetryPolicy.BackoffCoefficient
	}
	if r.RetryPolicy.MaximumAttempts > 0 {
		retryPolicy.MaximumAttempts = r.RetryPolicy.MaximumAttempts
	}
	if len(r.RetryPolicy.NonRetryableErrorTypes) > 0 {
		retryPolicy.NonRetryableErrorTypes = r.RetryPolicy.NonRetryableErrorTypes
	}
}

// setTimeoutIfValid parses duration string and sets timeout if valid
func setTimeoutIfValid(durationStr string, target *time.Duration) {
	if durationStr != "" {
		if d, err := ParseHumanDuration(durationStr); err == nil {
			*target = d
		}
	}
}

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

var (
	defaultStartToCloseTimeout           = "5 minutes"
	defaultScheduleToStartTimeout        = "1 minute"
	defaultScheduleToCloseTimeout        = "6 minutes"
	defaultRetryPolicyInitialInterval    = "1 second"
	defaultRetryPolicyBackoffCoefficient = 2.0
	defaultRetryPolicyMaximumInterval    = "1 minute"
	defaultRetryPolicyMaximumAttempts    = int32(3)
)

func applyDefaultTimeouts(resolved *ResolvedActivityOptions) {
	resolved.StartToCloseTimeout = defaultStartToCloseTimeout
	resolved.ScheduleToStartTimeout = defaultScheduleToStartTimeout
	resolved.ScheduleToCloseTimeout = defaultScheduleToCloseTimeout
}

func applyDefaultRetryPolicy(policy *RetryPolicyConfig) {
	policy.InitialInterval = defaultRetryPolicyInitialInterval
	policy.BackoffCoefficient = defaultRetryPolicyBackoffCoefficient
	policy.MaximumInterval = defaultRetryPolicyMaximumInterval
	policy.MaximumAttempts = defaultRetryPolicyMaximumAttempts
}

func mergeTimeouts(target *ResolvedActivityOptions, source *GlobalOpts) {
	if source.ScheduleToStartTimeout != "" {
		target.ScheduleToStartTimeout = source.ScheduleToStartTimeout
	}
	if source.StartToCloseTimeout != "" {
		target.StartToCloseTimeout = source.StartToCloseTimeout
	}
	if source.ScheduleToCloseTimeout != "" {
		target.ScheduleToCloseTimeout = source.ScheduleToCloseTimeout
	}
	if source.HeartbeatTimeout != "" {
		target.HeartbeatTimeout = source.HeartbeatTimeout
	}
}

func mergeRetryPolicy(target, source *RetryPolicyConfig) {
	if source.InitialInterval != "" {
		target.InitialInterval = source.InitialInterval
	}
	if source.BackoffCoefficient > 0 {
		target.BackoffCoefficient = source.BackoffCoefficient
	}
	if source.MaximumInterval != "" {
		target.MaximumInterval = source.MaximumInterval
	}
	if source.MaximumAttempts > 0 {
		target.MaximumAttempts = source.MaximumAttempts
	}
	if len(source.NonRetryableErrorTypes) > 0 {
		target.NonRetryableErrorTypes = source.NonRetryableErrorTypes
	}
}
