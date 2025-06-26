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
