package eviction

import (
	"testing"

	"github.com/compozy/compozy/engine/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFIFOEvictionPolicy_NewFIFOEvictionPolicy(t *testing.T) {
	t.Run("Should create FIFO eviction policy", func(t *testing.T) {
		policy := NewFIFOEvictionPolicy()
		require.NotNil(t, policy)
		assert.Equal(t, "fifo", policy.GetType())
	})
}

func TestFIFOEvictionPolicy_SelectMessagesToEvict(t *testing.T) {
	policy := NewFIFOEvictionPolicy()

	t.Run("Should return nil when no eviction needed", func(t *testing.T) {
		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Message 1"},
			{Role: llm.MessageRoleAssistant, Content: "Message 2"},
		}
		evicted := policy.SelectMessagesToEvict(messages, 5)
		assert.Nil(t, evicted)
	})

	t.Run("Should return nil when target equals message count", func(t *testing.T) {
		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Message 1"},
			{Role: llm.MessageRoleAssistant, Content: "Message 2"},
		}
		evicted := policy.SelectMessagesToEvict(messages, 2)
		assert.Nil(t, evicted)
	})

	t.Run("Should evict oldest messages first", func(t *testing.T) {
		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Oldest message"},
			{Role: llm.MessageRoleAssistant, Content: "Second message"},
			{Role: llm.MessageRoleUser, Content: "Third message"},
			{Role: llm.MessageRoleAssistant, Content: "Newest message"},
		}
		// Keep only 2 messages, evict 2 oldest
		evicted := policy.SelectMessagesToEvict(messages, 2)
		require.Len(t, evicted, 2)
		assert.Equal(t, "Oldest message", evicted[0].Content)
		assert.Equal(t, "Second message", evicted[1].Content)
	})

	t.Run("Should handle evicting all messages", func(t *testing.T) {
		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Message 1"},
			{Role: llm.MessageRoleAssistant, Content: "Message 2"},
			{Role: llm.MessageRoleUser, Content: "Message 3"},
		}
		// Target count of 0 should evict all
		evicted := policy.SelectMessagesToEvict(messages, 0)
		require.Len(t, evicted, 3)
		assert.Equal(t, messages, evicted)
	})

	t.Run("Should handle negative target count", func(t *testing.T) {
		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Message 1"},
		}
		evicted := policy.SelectMessagesToEvict(messages, -1)
		assert.Nil(t, evicted)
	})

	t.Run("Should handle empty message list", func(t *testing.T) {
		evicted := policy.SelectMessagesToEvict([]llm.Message{}, 0)
		assert.Nil(t, evicted)
	})

	t.Run("Should evict correct number of messages", func(t *testing.T) {
		messages := make([]llm.Message, 10)
		for i := 0; i < 10; i++ {
			messages[i] = llm.Message{
				Role:    llm.MessageRoleUser,
				Content: string(rune('A' + i)),
			}
		}
		// Keep 7 messages, evict 3
		evicted := policy.SelectMessagesToEvict(messages, 7)
		require.Len(t, evicted, 3)
		// Should evict A, B, C (oldest)
		assert.Equal(t, "A", evicted[0].Content)
		assert.Equal(t, "B", evicted[1].Content)
		assert.Equal(t, "C", evicted[2].Content)
	})
}

func TestFIFOEvictionPolicy_GetType(t *testing.T) {
	t.Run("Should return correct policy type", func(t *testing.T) {
		policy := NewFIFOEvictionPolicy()
		assert.Equal(t, "fifo", policy.GetType())
	})
}
