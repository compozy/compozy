package strategies

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/memory/core"
)

func TestStrategyFactory_CreateStrategy(t *testing.T) {
	factory := NewStrategyFactory()

	t.Run("Should create FIFO strategy with default config", func(t *testing.T) {
		config := &core.FlushingStrategyConfig{
			Type: core.SimpleFIFOFlushing,
		}
		strategy, err := factory.CreateStrategy(config, nil)
		require.NoError(t, err)
		require.NotNil(t, strategy)
		assert.Equal(t, core.SimpleFIFOFlushing, strategy.GetType())
	})

	t.Run("Should create LRU strategy", func(t *testing.T) {
		config := &core.FlushingStrategyConfig{
			Type: core.LRUFlushing,
		}
		strategy, err := factory.CreateStrategy(config, nil)
		require.NoError(t, err)
		require.NotNil(t, strategy)
		assert.Equal(t, core.LRUFlushing, strategy.GetType())
	})

	t.Run("Should create Token-Aware LRU strategy", func(t *testing.T) {
		config := &core.FlushingStrategyConfig{
			Type: core.TokenAwareLRUFlushing,
		}
		strategy, err := factory.CreateStrategy(config, nil)
		require.NoError(t, err)
		require.NotNil(t, strategy)
		assert.Equal(t, core.TokenAwareLRUFlushing, strategy.GetType())
	})

	t.Run("Should return error for unknown strategy type", func(t *testing.T) {
		config := &core.FlushingStrategyConfig{
			Type: core.FlushingStrategyType("unknown"),
		}
		strategy, err := factory.CreateStrategy(config, nil)
		assert.Error(t, err)
		assert.Nil(t, strategy)
		assert.Contains(t, err.Error(), "unknown flush strategy type")
	})

	t.Run("Should create default FIFO strategy when config is nil", func(t *testing.T) {
		strategy, err := factory.CreateStrategy(nil, nil)
		require.NoError(t, err)
		require.NotNil(t, strategy)
		assert.Equal(t, core.SimpleFIFOFlushing, strategy.GetType())
	})

	t.Run("Should NOT create priority-based flush strategy", func(t *testing.T) {
		// Priority-based flush strategy was removed - only eviction policies handle priority
		availableStrategies := factory.GetAvailableStrategies()
		for _, strategyType := range availableStrategies {
			assert.NotEqual(t, "priority_based", string(strategyType))
		}

		// Verify that priority is not supported
		assert.False(t, factory.IsStrategySupported("priority_based"))
	})

	t.Run("Should use strategy options for configuration", func(t *testing.T) {
		config := &core.FlushingStrategyConfig{
			Type:               core.SimpleFIFOFlushing,
			SummarizeThreshold: 0.9,
		}
		opts := &StrategyOptions{
			DefaultThreshold: 0.7,
			MaxTokens:        2000,
		}
		strategy, err := factory.CreateStrategy(config, opts)
		require.NoError(t, err)
		require.NotNil(t, strategy)
		assert.Equal(t, core.SimpleFIFOFlushing, strategy.GetType())
	})
}

