package strategies

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenAwareLRUStrategy_NewTokenAwareLRUStrategy(t *testing.T) {
	t.Run("Should create token-aware LRU strategy with default configuration", func(t *testing.T) {
		strategy, err := NewTokenAwareLRUStrategy(nil, nil)

		require.NoError(t, err)
		require.NotNil(t, strategy)
		assert.NotNil(t, strategy.cache)
		assert.NotNil(t, strategy.flushDecision)
		assert.Equal(t, int64(4000), strategy.maxTokens) // Should use default from options
		assert.Equal(t, 0.8, strategy.flushDecision.GetThreshold())
	})

	t.Run("Should create strategy with custom configuration", func(t *testing.T) {
		config := &core.FlushingStrategyConfig{
			Type:               core.TokenAwareLRUFlushing,
			SummarizeThreshold: 0.9,
		}
		options := &StrategyOptions{MaxTokens: 8000}

		strategy, err := NewTokenAwareLRUStrategy(config, options)

		require.NoError(t, err)
		require.NotNil(t, strategy)
		assert.Equal(t, config, strategy.config)
		assert.Equal(t, int64(8000), strategy.maxTokens)
		assert.Equal(t, 0.9, strategy.flushDecision.GetThreshold())
	})

	t.Run("Should use default max tokens when invalid value provided", func(t *testing.T) {
		options := &StrategyOptions{MaxTokens: -1} // Invalid value
		strategy, err := NewTokenAwareLRUStrategy(nil, options)

		require.NoError(t, err)
		require.NotNil(t, strategy)
		assert.Equal(t, int64(4000), strategy.maxTokens)
	})
}

func TestTokenAwareLRUStrategy_ShouldFlush(t *testing.T) {
	strategy, err := NewTokenAwareLRUStrategy(&core.FlushingStrategyConfig{
		Type:               core.TokenAwareLRUFlushing,
		SummarizeThreshold: 0.8,
	}, nil)
	require.NoError(t, err)

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

	t.Run("Should trigger flush for buffer memory based on tokens", func(t *testing.T) {
		config := &core.Resource{
			Type:        core.BufferMemory,
			MaxTokens:   2500,
			MaxMessages: 50,
		}

		// Below token threshold (80% of 2500 = 2000)
		shouldFlush := strategy.ShouldFlush(1900, 30, config)
		assert.False(t, shouldFlush)

		// At token threshold
		shouldFlush = strategy.ShouldFlush(2000, 30, config)
		assert.True(t, shouldFlush)

		// At message threshold (80% of 50 = 40)
		shouldFlush = strategy.ShouldFlush(1500, 40, config)
		assert.True(t, shouldFlush)
	})

	t.Run("Should handle message-based memory fallback", func(t *testing.T) {
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
	})
}

func TestTokenAwareLRUStrategy_PerformFlush(t *testing.T) {
	strategy, err := NewTokenAwareLRUStrategy(&core.FlushingStrategyConfig{
		Type:               core.TokenAwareLRUFlushing,
		SummarizeThreshold: 0.8,
	}, nil)
	require.NoError(t, err)

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

	t.Run("Should flush messages based on token cost efficiency", func(t *testing.T) {
		// Use a very small MaxTokens to ensure flush is needed
		config := &core.Resource{
			Type:      core.TokenBasedMemory,
			MaxTokens: 30, // Small limit to force flush
		}

		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Short"}, // Low token count (~6)
			{
				Role:    llm.MessageRoleAssistant,
				Content: "Much longer message with many more tokens that should be evicted first",
			}, // High token count (~22)
			{
				Role:    llm.MessageRoleUser,
				Content: "Medium length message",
			}, // Medium token count (~10)
			{
				Role:    llm.MessageRoleAssistant,
				Content: "Brief",
			}, // Low token count (~6)
		}

		result, err := strategy.PerformFlush(context.Background(), messages, config)

		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.False(t, result.SummaryGenerated)
		assert.Less(t, result.MessageCount, len(messages)) // Some messages should be flushed
		assert.Greater(t, result.TokenCount, 0)            // Some tokens should be freed
	})

	t.Run("Should respect token limits when flushing", func(t *testing.T) {
		config := &core.Resource{
			Type:      core.TokenBasedMemory,
			MaxTokens: 500,
		}

		// Create messages that exceed token limit
		messages := make([]llm.Message, 20)
		for i := range messages {
			messages[i] = llm.Message{
				Role:    llm.MessageRoleUser,
				Content: "This is a test message with some content",
			}
		}

		result, err := strategy.PerformFlush(context.Background(), messages, config)

		require.NoError(t, err)
		assert.True(t, result.Success)
		// Should reduce messages to fit within token budget
		assert.Less(t, result.MessageCount, len(messages))
	})

	t.Run("Should handle message-based configuration", func(t *testing.T) {
		config := &core.Resource{
			Type:        core.MessageCountBasedMemory,
			MaxMessages: 10,
		}

		messages := make([]llm.Message, 15)
		for i := range messages {
			messages[i] = llm.Message{
				Role:    llm.MessageRoleUser,
				Content: "Test message",
			}
		}

		result, err := strategy.PerformFlush(context.Background(), messages, config)

		require.NoError(t, err)
		assert.True(t, result.Success)
		// Should respect message count limits
		assert.LessOrEqual(t, result.MessageCount, 8) // Target ~60% of max
	})
}

