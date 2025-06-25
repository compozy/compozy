package memory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/llm"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/instance/eviction"
)

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

		policy := manager.createEvictionPolicy(resourceCfg)
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
		evicted := priorityPolicy.SelectMessagesToEvict(messages, 2)
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

		policy := manager.createEvictionPolicy(resourceCfg)
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

		evicted := priorityPolicy.SelectMessagesToEvict(messages, 2)
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

		policy := manager.createEvictionPolicy(resourceCfg)
		require.NotNil(t, policy)
		assert.Equal(t, "priority", policy.GetType())

		// Should fallback to default keywords
		priorityPolicy, ok := policy.(*eviction.PriorityEvictionPolicy)
		require.True(t, ok)

		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Normal message"},
			{Role: llm.MessageRoleUser, Content: "Error occurred"},
		}

		evicted := priorityPolicy.SelectMessagesToEvict(messages, 1)
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

		policy := manager.createEvictionPolicy(resourceCfg)
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

		policy := manager.createEvictionPolicy(resourceCfg)
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

		policy := manager.createEvictionPolicy(resourceCfg)
		require.NotNil(t, policy)
		assert.Equal(t, "fifo", policy.GetType())
	})
}
