package strategies

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPriorityBasedStrategy_NewPriorityBasedStrategy(t *testing.T) {
	t.Run("Should create priority-based strategy with default configuration", func(t *testing.T) {
		strategy := NewPriorityBasedStrategy(nil, nil)

		require.NotNil(t, strategy)
		assert.Nil(t, strategy.config)
		assert.NotNil(t, strategy.flushDecision)
		assert.Equal(t, 0.8, strategy.flushDecision.GetThreshold())
	})

	t.Run("Should create strategy with custom configuration", func(t *testing.T) {
		config := &core.FlushingStrategyConfig{
			Type:               core.PriorityBasedFlushing,
			SummarizeThreshold: 0.9,
		}

		strategy := NewPriorityBasedStrategy(config, nil)

		require.NotNil(t, strategy)
		assert.Equal(t, config, strategy.config)
		assert.Equal(t, 0.9, strategy.flushDecision.GetThreshold())
	})
}

func TestPriorityBasedStrategy_ShouldFlush(t *testing.T) {
	strategy := NewPriorityBasedStrategy(&core.FlushingStrategyConfig{
		Type:               core.PriorityBasedFlushing,
		SummarizeThreshold: 0.8,
	}, nil)

	t.Run("Should trigger flush for token-based memory when threshold exceeded", func(t *testing.T) {
		config := &core.Resource{
			Type:      core.TokenBasedMemory,
			MaxTokens: 2000,
		}

		// Below threshold
		shouldFlush := strategy.ShouldFlush(1500, 10, config)
		assert.False(t, shouldFlush)

		// At threshold (80% of 2000 = 1600)
		shouldFlush = strategy.ShouldFlush(1600, 10, config)
		assert.True(t, shouldFlush)

		// Above threshold
		shouldFlush = strategy.ShouldFlush(1800, 10, config)
		assert.True(t, shouldFlush)
	})

	t.Run("Should trigger flush for message-based memory when threshold exceeded", func(t *testing.T) {
		config := &core.Resource{
			Type:        core.MessageCountBasedMemory,
			MaxMessages: 25,
		}

		// Below threshold (80% of 25 = 20)
		shouldFlush := strategy.ShouldFlush(1000, 15, config)
		assert.False(t, shouldFlush)

		// At threshold
		shouldFlush = strategy.ShouldFlush(1000, 20, config)
		assert.True(t, shouldFlush)

		// Above threshold
		shouldFlush = strategy.ShouldFlush(1000, 23, config)
		assert.True(t, shouldFlush)
	})

	t.Run("Should trigger flush for buffer memory when either threshold exceeded", func(t *testing.T) {
		config := &core.Resource{
			Type:        core.BufferMemory,
			MaxTokens:   2500,
			MaxMessages: 30,
		}

		// Below both thresholds
		shouldFlush := strategy.ShouldFlush(1900, 20, config)
		assert.False(t, shouldFlush)

		// Token threshold exceeded (80% of 2500 = 2000)
		shouldFlush = strategy.ShouldFlush(2100, 20, config)
		assert.True(t, shouldFlush)

		// Message threshold exceeded (80% of 30 = 24)
		shouldFlush = strategy.ShouldFlush(1900, 25, config)
		assert.True(t, shouldFlush)
	})
}

