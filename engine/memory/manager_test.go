package memory

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/llm"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/instance/eviction"
	"github.com/compozy/compozy/engine/memory/privacy"
)

// Simplified manager test focusing on basic functionality

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

func TestManager_createEvictionPolicy(t *testing.T) {
	manager := &Manager{} // Minimal manager for testing

	t.Run("Should create priority policy with custom keywords from config", func(t *testing.T) {
		resourceCfg := &memcore.Resource{
			ID:        "test-memory",
			Type:      memcore.TokenBasedMemory,
			MaxTokens: 1000,
			EvictionPolicyConfig: &memcore.EvictionPolicyConfig{
				Type:             memcore.PriorityEviction,
				PriorityKeywords: []string{"security", "vulnerability", "breach", "attack"},
			},
			FlushingStrategy: &memcore.FlushingStrategyConfig{
				Type: memcore.SimpleFIFOFlushing,
			},
			Persistence: memcore.PersistenceConfig{
				Type: memcore.InMemoryPersistence,
				TTL:  "1h",
			},
		}

		policy := manager.createEvictionPolicy(t.Context(), resourceCfg)
		require.NotNil(t, policy)
		assert.Equal(t, "priority", policy.GetType())

		// Verify it's a priority policy with custom keywords
		priorityPolicy, ok := policy.(*eviction.PriorityEvictionPolicy)
		require.True(t, ok, "Should be a PriorityEvictionPolicy")

		// Test that it uses the custom keywords by checking eviction behavior
		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Normal user message"},
			{Role: llm.MessageRoleUser, Content: "Security vulnerability found"},
			{Role: llm.MessageRoleUser, Content: "Another normal message"},
			{Role: llm.MessageRoleUser, Content: "Potential attack detected"},
		}

		// Keep only 2 messages - should evict normal ones and keep security-related
		evicted := priorityPolicy.SelectMessagesToEvict(t.Context(), messages, 2)
		require.Len(t, evicted, 2)

		evictedContents := []string{evicted[0].Content, evicted[1].Content}
		assert.Contains(t, evictedContents, "Normal user message")
		assert.Contains(t, evictedContents, "Another normal message")
		assert.NotContains(t, evictedContents, "Security vulnerability found")
		assert.NotContains(t, evictedContents, "Potential attack detected")
	})

	t.Run("Should create priority policy with default keywords when no config provided", func(t *testing.T) {
		resourceCfg := &memcore.Resource{
			ID:        "test-memory-default",
			Type:      memcore.TokenBasedMemory,
			MaxTokens: 1000,
			EvictionPolicyConfig: &memcore.EvictionPolicyConfig{
				Type: memcore.PriorityEviction,
				// No PriorityKeywords configured - will use defaults
			},
			Persistence: memcore.PersistenceConfig{
				Type: memcore.InMemoryPersistence,
				TTL:  "1h",
			},
		}

		policy := manager.createEvictionPolicy(t.Context(), resourceCfg)
		require.NotNil(t, policy)
		assert.Equal(t, "priority", policy.GetType())

		// Should use default keywords
		priorityPolicy, ok := policy.(*eviction.PriorityEvictionPolicy)
		require.True(t, ok)

		// Test with default keywords
		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Normal message"},
			{Role: llm.MessageRoleUser, Content: "Critical error occurred"},
			{Role: llm.MessageRoleUser, Content: "Another normal message"},
			{Role: llm.MessageRoleUser, Content: "Important warning"},
		}

		evicted := priorityPolicy.SelectMessagesToEvict(t.Context(), messages, 2)
		require.Len(t, evicted, 2)

		// Should use default keywords (critical, error, important, warning)
		evictedContents := []string{evicted[0].Content, evicted[1].Content}
		assert.Contains(t, evictedContents, "Normal message")
		assert.Contains(t, evictedContents, "Another normal message")
		assert.NotContains(t, evictedContents, "Critical error occurred")
		assert.NotContains(t, evictedContents, "Important warning")
	})

	t.Run("Should create priority policy with default keywords when empty keywords provided", func(t *testing.T) {
		resourceCfg := &memcore.Resource{
			ID:        "test-memory-empty",
			Type:      memcore.TokenBasedMemory,
			MaxTokens: 1000,
			EvictionPolicyConfig: &memcore.EvictionPolicyConfig{
				Type:             memcore.PriorityEviction,
				PriorityKeywords: []string{}, // Explicitly empty
			},
			Persistence: memcore.PersistenceConfig{
				Type: memcore.InMemoryPersistence,
				TTL:  "1h",
			},
		}

		policy := manager.createEvictionPolicy(t.Context(), resourceCfg)
		require.NotNil(t, policy)
		assert.Equal(t, "priority", policy.GetType())

		// Should fallback to default keywords
		priorityPolicy, ok := policy.(*eviction.PriorityEvictionPolicy)
		require.True(t, ok)

		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Normal message"},
			{Role: llm.MessageRoleUser, Content: "Error occurred"},
		}

		evicted := priorityPolicy.SelectMessagesToEvict(t.Context(), messages, 1)
		require.Len(t, evicted, 1)
		assert.Equal(t, "Normal message", evicted[0].Content)
	})

	t.Run("Should create LRU policy when specified", func(t *testing.T) {
		resourceCfg := &memcore.Resource{
			ID:        "test-memory-lru",
			Type:      memcore.TokenBasedMemory,
			MaxTokens: 1000,
			EvictionPolicyConfig: &memcore.EvictionPolicyConfig{
				Type: memcore.LRUEviction,
			},
			Persistence: memcore.PersistenceConfig{
				Type: memcore.InMemoryPersistence,
				TTL:  "1h",
			},
		}

		policy := manager.createEvictionPolicy(t.Context(), resourceCfg)
		require.NotNil(t, policy)
		assert.Equal(t, "lru", policy.GetType())
	})

	t.Run("Should create FIFO policy when no eviction policy specified", func(t *testing.T) {
		resourceCfg := &memcore.Resource{
			ID:        "test-memory-fifo",
			Type:      memcore.TokenBasedMemory,
			MaxTokens: 1000,
			// No EvictionPolicyConfig specified - will use default
			Persistence: memcore.PersistenceConfig{
				Type: memcore.InMemoryPersistence,
				TTL:  "1h",
			},
		}

		policy := manager.createEvictionPolicy(t.Context(), resourceCfg)
		require.NotNil(t, policy)
		assert.Equal(t, "fifo", policy.GetType())
	})

	t.Run("Should create FIFO policy for unknown eviction policy", func(t *testing.T) {
		resourceCfg := &memcore.Resource{
			ID:        "test-memory-unknown",
			Type:      memcore.TokenBasedMemory,
			MaxTokens: 1000,
			EvictionPolicyConfig: &memcore.EvictionPolicyConfig{
				Type: "unknown-policy", // Unknown type
			},
			Persistence: memcore.PersistenceConfig{
				Type: memcore.InMemoryPersistence,
				TTL:  "1h",
			},
		}

		policy := manager.createEvictionPolicy(t.Context(), resourceCfg)
		require.NotNil(t, policy)
		assert.Equal(t, "fifo", policy.GetType())
	})
}
