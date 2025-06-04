package core

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func TestParseHumanDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
		hasError bool
	}{
		{
			name:     "Should parse 1 second",
			input:    "1 second",
			expected: time.Second,
			hasError: false,
		},
		{
			name:     "Should parse 1 minute",
			input:    "1 minute",
			expected: time.Minute,
			hasError: false,
		},
		{
			name:     "Should parse 30 minutes",
			input:    "30 minutes",
			expected: 30 * time.Minute,
			hasError: false,
		},
		{
			name:     "Should parse 35 minutes",
			input:    "35 minutes",
			expected: 35 * time.Minute,
			hasError: false,
		},
		{
			name:     "Should parse 1 hour",
			input:    "1 hour",
			expected: time.Hour,
			hasError: false,
		},
		{
			name:     "Should parse 2 hours",
			input:    "2 hours",
			expected: 2 * time.Hour,
			hasError: false,
		},
		{
			name:     "Should parse Go format 1s",
			input:    "1s",
			expected: time.Second,
			hasError: false,
		},
		{
			name:     "Should parse Go format 30m",
			input:    "30m",
			expected: 30 * time.Minute,
			hasError: false,
		},
		{
			name:     "Should parse Go format 1h30m",
			input:    "1h30m",
			expected: time.Hour + 30*time.Minute,
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseHumanDuration(tt.input)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestResolveActivityOptions(t *testing.T) {
	t.Run("Should apply default options when no global options provided", func(t *testing.T) {
		resolved := ResolveActivityOptions(nil, nil, nil)

		assert.Equal(t, "1 minute", resolved.ScheduleToStartTimeout)
		assert.Equal(t, "30 minutes", resolved.StartToCloseTimeout)
		assert.Equal(t, "35 minutes", resolved.ScheduleToCloseTimeout)
		assert.Equal(t, "", resolved.HeartbeatTimeout)

		require.NotNil(t, resolved.RetryPolicy)
		assert.Equal(t, "1 second", resolved.RetryPolicy.InitialInterval)
		assert.Equal(t, float64(2.0), resolved.RetryPolicy.BackoffCoefficient)
		assert.Equal(t, "1 minute", resolved.RetryPolicy.MaximumInterval)
		assert.Equal(t, int32(3), resolved.RetryPolicy.MaximumAttempts)
	})

	t.Run("Should override with project options", func(t *testing.T) {
		projectOpts := &GlobalOpts{
			StartToCloseTimeout: "1 hour",
			RetryPolicy: &RetryPolicyConfig{
				MaximumAttempts: 5,
			},
		}

		resolved := ResolveActivityOptions(projectOpts, nil, nil)

		assert.Equal(t, "1 hour", resolved.StartToCloseTimeout)
		assert.Equal(t, "1 minute", resolved.ScheduleToStartTimeout) // default
		assert.Equal(t, int32(5), resolved.RetryPolicy.MaximumAttempts)
		assert.Equal(t, "1 second", resolved.RetryPolicy.InitialInterval) // default
	})

	t.Run("Should override with workflow options", func(t *testing.T) {
		projectOpts := &GlobalOpts{
			StartToCloseTimeout: "1 hour",
		}
		workflowOpts := &GlobalOpts{
			StartToCloseTimeout: "2 hours",
		}

		resolved := ResolveActivityOptions(projectOpts, workflowOpts, nil)

		assert.Equal(t, "2 hours", resolved.StartToCloseTimeout) // workflow overrides project
	})

	t.Run("Should override with task options", func(t *testing.T) {
		projectOpts := &GlobalOpts{
			StartToCloseTimeout: "1 hour",
		}
		workflowOpts := &GlobalOpts{
			StartToCloseTimeout: "2 hours",
		}
		taskOpts := &GlobalOpts{
			StartToCloseTimeout: "3 hours",
		}

		resolved := ResolveActivityOptions(projectOpts, workflowOpts, taskOpts)

		assert.Equal(t, "3 hours", resolved.StartToCloseTimeout) // task overrides all
	})
}

func TestToTemporalActivityOptions(t *testing.T) {
	t.Run("Should return exact default values when no global options provided", func(t *testing.T) {
		// Test with completely empty resolved options (no project, workflow, or task options)
		resolved := ResolveActivityOptions(nil, nil, nil)
		opts := resolved.ToTemporalActivityOptions()

		// Verify it matches the expected default structure exactly
		expected := workflow.ActivityOptions{
			StartToCloseTimeout: 30 * time.Minute,
			RetryPolicy: &temporal.RetryPolicy{
				InitialInterval:    time.Second,
				BackoffCoefficient: 2.0,
				MaximumInterval:    time.Minute,
				MaximumAttempts:    3,
			},
		}

		// Essential checks for the default behavior
		assert.Equal(t, expected.StartToCloseTimeout, opts.StartToCloseTimeout)
		require.NotNil(t, opts.RetryPolicy)
		assert.Equal(t, expected.RetryPolicy.InitialInterval, opts.RetryPolicy.InitialInterval)
		assert.Equal(t, expected.RetryPolicy.BackoffCoefficient, opts.RetryPolicy.BackoffCoefficient)
		assert.Equal(t, expected.RetryPolicy.MaximumInterval, opts.RetryPolicy.MaximumInterval)
		assert.Equal(t, expected.RetryPolicy.MaximumAttempts, opts.RetryPolicy.MaximumAttempts)

		// Additional timeouts that should be set by defaults but not in the minimal expected structure
		assert.Equal(t, time.Minute, opts.ScheduleToStartTimeout)    // from "1 minute" default
		assert.Equal(t, 35*time.Minute, opts.ScheduleToCloseTimeout) // from "35 minutes" default
		assert.Equal(t, time.Duration(0), opts.HeartbeatTimeout)     // should be 0 when not set
		assert.Empty(t, opts.RetryPolicy.NonRetryableErrorTypes)
	})

	t.Run("Should convert default options correctly", func(t *testing.T) {
		resolved := ResolveActivityOptions(nil, nil, nil)
		opts := resolved.ToTemporalActivityOptions()

		// Check timeouts
		assert.Equal(t, time.Minute, opts.ScheduleToStartTimeout)
		assert.Equal(t, 30*time.Minute, opts.StartToCloseTimeout)
		assert.Equal(t, 35*time.Minute, opts.ScheduleToCloseTimeout)
		assert.Equal(t, time.Duration(0), opts.HeartbeatTimeout) // should be 0 when not set

		// Check retry policy
		require.NotNil(t, opts.RetryPolicy)
		assert.Equal(t, time.Second, opts.RetryPolicy.InitialInterval)
		assert.Equal(t, float64(2.0), opts.RetryPolicy.BackoffCoefficient)
		assert.Equal(t, time.Minute, opts.RetryPolicy.MaximumInterval)
		assert.Equal(t, int32(3), opts.RetryPolicy.MaximumAttempts)
		assert.Empty(t, opts.RetryPolicy.NonRetryableErrorTypes)
	})

	t.Run("Should ensure minimum timeout requirements", func(t *testing.T) {
		resolved := &ResolvedActivityOptions{
			ScheduleToStartTimeout: "",
			StartToCloseTimeout:    "",
			ScheduleToCloseTimeout: "",
			HeartbeatTimeout:       "",
			RetryPolicy: &RetryPolicyConfig{
				InitialInterval:    "1 second",
				BackoffCoefficient: 2.0,
				MaximumInterval:    "1 minute",
				MaximumAttempts:    3,
			},
		}

		opts := resolved.ToTemporalActivityOptions()

		// Should set default StartToCloseTimeout when no timeouts are provided
		assert.Equal(t, 30*time.Minute, opts.StartToCloseTimeout)
		assert.Equal(t, time.Duration(0), opts.ScheduleToStartTimeout)
		assert.Equal(t, time.Duration(0), opts.ScheduleToCloseTimeout)
	})

	t.Run("Should handle custom timeout values", func(t *testing.T) {
		resolved := &ResolvedActivityOptions{
			ScheduleToStartTimeout: "2 minutes",
			StartToCloseTimeout:    "1 hour",
			ScheduleToCloseTimeout: "2 hours",
			HeartbeatTimeout:       "30 minutes",
			RetryPolicy: &RetryPolicyConfig{
				InitialInterval:        "2 seconds",
				BackoffCoefficient:     3.0,
				MaximumInterval:        "5 minutes",
				MaximumAttempts:        10,
				NonRetryableErrorTypes: []string{"CustomError"},
			},
		}

		opts := resolved.ToTemporalActivityOptions()

		assert.Equal(t, 2*time.Minute, opts.ScheduleToStartTimeout)
		assert.Equal(t, time.Hour, opts.StartToCloseTimeout)
		assert.Equal(t, 2*time.Hour, opts.ScheduleToCloseTimeout)
		assert.Equal(t, 30*time.Minute, opts.HeartbeatTimeout)

		require.NotNil(t, opts.RetryPolicy)
		assert.Equal(t, 2*time.Second, opts.RetryPolicy.InitialInterval)
		assert.Equal(t, float64(3.0), opts.RetryPolicy.BackoffCoefficient)
		assert.Equal(t, 5*time.Minute, opts.RetryPolicy.MaximumInterval)
		assert.Equal(t, int32(10), opts.RetryPolicy.MaximumAttempts)
		assert.Equal(t, []string{"CustomError"}, opts.RetryPolicy.NonRetryableErrorTypes)
	})

	t.Run("Should handle invalid duration strings gracefully", func(t *testing.T) {
		resolved := &ResolvedActivityOptions{
			ScheduleToStartTimeout: "invalid duration",
			StartToCloseTimeout:    "another invalid",
			ScheduleToCloseTimeout: "",
			HeartbeatTimeout:       "",
			RetryPolicy: &RetryPolicyConfig{
				InitialInterval: "invalid",
				MaximumInterval: "also invalid",
			},
		}

		opts := resolved.ToTemporalActivityOptions()

		// Should fallback to defaults when parsing fails
		assert.Equal(t, time.Duration(0), opts.ScheduleToStartTimeout)
		assert.Equal(t, 30*time.Minute, opts.StartToCloseTimeout) // default fallback
		assert.Equal(t, time.Duration(0), opts.ScheduleToCloseTimeout)

		require.NotNil(t, opts.RetryPolicy)
		assert.Equal(t, time.Duration(0), opts.RetryPolicy.InitialInterval)
		assert.Equal(t, time.Duration(0), opts.RetryPolicy.MaximumInterval)
	})
}

func TestConvertHumanToGoFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Should convert 1 second",
			input:    "1 second",
			expected: "1s",
		},
		{
			name:     "Should convert 1 minute",
			input:    "1 minute",
			expected: "1m",
		},
		{
			name:     "Should convert 30 minutes",
			input:    "30 minutes",
			expected: "30m",
		},
		{
			name:     "Should convert 35 minutes",
			input:    "35 minutes",
			expected: "35m",
		},
		{
			name:     "Should convert 2 minutes",
			input:    "2 minutes",
			expected: "2m",
		},
		{
			name:     "Should convert 2 seconds",
			input:    "2 seconds",
			expected: "2s",
		},
		{
			name:     "Should convert 5 minutes",
			input:    "5 minutes",
			expected: "5m",
		},
		{
			name:     "Should convert 1 hour",
			input:    "1 hour",
			expected: "1h",
		},
		{
			name:     "Should convert 2 hours",
			input:    "2 hours",
			expected: "2h",
		},
		{
			name:     "Should leave unknown formats unchanged",
			input:    "3 days",
			expected: "3 days",
		},
		{
			name:     "Should leave Go format unchanged",
			input:    "1h30m",
			expected: "1h30m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertHumanToGoFormat(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