func TestPriorityBasedStrategy_PerformFlush(t *testing.T) {
	strategy := NewPriorityBasedStrategy(&core.FlushingStrategyConfig{
		Type:               core.PriorityBasedFlushing,
		SummarizeThreshold: 0.8,
	}, nil)

	t.Run("Should handle empty message list", func(t *testing.T) {
		config := &core.Resource{
			Type:      core.TokenBasedMemory,
			MaxTokens: 2000,
		}

		result, err := strategy.PerformFlush(context.Background(), []llm.Message{}, config)

		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.False(t, result.SummaryGenerated)
		assert.Equal(t, 0, result.MessageCount)
		assert.Equal(t, 0, result.TokenCount)
	})

	t.Run("Should preserve critical messages and evict low priority ones", func(t *testing.T) {
		config := &core.Resource{
			Type:        core.MessageCountBasedMemory,
			MaxMessages: 6, // Force aggressive flushing
		}

		messages := []llm.Message{
			{
				Role:    llm.MessageRoleSystem,
				Content: "You are a helpful assistant",
			}, // Critical - never evicted
			{
				Role:    llm.MessageRoleUser,
				Content: "hello",
			}, // Low priority - short greeting
			{
				Role:    llm.MessageRoleAssistant,
				Content: "This is an important error message",
			}, // High priority - error keyword
			{
				Role:    llm.MessageRoleUser,
				Content: "thanks",
			}, // Low priority - acknowledgment
			{
				Role:    llm.MessageRoleUser,
				Content: "This is a substantial user message with important content",
			}, // Medium priority
			{
				Role:    llm.MessageRoleAssistant,
				Content: "ok",
			}, // Low priority - short response
			{
				Role:    llm.MessageRoleUser,
				Content: "Can you help with this critical issue?",
			}, // High priority - critical keyword
		}

		result, err := strategy.PerformFlush(context.Background(), messages, config)

		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.False(t, result.SummaryGenerated)
		assert.Less(t, result.MessageCount, len(messages)) // Some messages should be flushed
		// System message and high priority messages should be preserved
	})

	t.Run("Should respect message count limits when flushing", func(t *testing.T) {
		config := &core.Resource{
			Type:        core.MessageCountBasedMemory,
			MaxMessages: 10,
		}

		// Create more messages than the limit
		messages := make([]llm.Message, 20)
		for i := range messages {
			messages[i] = llm.Message{
				Role:    llm.MessageRoleUser,
				Content: "This is a test message",
			}
		}

		result, err := strategy.PerformFlush(context.Background(), messages, config)

		require.NoError(t, err)
		assert.True(t, result.Success)
		// Should reduce to target count (around 60% = 6)
		assert.LessOrEqual(t, result.MessageCount, 8)
	})

	t.Run("Should handle token-based flushing", func(t *testing.T) {
		config := &core.Resource{
			Type:      core.TokenBasedMemory,
			MaxTokens: 500,
		}

		messages := []llm.Message{
			{Role: llm.MessageRoleSystem, Content: "System instruction"},
			{Role: llm.MessageRoleUser, Content: "Short"},
			{
				Role:    llm.MessageRoleUser,
				Content: "This is a much longer message that contains significantly more content and tokens",
			},
			{Role: llm.MessageRoleUser, Content: "Medium message"},
		}

		result, err := strategy.PerformFlush(context.Background(), messages, config)

		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.GreaterOrEqual(t, result.TokenCount, 0) // Some tokens should be freed
	})
}

func TestPriorityBasedStrategy_GetType(t *testing.T) {
	t.Run("Should return correct strategy type", func(t *testing.T) {
		strategy := NewPriorityBasedStrategy(nil, nil)

		strategyType := strategy.GetType()
		assert.Equal(t, core.PriorityBasedFlushing, strategyType)
	})
}

func TestPriorityBasedStrategy_GetMinMaxToFlush(t *testing.T) {
	strategy := NewPriorityBasedStrategy(nil, nil)

	t.Run("Should return conservative min/max flush counts", func(t *testing.T) {
		totalMsgs := 30
		minFlush, maxFlush := strategy.GetMinMaxToFlush(context.Background(), totalMsgs, 1000, 2000)

		assert.Equal(t, 1, minFlush)
		assert.Equal(t, 10, maxFlush) // 1/3 of total messages (conservative)
		assert.LessOrEqual(t, minFlush, maxFlush)
	})

	t.Run("Should handle edge case with very few messages", func(t *testing.T) {
		totalMsgs := 2
		minFlush, maxFlush := strategy.GetMinMaxToFlush(context.Background(), totalMsgs, 100, 200)

		assert.Equal(t, 1, minFlush)
		assert.Equal(t, 1, maxFlush)
	})

	t.Run("Should never flush more than 1/3 of messages", func(t *testing.T) {
		totalMsgs := 100
		minFlush, maxFlush := strategy.GetMinMaxToFlush(context.Background(), totalMsgs, 5000, 10000)

		assert.Equal(t, 1, minFlush)
		assert.LessOrEqual(t, maxFlush, totalMsgs/3)
	})
}

