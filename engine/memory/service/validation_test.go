package service

import (
	"testing"

	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/instance/strategies"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateFlushConfig(t *testing.T) {
	factory := strategies.NewStrategyFactory()

	tests := []struct {
		name      string
		config    *FlushConfig
		expectErr bool
		errMsg    string
	}{
		{
			name:      "nil config is valid",
			config:    nil,
			expectErr: false,
		},
		{
			name: "valid config with simple_fifo strategy",
			config: &FlushConfig{
				Strategy:  "simple_fifo",
				MaxKeys:   100,
				Threshold: 0.8,
			},
			expectErr: false,
		},
		{
			name: "valid config with lru strategy",
			config: &FlushConfig{
				Strategy:  "lru",
				MaxKeys:   50,
				Threshold: 0.5,
			},
			expectErr: false,
		},
		{
			name: "valid config with token_aware_lru strategy",
			config: &FlushConfig{
				Strategy: "token_aware_lru",
			},
			expectErr: false,
		},
		{
			name: "valid config with fifo alias",
			config: &FlushConfig{
				Strategy: "fifo",
			},
			expectErr: false,
		},
		{
			name: "invalid strategy",
			config: &FlushConfig{
				Strategy: "invalid_strategy",
			},
			expectErr: true,
			errMsg:    "invalid strategy 'invalid_strategy'",
		},
		{
			name: "empty strategy is valid",
			config: &FlushConfig{
				Strategy: "",
			},
			expectErr: false,
		},
		{
			name: "negative max keys",
			config: &FlushConfig{
				MaxKeys: -1,
			},
			expectErr: true,
			errMsg:    "max_keys must be non-negative",
		},
		{
			name: "max keys too large",
			config: &FlushConfig{
				MaxKeys: 10001,
			},
			expectErr: true,
			errMsg:    "max_keys too large",
		},
		{
			name: "threshold below 0",
			config: &FlushConfig{
				Threshold: -0.1,
			},
			expectErr: true,
			errMsg:    "threshold must be between 0 and 1",
		},
		{
			name: "threshold above 1",
			config: &FlushConfig{
				Threshold: 1.1,
			},
			expectErr: true,
			errMsg:    "threshold must be between 0 and 1",
		},
		{
			name: "obsolete strategies are rejected",
			config: &FlushConfig{
				Strategy: "summarize",
			},
			expectErr: true,
			errMsg:    "invalid strategy 'summarize'",
		},
		{
			name: "hybrid_summary is rejected",
			config: &FlushConfig{
				Strategy: "hybrid_summary",
			},
			expectErr: true,
			errMsg:    "invalid strategy 'hybrid_summary'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFlushConfig(tt.config, factory)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				// Verify it's a MemoryError with proper code
				memErr, ok := err.(*memcore.MemoryError)
				require.True(t, ok, "error should be MemoryError")
				assert.Equal(t, memcore.ErrCodeInvalidConfig, memErr.Code)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateFlushConfig_StrategyList(t *testing.T) {
	t.Run("Should return correct valid strategies in error message", func(t *testing.T) {
		factory := strategies.NewStrategyFactory()
		config := &FlushConfig{
			Strategy: "invalid",
		}

		err := ValidateFlushConfig(config, factory)
		require.Error(t, err)

		// Verify error message contains all valid strategies
		errMsg := err.Error()
		assert.Contains(t, errMsg, "simple_fifo")
		assert.Contains(t, errMsg, "fifo")
		assert.Contains(t, errMsg, "lru")
		assert.Contains(t, errMsg, "token_aware_lru")
		assert.Contains(t, errMsg, "must be one of:")

		// Verify MemoryError context
		memErr, ok := err.(*memcore.MemoryError)
		require.True(t, ok)

		// Check context contains valid strategies
		validStrategies, ok := memErr.Context["valid_strategies"].([]string)
		require.True(t, ok, "should have valid_strategies in context")
		assert.Contains(t, validStrategies, "simple_fifo")
		assert.Contains(t, validStrategies, "fifo")
		assert.Contains(t, validStrategies, "lru")
		assert.Contains(t, validStrategies, "token_aware_lru")
	})
}