func TestStrategyFactory_ValidateStrategyConfig(t *testing.T) {
	factory := NewStrategyFactory()

	t.Run("Should validate valid FIFO config", func(t *testing.T) {
		config := &core.FlushingStrategyConfig{
			Type:               core.SimpleFIFOFlushing,
			SummarizeThreshold: 0.8,
		}
		err := factory.ValidateStrategyConfig(config)
		assert.NoError(t, err)
	})

	t.Run("Should validate valid LRU config", func(t *testing.T) {
		config := &core.FlushingStrategyConfig{
			Type: core.LRUFlushing,
		}
		err := factory.ValidateStrategyConfig(config)
		assert.NoError(t, err)
	})

	t.Run("Should return error for nil config", func(t *testing.T) {
		err := factory.ValidateStrategyConfig(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("Should return error for invalid threshold", func(t *testing.T) {
		config := &core.FlushingStrategyConfig{
			Type:               core.SimpleFIFOFlushing,
			SummarizeThreshold: 1.5, // Invalid - greater than 1
		}
		err := factory.ValidateStrategyConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "summarize threshold must be between 0 and 1")
	})

	t.Run("Should return error for unsupported strategy type", func(t *testing.T) {
		config := &core.FlushingStrategyConfig{
			Type: core.FlushingStrategyType("priority_based"), // No longer supported
		}
		err := factory.ValidateStrategyConfig(config)
		assert.Error(t, err)
		// The validation fails because priority_based is not in IsValid() list
		assert.Contains(t, err.Error(), "invalid strategy type")
	})
}

func TestStrategyFactory_GetAvailableStrategies(t *testing.T) {
	factory := NewStrategyFactory()
	strategies := factory.GetAvailableStrategies()

	t.Run("Should include all registered strategies", func(t *testing.T) {
		assert.Contains(t, strategies, core.SimpleFIFOFlushing)
		assert.Contains(t, strategies, core.LRUFlushing)
		assert.Contains(t, strategies, core.TokenAwareLRUFlushing)
	})

	t.Run("Should NOT include priority-based strategy", func(t *testing.T) {
		for _, strategy := range strategies {
			assert.NotEqual(t, "priority_based", string(strategy))
		}
	})

	t.Run("Should have at least 3 strategies", func(t *testing.T) {
		assert.GreaterOrEqual(t, len(strategies), 3)
	})
}

func TestGetDefaultStrategyOptions(t *testing.T) {
	opts := GetDefaultStrategyOptions()

	t.Run("Should provide reasonable defaults", func(t *testing.T) {
		assert.Equal(t, 1000, opts.CacheSize)
		assert.Equal(t, 4000, opts.MaxTokens)
		assert.Equal(t, 0.8, opts.DefaultThreshold)
	})

	t.Run("Should provide LRU-specific defaults", func(t *testing.T) {
		assert.Equal(t, 0.5, opts.LRUTargetCapacityPercent)
		assert.Equal(t, 0.25, opts.LRUMinFlushPercent)
	})

	t.Run("Should provide Token-LRU-specific defaults", func(t *testing.T) {
		assert.Equal(t, 0.5, opts.TokenLRUTargetCapacityPercent)
	})

	t.Run("Should provide Priority-specific defaults for backward compatibility", func(t *testing.T) {
		// Even though priority flush strategy was removed, the options remain
		// for potential future use or backward compatibility
		assert.Equal(t, 0.6, opts.PriorityTargetCapacityPercent)
		assert.Equal(t, 0.75, opts.PriorityConservativePercent)
		assert.Equal(t, 0.8, opts.PriorityRecentThreshold)
		assert.Equal(t, 0.33, opts.PriorityMaxFlushRatio)
	})
}

func TestStrategyFactory_CreateDefaultStrategy(t *testing.T) {
	factory := NewStrategyFactory()
	strategy, err := factory.CreateDefaultStrategy()

	t.Run("Should create default FIFO strategy without error", func(t *testing.T) {
		require.NoError(t, err)
		require.NotNil(t, strategy)
		assert.Equal(t, core.SimpleFIFOFlushing, strategy.GetType())
	})
}

func TestStrategyFactory_ValidateStrategyType(t *testing.T) {
	factory := NewStrategyFactory()

	tests := []struct {
		name        string
		strategy    string
		expectError bool
		errMsg      string
	}{
		{
			name:        "valid simple_fifo strategy",
			strategy:    "simple_fifo",
			expectError: false,
		},
		{
			name:        "valid fifo alias",
			strategy:    "fifo",
			expectError: false,
		},
		{
			name:        "valid lru strategy",
			strategy:    "lru",
			expectError: false,
		},
		{
			name:        "valid token_aware_lru strategy",
			strategy:    "token_aware_lru",
			expectError: false,
		},
		{
			name:        "empty string is valid (uses default)",
			strategy:    "",
			expectError: false,
		},
		{
			name:        "invalid strategy",
			strategy:    "invalid_strategy",
			expectError: true,
			errMsg:      "invalid strategy type: invalid_strategy",
		},
		{
			name:        "obsolete summarize strategy",
			strategy:    "summarize",
			expectError: true,
			errMsg:      "invalid strategy type: summarize",
		},
		{
			name:        "obsolete hybrid strategy",
			strategy:    "hybrid",
			expectError: true,
			errMsg:      "invalid strategy type: hybrid",
		},
		{
			name:        "obsolete hybrid_summary strategy",
			strategy:    "hybrid_summary",
			expectError: true,
			errMsg:      "strategy type hybrid_summary is not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := factory.ValidateStrategyType(tt.strategy)
			if tt.expectError {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStrategyFactory_GetSupportedStrategies(t *testing.T) {
	factory := NewStrategyFactory()
	strategies := factory.GetSupportedStrategies()

	t.Run("Should return all supported strategies as strings", func(t *testing.T) {
		// Should include all valid strategies
		assert.Contains(t, strategies, "simple_fifo")
		assert.Contains(t, strategies, "fifo")
		assert.Contains(t, strategies, "lru")
		assert.Contains(t, strategies, "token_aware_lru")

		// Should have exactly 4 strategies (including fifo alias)
		assert.Len(t, strategies, 4)
	})

	t.Run("Should NOT include obsolete strategies", func(t *testing.T) {
		assert.NotContains(t, strategies, "summarize")
		assert.NotContains(t, strategies, "trim")
		assert.NotContains(t, strategies, "archive")
		assert.NotContains(t, strategies, "hybrid")
		assert.NotContains(t, strategies, "hybrid_summary")
		assert.NotContains(t, strategies, "priority_based")
	})
}

func TestStrategyFactory_IsValidStrategy(t *testing.T) {
	factory := NewStrategyFactory()

	tests := []struct {
		name     string
		strategy string
		expected bool
	}{
		{"simple_fifo is valid", "simple_fifo", true},
		{"fifo alias is valid", "fifo", true},
		{"lru is valid", "lru", true},
		{"token_aware_lru is valid", "token_aware_lru", true},
		{"empty string is valid", "", true},
		{"invalid_strategy is invalid", "invalid_strategy", false},
		{"summarize is invalid", "summarize", false},
		{"hybrid is invalid", "hybrid", false},
		{"hybrid_summary is invalid", "hybrid_summary", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := factory.IsValidStrategy(tt.strategy)
			assert.Equal(t, tt.expected, result)
		})
	}
}
