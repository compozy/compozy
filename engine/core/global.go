package core

import (
	"time"

	"dario.cat/mergo"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// SuccessTransition represents a success transition configuration
type SuccessTransition struct {
	Next *string `json:"next,omitempty" yaml:"next,omitempty" mapstructure:"next,omitempty"`
	With *Input  `json:"with,omitempty" yaml:"with,omitempty" mapstructure:"with,omitempty"`
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

// ErrorTransition represents an error transition configuration
type ErrorTransition struct {
	Next *string `json:"next,omitempty" yaml:"next,omitempty" mapstructure:"next,omitempty"`
	With *Input  `json:"with,omitempty" yaml:"with,omitempty" mapstructure:"with,omitempty"`
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

// RetryPolicyConfig defines the retry behavior for a transition
type RetryPolicyConfig struct {
	InitialInterval        string   `json:"initial_interval,omitempty"          yaml:"initial_interval,omitempty"          mapstructure:"initial_interval,omitempty"`
	BackoffCoefficient     float64  `json:"backoff_coefficient,omitempty"       yaml:"backoff_coefficient,omitempty"       mapstructure:"backoff_coefficient,omitempty"`
	MaximumAttempts        int32    `json:"maximum_attempts,omitempty"          yaml:"maximum_attempts,omitempty"          mapstructure:"maximum_attempts,omitempty"`
	MaximumInterval        string   `json:"maximum_interval,omitempty"          yaml:"maximum_interval,omitempty"          mapstructure:"maximum_interval,omitempty"`
	NonRetryableErrorTypes []string `json:"non_retryable_error_types,omitempty" yaml:"non_retryable_error_types,omitempty" mapstructure:"non_retryable_error_types,omitempty"`
}

type GlobalOpts struct {
	OnError                *ErrorTransition   `json:"on_error,omitempty"                  yaml:"on_error,omitempty"                  mapstructure:"on_error,omitempty"`
	RetryPolicy            *RetryPolicyConfig `json:"retry_policy,omitempty"              yaml:"retry_policy,omitempty"              mapstructure:"retry_policy,omitempty"`
	ScheduleToStartTimeout string             `json:"schedule_to_start_timeout,omitempty" yaml:"schedule_to_start_timeout,omitempty" mapstructure:"schedule_to_start_timeout,omitempty"`
	StartToCloseTimeout    string             `json:"start_to_close_timeout,omitempty"    yaml:"start_to_close_timeout,omitempty"    mapstructure:"start_to_close_timeout,omitempty"`
	ScheduleToCloseTimeout string             `json:"schedule_to_close_timeout,omitempty" yaml:"schedule_to_close_timeout,omitempty" mapstructure:"schedule_to_close_timeout,omitempty"`
	HeartbeatTimeout       string             `json:"heartbeat_timeout,omitempty"         yaml:"heartbeat_timeout,omitempty"         mapstructure:"heartbeat_timeout,omitempty"`
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