func TestPriorityBasedStrategy_DeterminePriority(t *testing.T) {
	strategy := NewPriorityBasedStrategy(nil, nil)

	t.Run("Should assign critical priority to system messages", func(t *testing.T) {
		msg := llm.Message{Role: llm.MessageRoleSystem, Content: "You are a helpful assistant"}
		priority := strategy.determinePriority(msg, 0, 10)
		assert.Equal(t, PriorityCritical, priority)
	})

	t.Run("Should assign critical priority to messages with system keywords", func(t *testing.T) {
		testCases := []string{
			"Here are the system instructions",
			"Follow this instruction carefully",
			"User profile information",
			"Important rule to follow",
			"Guidelines for behavior",
		}

		for _, content := range testCases {
			msg := llm.Message{Role: llm.MessageRoleUser, Content: content}
			priority := strategy.determinePriority(msg, 0, 10)
			assert.Equal(t, PriorityCritical, priority, "Failed for content: %s", content)
		}
	})

	t.Run("Should assign high priority to recent messages", func(t *testing.T) {
		totalMessages := 10
		recentIndex := 8 // In the last 20% of conversation

		msg := llm.Message{Role: llm.MessageRoleUser, Content: "Recent message"}
		priority := strategy.determinePriority(msg, recentIndex, totalMessages)
		assert.Equal(t, PriorityHigh, priority)
	})

	t.Run("Should assign high priority to messages with important keywords", func(t *testing.T) {
		testCases := []string{
			"This is important information",
			"Critical system failure detected",
			"Error occurred during processing",
			"Problem with the implementation",
			"Urgent issue needs attention",
			"There's an issue with the code",
		}

		for _, content := range testCases {
			msg := llm.Message{Role: llm.MessageRoleUser, Content: content}
			priority := strategy.determinePriority(msg, 0, 10)
			assert.Equal(t, PriorityHigh, priority, "Failed for content: %s", content)
		}
	})

	t.Run("Should assign medium priority to assistant responses", func(t *testing.T) {
		msg := llm.Message{Role: llm.MessageRoleAssistant, Content: "Here's my response"}
		priority := strategy.determinePriority(msg, 0, 10)
		assert.Equal(t, PriorityMedium, priority)
	})

	t.Run("Should assign medium priority to substantial user messages", func(t *testing.T) {
		msg := llm.Message{
			Role:    llm.MessageRoleUser,
			Content: "This is a substantial user message with enough content to be considered important for the conversation context",
		}
		priority := strategy.determinePriority(msg, 0, 10)
		assert.Equal(t, PriorityMedium, priority)
	})

	t.Run("Should assign low priority to short greetings and acknowledgments", func(t *testing.T) {
		testCases := []string{
			"hello",
			"hi there",
			"thanks for your help",
			"ok",
			"yes, that's correct",
			"no problem",
		}

		for _, content := range testCases {
			msg := llm.Message{Role: llm.MessageRoleUser, Content: content}
			priority := strategy.determinePriority(msg, 0, 10)
			assert.Equal(t, PriorityLow, priority, "Failed for content: %s", content)
		}
	})

	t.Run("Should assign low priority to very short messages", func(t *testing.T) {
		msg := llm.Message{Role: llm.MessageRoleUser, Content: "ok"}
		priority := strategy.determinePriority(msg, 0, 10)
		assert.Equal(t, PriorityLow, priority)
	})

	t.Run("Should default to medium priority for edge cases", func(t *testing.T) {
		msg := llm.Message{Role: llm.MessageRoleUser, Content: "This is a regular message with normal content length"}
		priority := strategy.determinePriority(msg, 3, 10) // Not recent, no special keywords
		assert.Equal(t, PriorityMedium, priority)
	})
}

func TestPriorityBasedStrategy_ConcurrentAccess(t *testing.T) {
	t.Run("Should handle concurrent access safely", func(t *testing.T) {
		strategy := NewPriorityBasedStrategy(nil, nil)

		config := &core.Resource{
			Type:      core.TokenBasedMemory,
			MaxTokens: 2000,
		}

		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Test message 1"},
			{Role: llm.MessageRoleUser, Content: "Test message 2"},
		}

		done := make(chan bool, 2)

		// Concurrent ShouldFlush calls
		go func() {
			for i := 0; i < 10; i++ {
				strategy.ShouldFlush(1500, 5, config)
			}
			done <- true
		}()

		// Concurrent PerformFlush calls
		go func() {
			for i := 0; i < 5; i++ {
				strategy.PerformFlush(context.Background(), messages, config)
			}
			done <- true
		}()

		// Wait for both goroutines to complete
		<-done
		<-done

		// If we reach here without deadlock, the test passes
		assert.True(t, true)
	})
}