func TestTokenAwareLRUStrategy_GetType(t *testing.T) {
	t.Run("Should return correct strategy type", func(t *testing.T) {
		strategy, err := NewTokenAwareLRUStrategy(nil, nil)
		require.NoError(t, err)

		strategyType := strategy.GetType()
		assert.Equal(t, core.TokenAwareLRUFlushing, strategyType)
	})
}

func TestTokenAwareLRUStrategy_GetMinMaxToFlush(t *testing.T) {
	strategy, err := NewTokenAwareLRUStrategy(nil, nil)
	require.NoError(t, err)

	t.Run("Should return token-aware min/max flush counts", func(t *testing.T) {
		totalMsgs := 30
		currentTokens := 3000
		maxTokens := 4000

		minFlush, maxFlush := strategy.GetMinMaxToFlush(context.Background(), totalMsgs, currentTokens, maxTokens)

		assert.Equal(t, 1, minFlush)
		assert.Greater(t, maxFlush, minFlush)
		assert.LessOrEqual(t, maxFlush, totalMsgs)
	})

	t.Run("Should handle high token pressure", func(t *testing.T) {
		totalMsgs := 20
		currentTokens := 3800 // Very high token usage
		maxTokens := 4000

		minFlush, maxFlush := strategy.GetMinMaxToFlush(context.Background(), totalMsgs, currentTokens, maxTokens)

		assert.Equal(t, 1, minFlush)
		// Should be more aggressive when token pressure is high
		assert.Greater(t, maxFlush, totalMsgs/3)
	})

	t.Run("Should handle edge case with few messages", func(t *testing.T) {
		totalMsgs := 3
		minFlush, maxFlush := strategy.GetMinMaxToFlush(context.Background(), totalMsgs, 1000, 2000)

		assert.Equal(t, 1, minFlush)
		assert.GreaterOrEqual(t, maxFlush, minFlush)
		assert.LessOrEqual(t, maxFlush, totalMsgs)
	})
}

func TestTokenAwareLRUStrategy_TokenCalculation(t *testing.T) {
	strategy, err := NewTokenAwareLRUStrategy(nil, nil)
	require.NoError(t, err)

	t.Run("Should estimate token counts consistently", func(t *testing.T) {
		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Short"},
			{Role: llm.MessageRoleUser, Content: "Much longer message with significantly more content"},
		}

		// Convert to MessageWithTokens
		msgWithTokens := make([]MessageWithTokens, len(messages))
		for i, msg := range messages {
			msgWithTokens[i] = MessageWithTokens{
				Message:    msg,
				TokenCount: strategy.tokenCounter.CountTokens(msg),
				Index:      i,
			}
		}

		// Longer message should have more tokens
		assert.Greater(t, msgWithTokens[1].TokenCount, msgWithTokens[0].TokenCount)

		// Token counts should be positive
		for _, msg := range msgWithTokens {
			assert.Greater(t, msg.TokenCount, 0)
		}
	})
}

func TestTokenAwareLRUStrategy_ConcurrentAccess(t *testing.T) {
	t.Run("Should handle concurrent access safely", func(t *testing.T) {
		strategy, err := NewTokenAwareLRUStrategy(nil, nil)
		require.NoError(t, err)

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
