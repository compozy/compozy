package memory

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/memory/privacy"
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

// TestManager_ResilienceConfig tests resilience configuration handling
func TestManager_ResilienceConfig(t *testing.T) {
	t.Run("Should validate resilience config", func(t *testing.T) {
		tests := []struct {
			name    string
			config  *privacy.ResilienceConfig
			wantErr bool
		}{
			{
				name: "valid config",
				config: &privacy.ResilienceConfig{
					TimeoutDuration:             100 * time.Millisecond,
					ErrorPercentThresholdToOpen: 50,
					MinimumRequestToOpen:        10,
					WaitDurationInOpenState:     5 * time.Second,
					RetryTimes:                  3,
					RetryWaitBase:               50 * time.Millisecond,
				},
				wantErr: false,
			},
			{
				name:    "nil config",
				config:  nil,
				wantErr: true,
			},
			{
				name: "invalid timeout",
				config: &privacy.ResilienceConfig{
					TimeoutDuration:             0,
					ErrorPercentThresholdToOpen: 50,
					MinimumRequestToOpen:        10,
					WaitDurationInOpenState:     5 * time.Second,
					RetryTimes:                  3,
					RetryWaitBase:               50 * time.Millisecond,
				},
				wantErr: true,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := privacy.ValidateConfig(tt.config)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
	t.Run("Should use default resilience config", func(t *testing.T) {
		config := privacy.DefaultResilienceConfig()
		require.NotNil(t, config)
		assert.Equal(t, 100*time.Millisecond, config.TimeoutDuration)
		assert.Equal(t, 50, config.ErrorPercentThresholdToOpen)
		assert.Equal(t, 10, config.MinimumRequestToOpen)
		assert.Equal(t, 5*time.Second, config.WaitDurationInOpenState)
		assert.Equal(t, 3, config.RetryTimes)
		assert.Equal(t, 50*time.Millisecond, config.RetryWaitBase)
	})
}
