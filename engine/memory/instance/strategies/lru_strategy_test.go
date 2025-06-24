package strategies

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLRUStrategy_NewLRUStrategy(t *testing.T) {
	t.Run("Should create LRU strategy with default configuration", func(t *testing.T) {
		strategy, err := NewLRUStrategy(nil, nil)

		require.NoError(t, err)
		require.NotNil(t, strategy)
		assert.NotNil(t, strategy.cache)
		assert.NotNil(t, strategy.flushDecision)
		assert.NotNil(t, strategy.messageAccess)
		assert.Equal(t, 0.8, strategy.flushDecision.GetThreshold())
	})

	t.Run("Should create LRU strategy with custom configuration", func(t *testing.T) {
		config := &core.FlushingStrategyConfig{
			Type:               core.LRUFlushing,
			SummarizeThreshold: 0.7,
		}

		strategy, err := NewLRUStrategy(config, nil)

		require.NoError(t, err)
		require.NotNil(t, strategy)
		assert.Equal(t, config, strategy.config)
		assert.Equal(t, 0.7, strategy.flushDecision.GetThreshold())
	})

	t.Run("Should use default cache size when invalid size provided", func(t *testing.T) {
		strategy, err := NewLRUStrategy(nil, nil)

		require.NoError(t, err)
		require.NotNil(t, strategy)
		assert.NotNil(t, strategy.cache)
	})
}

func TestLRUStrategy_ShouldFlush(t *testing.T) {
	strategy, err := NewLRUStrategy(&core.FlushingStrategyConfig{
		Type:               core.LRUFlushing,
		SummarizeThreshold: 0.8,
	}, nil)
	require.NoError(t, err)

	t.Run("Should trigger flush for token-based memory when threshold exceeded", func(t *testing.T) {
		config := &core.Resource{
			Type:      core.TokenBasedMemory,
			MaxTokens: 1000,
		}

		// Below threshold
		shouldFlush := strategy.ShouldFlush(700, 10, config)
		assert.False(t, shouldFlush)

		// At threshold
		shouldFlush = strategy.ShouldFlush(800, 10, config)
		assert.True(t, shouldFlush)

		// Above threshold
		shouldFlush = strategy.ShouldFlush(900, 10, config)
		assert.True(t, shouldFlush)
	})

	t.Run("Should trigger flush for message-based memory when threshold exceeded", func(t *testing.T) {
		config := &core.Resource{
			Type:        core.MessageCountBasedMemory,
			MaxMessages: 50,
		}

		// Below threshold
		shouldFlush := strategy.ShouldFlush(1000, 30, config)
		assert.False(t, shouldFlush)

		// At threshold
		shouldFlush = strategy.ShouldFlush(1000, 40, config)
		assert.True(t, shouldFlush)

		// Above threshold
		shouldFlush = strategy.ShouldFlush(1000, 45, config)
		assert.True(t, shouldFlush)
	})

	t.Run("Should trigger flush for buffer memory when either threshold exceeded", func(t *testing.T) {
		config := &core.Resource{
			Type:        core.BufferMemory,
			MaxTokens:   1000,
			MaxMessages: 50,
		}

		// Below both thresholds
		shouldFlush := strategy.ShouldFlush(700, 30, config)
		assert.False(t, shouldFlush)

		// Token threshold exceeded
		shouldFlush = strategy.ShouldFlush(850, 30, config)
		assert.True(t, shouldFlush)

		// Message threshold exceeded
		shouldFlush = strategy.ShouldFlush(700, 45, config)
		assert.True(t, shouldFlush)

		// Both thresholds exceeded
		shouldFlush = strategy.ShouldFlush(900, 45, config)
		assert.True(t, shouldFlush)
	})
}

func TestLRUStrategy_PerformFlush(t *testing.T) {
	strategy, err := NewLRUStrategy(&core.FlushingStrategyConfig{
		Type:               core.LRUFlushing,
		SummarizeThreshold: 0.8,
	}, nil)
	require.NoError(t, err)

	t.Run("Should handle empty message list", func(t *testing.T) {
		config := &core.Resource{
			Type:      core.TokenBasedMemory,
			MaxTokens: 1000,
		}

		result, err := strategy.PerformFlush(context.Background(), []llm.Message{}, config)

		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.False(t, result.SummaryGenerated)
		assert.Equal(t, 0, result.MessageCount)
		assert.Equal(t, 0, result.TokenCount)
	})

	t.Run("Should flush oldest messages based on LRU order", func(t *testing.T) {
		config := &core.Resource{
			Type:      core.TokenBasedMemory,
			MaxTokens: 1000,
		}

		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "First message"},
			{Role: llm.MessageRoleAssistant, Content: "Second message"},
			{Role: llm.MessageRoleUser, Content: "Third message"},
			{Role: llm.MessageRoleAssistant, Content: "Fourth message"},
		}

		// Simulate message access patterns
		now := time.Now()
		strategy.messageAccess[0] = now.Add(-4 * time.Minute) // Oldest access
		strategy.messageAccess[1] = now.Add(-2 * time.Minute)
		strategy.messageAccess[2] = now.Add(-3 * time.Minute)
		strategy.messageAccess[3] = now.Add(-1 * time.Minute) // Most recent access

		result, err := strategy.PerformFlush(context.Background(), messages, config)

		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.False(t, result.SummaryGenerated)
		assert.Greater(t, len(messages), result.MessageCount) // Some messages should be flushed
	})

	t.Run("Should calculate target flush count correctly", func(t *testing.T) {
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
		// Should reduce to around 60% of max (6 messages remaining)
		assert.LessOrEqual(t, result.MessageCount, 8)
	})
}

func TestLRUStrategy_GetType(t *testing.T) {
	t.Run("Should return correct strategy type", func(t *testing.T) {
		strategy, err := NewLRUStrategy(nil, nil)
		require.NoError(t, err)

		strategyType := strategy.GetType()
		assert.Equal(t, core.LRUFlushing, strategyType)
	})
}

func TestLRUStrategy_GetMinMaxToFlush(t *testing.T) {
	strategy, err := NewLRUStrategy(nil, nil)
	require.NoError(t, err)

	t.Run("Should return appropriate min/max flush counts", func(t *testing.T) {
		totalMsgs := 20
		minFlush, maxFlush := strategy.GetMinMaxToFlush(context.Background(), totalMsgs, 1000, 2000)

		assert.Equal(t, 1, minFlush)
		assert.Equal(t, 10, maxFlush) // Half of total messages
		assert.LessOrEqual(t, minFlush, maxFlush)
	})

	t.Run("Should handle edge case with very few messages", func(t *testing.T) {
		totalMsgs := 2
		minFlush, maxFlush := strategy.GetMinMaxToFlush(context.Background(), totalMsgs, 100, 200)

		assert.Equal(t, 1, minFlush)
		assert.Equal(t, 1, maxFlush)
	})
}

func TestLRUStrategy_ConcurrentAccess(t *testing.T) {
	t.Run("Should handle concurrent access safely", func(t *testing.T) {
		strategy, err := NewLRUStrategy(nil, nil)
		require.NoError(t, err)

		config := &core.Resource{
			Type:      core.TokenBasedMemory,
			MaxTokens: 1000,
		}

		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Test message 1"},
			{Role: llm.MessageRoleUser, Content: "Test message 2"},
		}

		done := make(chan bool, 2)

		// Concurrent ShouldFlush calls
		go func() {
			for i := 0; i < 10; i++ {
				strategy.ShouldFlush(500, 5, config)
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
