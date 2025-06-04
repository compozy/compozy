package task

import "github.com/compozy/compozy/engine/core"

// SuccessTransition represents a success transition configuration
type SuccessTransition struct {
	Next *string     `json:"next,omitempty" yaml:"next,omitempty"`
	With *core.Input `json:"with,omitempty" yaml:"with,omitempty"`
}

// RetryPolicyConfig defines the retry behavior for a transition
type RetryPolicyConfig struct {
	// Maximum number of retry attempts
	MaxAttempts *int `json:"max_attempts,omitempty" yaml:"max_attempts,omitempty"`
	// Initial backoff duration (e.g., "1s", "500ms")
	BackoffInitial *string `json:"backoff_initial,omitempty" yaml:"backoff_initial,omitempty"`
	// Maximum backoff duration (e.g., "30s", "1m")
	BackoffMax *string `json:"backoff_max,omitempty"     yaml:"backoff_max,omitempty"`
	// Multiplicative factor for backoff duration (e.g., 2.0)
	BackoffFactor *float64 `json:"backoff_factor,omitempty"  yaml:"backoff_factor,omitempty"`
}

// ErrorTransition represents an error transition configuration
type ErrorTransition struct {
	Next        *string            `json:"next,omitempty"         yaml:"next,omitempty"`
	With        *core.Input        `json:"with,omitempty"         yaml:"with,omitempty"`
	RetryPolicy *RetryPolicyConfig `json:"retry_policy,omitempty" yaml:"retry_policy,omitempty"`
}
