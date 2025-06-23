package memory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Simplified manager test focusing on basic functionality

func TestDefaultComponentCacheConfig(t *testing.T) {
	t.Run("Should return default configuration", func(t *testing.T) {
		config := DefaultComponentCacheConfig()
		assert.Equal(t, int64(50<<20), config.MaxCost)  // 50 MB
		assert.Equal(t, int64(1e6), config.NumCounters) // 1 million
		assert.Equal(t, int64(64), config.BufferItems)  // 64 items
	})
}

func TestNewCacheableTiktokenCounter(t *testing.T) {
	t.Run("Should create cacheable token counter", func(t *testing.T) {
		counter, err := NewCacheableTiktokenCounter("gpt-4")
		require.NoError(t, err)
		assert.NotNil(t, counter)
		assert.Equal(t, "gpt-4", counter.model)
		assert.NotNil(t, counter.TiktokenCounter)
	})
	t.Run("Should return cache key", func(t *testing.T) {
		counter, err := NewCacheableTiktokenCounter("gpt-3.5-turbo")
		require.NoError(t, err)
		key := counter.GetCacheKey()
		assert.Equal(t, "token-counter:gpt-3.5-turbo", key)
	})
	t.Run("Should estimate cost", func(t *testing.T) {
		counter, err := NewCacheableTiktokenCounter("gpt-4")
		require.NoError(t, err)
		cost := counter.EstimateCost()
		assert.Equal(t, int64(1<<20), cost) // 1 MB
	})
}

func TestNewManager_Validation(t *testing.T) {
	t.Run("Should fail when required options are nil", func(t *testing.T) {
		opts := &ManagerOptions{}
		manager, err := NewManager(opts)
		require.Error(t, err)
		assert.Nil(t, manager)
		assert.Contains(t, err.Error(), "resource registry cannot be nil")
	})
}

// Manager GetInstance tests require complex mocking, skipping for basic coverage
