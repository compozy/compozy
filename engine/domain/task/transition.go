package task

import "github.com/compozy/compozy/engine/common"

// SuccessTransitionConfig represents a success transition configuration
type SuccessTransitionConfig struct {
	Next *string       `json:"next,omitempty" yaml:"next,omitempty"`
	With *common.Input `json:"with,omitempty" yaml:"with,omitempty"`
}

// RetryPolicyConfig defines the retry behavior for a transition
type RetryPolicyConfig struct {
	MaxAttempts    *int     `json:"max_attempts,omitempty"    yaml:"max_attempts,omitempty"`    // Maximum number of retry attempts
	BackoffInitial *string  `json:"backoff_initial,omitempty" yaml:"backoff_initial,omitempty"` // Initial backoff duration (e.g., "1s", "500ms")
	BackoffMax     *string  `json:"backoff_max,omitempty"     yaml:"backoff_max,omitempty"`     // Maximum backoff duration (e.g., "30s", "1m")
	BackoffFactor  *float64 `json:"backoff_factor,omitempty"  yaml:"backoff_factor,omitempty"`  // Multiplicative factor for backoff duration (e.g., 2.0)
}

// ErrorTransitionConfig represents an error transition configuration
type ErrorTransitionConfig struct {
	Next        *string            `json:"next,omitempty"        yaml:"next,omitempty"`
	With        *common.Input      `json:"with,omitempty"        yaml:"with,omitempty"`
	RetryPolicy *RetryPolicyConfig `json:"retry_policy,omitempty" yaml:"retry_policy,omitempty"`
}
