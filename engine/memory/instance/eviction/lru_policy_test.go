package eviction

import (
	"testing"
	"time"

	"github.com/compozy/compozy/engine/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLRUEvictionPolicy_NewLRUEvictionPolicy(t *testing.T) {
	t.Run("Should create LRU eviction policy", func(t *testing.T) {
		policy := NewLRUEvictionPolicy()
		require.NotNil(t, policy)
		assert.Equal(t, "lru", policy.GetType())
	})
}

func TestLRUEvictionPolicy_SelectMessagesToEvict(t *testing.T) {
	policy := NewLRUEvictionPolicy()

	t.Run("Should return nil when no eviction needed", func(t *testing.T) {
		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Message 1"},
			{Role: llm.MessageRoleAssistant, Content: "Message 2"},
		}
		evicted := policy.SelectMessagesToEvict(messages, 5)
		assert.Nil(t, evicted)
	})

	t.Run("Should evict never-accessed messages first", func(t *testing.T) {
		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Never accessed 1"},
			{Role: llm.MessageRoleAssistant, Content: "Accessed message"},
			{Role: llm.MessageRoleUser, Content: "Never accessed 2"},
		}
		// Update access for middle message only
		policy.UpdateAccess(messages[1])
		// Keep only 1 message
		evicted := policy.SelectMessagesToEvict(messages, 1)
		require.Len(t, evicted, 2)
		// Should evict the never-accessed messages
		assert.Equal(t, "Never accessed 1", evicted[0].Content)
		assert.Equal(t, "Never accessed 2", evicted[1].Content)
	})

	t.Run("Should evict least recently used messages", func(t *testing.T) {
		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Oldest access"},
			{Role: llm.MessageRoleAssistant, Content: "Recent access"},
			{Role: llm.MessageRoleUser, Content: "Middle access"},
			{Role: llm.MessageRoleAssistant, Content: "Most recent"},
		}
		// Update access times with delays
		policy.UpdateAccess(messages[0])
		time.Sleep(10 * time.Millisecond)
		policy.UpdateAccess(messages[2])
		time.Sleep(10 * time.Millisecond)
		policy.UpdateAccess(messages[1])
		time.Sleep(10 * time.Millisecond)
		policy.UpdateAccess(messages[3])
		// Keep only 2 messages
		evicted := policy.SelectMessagesToEvict(messages, 2)
		require.Len(t, evicted, 2)
		// Should evict oldest and middle access
		assert.Equal(t, "Oldest access", evicted[0].Content)
		assert.Equal(t, "Middle access", evicted[1].Content)
	})

	t.Run("Should handle batch access updates", func(t *testing.T) {
		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Message 1"},
			{Role: llm.MessageRoleAssistant, Content: "Message 2"},
			{Role: llm.MessageRoleUser, Content: "Message 3"},
		}
		// Update first two messages in batch
		policy.UpdateAccessBatch(messages[:2])
		// Keep only 1 message
		evicted := policy.SelectMessagesToEvict(messages, 1)
		require.Len(t, evicted, 2)
		// Message 3 should be evicted first (never accessed)
		assert.Equal(t, "Message 3", evicted[0].Content)
		// Then one of the batch-updated messages
		assert.Contains(t, []string{"Message 1", "Message 2"}, evicted[1].Content)
	})

	t.Run("Should maintain FIFO order for never-accessed messages", func(t *testing.T) {
		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "First"},
			{Role: llm.MessageRoleAssistant, Content: "Second"},
			{Role: llm.MessageRoleUser, Content: "Third"},
			{Role: llm.MessageRoleAssistant, Content: "Fourth"},
		}
		// Don't update any access times
		// Keep only 2 messages
		evicted := policy.SelectMessagesToEvict(messages, 2)
		require.Len(t, evicted, 2)
		// Should evict in FIFO order when no access times
		assert.Equal(t, "First", evicted[0].Content)
		assert.Equal(t, "Second", evicted[1].Content)
	})

	t.Run("Should handle clearing access history", func(t *testing.T) {
		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Message 1"},
			{Role: llm.MessageRoleAssistant, Content: "Message 2"},
		}
		// Update access times
		policy.UpdateAccessBatch(messages)
		// Clear history
		policy.ClearAccessHistory()
		// Both messages should now be treated as never accessed
		evicted := policy.SelectMessagesToEvict(messages, 1)
		require.Len(t, evicted, 1)
		// Should evict first message (FIFO for never accessed)
		assert.Equal(t, "Message 1", evicted[0].Content)
	})

	t.Run("Should handle empty message list", func(t *testing.T) {
		evicted := policy.SelectMessagesToEvict([]llm.Message{}, 0)
		assert.Nil(t, evicted)
	})

	t.Run("Should handle negative target count", func(t *testing.T) {
		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Message 1"},
		}
		evicted := policy.SelectMessagesToEvict(messages, -1)
		assert.Nil(t, evicted)
	})
}

func TestLRUEvictionPolicy_UpdateAccess(t *testing.T) {
	t.Run("Should update access time for message", func(t *testing.T) {
		policy := NewLRUEvictionPolicy()
		msg := llm.Message{Role: llm.MessageRoleUser, Content: "Test message"}
		// Update access
		policy.UpdateAccess(msg)
		// Message should not be evicted when another hasn't been accessed
		messages := []llm.Message{
			msg,
			{Role: llm.MessageRoleAssistant, Content: "Never accessed"},
		}
		evicted := policy.SelectMessagesToEvict(messages, 1)
		require.Len(t, evicted, 1)
		assert.Equal(t, "Never accessed", evicted[0].Content)
	})
}

func TestLRUEvictionPolicy_GetType(t *testing.T) {
	t.Run("Should return correct policy type", func(t *testing.T) {
		policy := NewLRUEvictionPolicy()
		assert.Equal(t, "lru", policy.GetType())
	})
}

func TestLRUEvictionPolicy_ConcurrentAccess(t *testing.T) {
	t.Run("Should handle concurrent access updates safely", func(t *testing.T) {
		policy := NewLRUEvictionPolicy()
		messages := make([]llm.Message, 100)
		for i := 0; i < 100; i++ {
			messages[i] = llm.Message{
				Role:    llm.MessageRoleUser,
				Content: string(rune('A' + (i % 26))),
			}
		}
		// Concurrent access updates
		done := make(chan bool, 3)
		// Goroutine 1: Update first 50 messages
		go func() {
			for i := 0; i < 50; i++ {
				policy.UpdateAccess(messages[i])
			}
			done <- true
		}()
		// Goroutine 2: Update last 50 messages
		go func() {
			for i := 50; i < 100; i++ {
				policy.UpdateAccess(messages[i])
			}
			done <- true
		}()
		// Goroutine 3: Perform evictions
		go func() {
			for i := 0; i < 10; i++ {
				policy.SelectMessagesToEvict(messages, 50)
			}
			done <- true
		}()
		// Wait for all goroutines
		for i := 0; i < 3; i++ {
			<-done
		}
		// Should complete without panic
		assert.True(t, true)
	})
}
