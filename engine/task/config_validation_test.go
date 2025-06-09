package task

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateStrategy(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
		expected bool
	}{
		{
			name:     "wait_all should be valid",
			strategy: "wait_all",
			expected: true,
		},
		{
			name:     "fail_fast should be valid",
			strategy: "fail_fast",
			expected: true,
		},
		{
			name:     "best_effort should be valid",
			strategy: "best_effort",
			expected: true,
		},
		{
			name:     "race should be valid",
			strategy: "race",
			expected: true,
		},
		{
			name:     "invalid_strategy should be invalid",
			strategy: "invalid_strategy",
			expected: false,
		},
		{
			name:     "empty string should be invalid",
			strategy: "",
			expected: false,
		},
		{
			name:     "wait-all (with hyphen) should be invalid",
			strategy: "wait-all",
			expected: false,
		},
		{
			name:     "WAIT_ALL (uppercase) should be invalid",
			strategy: "WAIT_ALL",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateStrategy(tt.strategy)
			assert.Equal(
				t,
				tt.expected,
				result,
				"ValidateStrategy(%q) = %v, expected %v",
				tt.strategy,
				result,
				tt.expected,
			)
		})
	}
}
