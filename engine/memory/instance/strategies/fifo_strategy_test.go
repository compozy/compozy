package strategies

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory/core"
)

func TestFIFOStrategy_ShouldFlush(t *testing.T) {
	strategy := NewFIFOStrategy(0.8) // 80% threshold

	t.Run("Should trigger flush for token-based memory when threshold exceeded", func(t *testing.T) {
		config := &core.Resource{
			Type:      core.TokenBasedMemory,
			MaxTokens: 1000,
		}
		// 900 tokens = 90% > 80% threshold
		shouldFlush := strategy.ShouldFlush(900, 10, config)
		assert.True(t, shouldFlush)
	})

	t.Run("Should NOT trigger flush for token-based memory when under threshold", func(t *testing.T) {
		config := &core.Resource{
			Type:      core.TokenBasedMemory,
			MaxTokens: 1000,
		}
		// 700 tokens = 70% < 80% threshold
		shouldFlush := strategy.ShouldFlush(700, 10, config)
		assert.False(t, shouldFlush)
	})

	t.Run("Should trigger flush for message-count-based memory when threshold exceeded", func(t *testing.T) {
		config := &core.Resource{
			Type:        core.MessageCountBasedMemory,
			MaxMessages: 50,
		}
		// 45 messages = 90% > 80% threshold
		shouldFlush := strategy.ShouldFlush(1000, 45, config)
		assert.True(t, shouldFlush)
	})

	t.Run("Should NOT trigger flush for message-count-based memory when under threshold", func(t *testing.T) {
		config := &core.Resource{
			Type:        core.MessageCountBasedMemory,
			MaxMessages: 50,
		}
		// 30 messages = 60% < 80% threshold
		shouldFlush := strategy.ShouldFlush(1000, 30, config)
		assert.False(t, shouldFlush)
	})

	t.Run("Should trigger flush for buffer memory when token threshold exceeded", func(t *testing.T) {
		config := &core.Resource{
			Type:        core.BufferMemory,
			MaxTokens:   1000,
			MaxMessages: 100,
		}
		// 850 tokens = 85% > 80% threshold
		shouldFlush := strategy.ShouldFlush(850, 50, config)
		assert.True(t, shouldFlush)
	})

	t.Run("Should trigger flush for buffer memory when message threshold exceeded", func(t *testing.T) {
		config := &core.Resource{
			Type:        core.BufferMemory,
			MaxTokens:   1000,
			MaxMessages: 100,
		}
		// 85 messages = 85% > 80% threshold
		shouldFlush := strategy.ShouldFlush(500, 85, config)
		assert.True(t, shouldFlush)
	})

	t.Run("Should NOT trigger flush for buffer memory when both under threshold", func(t *testing.T) {
		config := &core.Resource{
			Type:        core.BufferMemory,
			MaxTokens:   1000,
			MaxMessages: 100,
		}
		// 700 tokens = 70% and 70 messages = 70%, both < 80% threshold
		shouldFlush := strategy.ShouldFlush(700, 70, config)
		assert.False(t, shouldFlush)
	})
}

func TestFIFOStrategy_PerformFlush(t *testing.T) {
	strategy := NewFIFOStrategy(0.8)
	ctx := t.Context()

	t.Run("Should handle empty message list", func(t *testing.T) {
		config := &core.Resource{
			Type:      core.TokenBasedMemory,
			MaxTokens: 1000,
		}
		messages := []llm.Message{}

		result, err := strategy.PerformFlush(ctx, messages, config)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.Success)
		assert.False(t, result.SummaryGenerated)
		assert.Equal(t, 0, result.MessageCount)
	})

	t.Run("Should flush messages to target capacity", func(t *testing.T) {
		config := &core.Resource{
			Type:        core.MessageCountBasedMemory,
			MaxMessages: 100,
		}
		// Create 80 messages (should reduce to 50 = 50% of max)
		messages := make([]llm.Message, 80)
		for i := range messages {
			messages[i] = llm.Message{
				Role:    llm.MessageRoleUser,
				Content: "Test message content",
			}
		}

		result, err := strategy.PerformFlush(ctx, messages, config)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.Success)
		assert.False(t, result.SummaryGenerated) // FIFO doesn't generate summaries
		// Should remove 30 messages (80 - 50 target)
		assert.Equal(t, 50, result.MessageCount)
	})

	t.Run("Should use default 25% removal when no max messages configured", func(t *testing.T) {
		config := &core.Resource{
			Type:      core.TokenBasedMemory,
			MaxTokens: 1000,
		}
		// Create 40 messages
		messages := make([]llm.Message, 40)
		for i := range messages {
			messages[i] = llm.Message{
				Role:    llm.MessageRoleUser,
				Content: "Test message content",
			}
		}

		result, err := strategy.PerformFlush(ctx, messages, config)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.Success)
		// Should remove 10 messages (25% of 40)
		assert.Equal(t, 30, result.MessageCount)
	})

	t.Run("Should not flush when no removal needed", func(t *testing.T) {
		config := &core.Resource{
			Type:        core.MessageCountBasedMemory,
			MaxMessages: 100,
		}
		// Create only 30 messages (already under 50% target)
		messages := make([]llm.Message, 30)
		for i := range messages {
			messages[i] = llm.Message{
				Role:    llm.MessageRoleUser,
				Content: "Test message content",
			}
		}

		result, err := strategy.PerformFlush(ctx, messages, config)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.Success)
		assert.False(t, result.SummaryGenerated)
		// Should not remove any messages
		assert.Equal(t, 30, result.MessageCount)
	})
}

func TestFIFOStrategy_GetType(t *testing.T) {
	strategy := NewFIFOStrategy(0.8)

	t.Run("Should return correct strategy type", func(t *testing.T) {
		assert.Equal(t, core.SimpleFIFOFlushing, strategy.GetType())
	})
}

func TestNewFIFOStrategy(t *testing.T) {
	t.Run("Should use provided threshold when valid", func(t *testing.T) {
		strategy := NewFIFOStrategy(0.9)
		require.NotNil(t, strategy)
		assert.Equal(t, 0.9, strategy.thresholdPercent)
	})

	t.Run("Should use default threshold when invalid threshold provided", func(t *testing.T) {
		strategy := NewFIFOStrategy(0.0) // Invalid threshold
		require.NotNil(t, strategy)
		assert.Equal(t, defaultThresholdPercent, strategy.thresholdPercent)

		strategy = NewFIFOStrategy(1.5) // Invalid threshold
		require.NotNil(t, strategy)
		assert.Equal(t, defaultThresholdPercent, strategy.thresholdPercent)

		strategy = NewFIFOStrategy(-0.1) // Invalid threshold
		require.NotNil(t, strategy)
		assert.Equal(t, defaultThresholdPercent, strategy.thresholdPercent)
	})
}

func TestFIFOStrategy_calculateMessagesToRemove(t *testing.T) {
	strategy := NewFIFOStrategy(0.8)

	t.Run("Should calculate correct removal for configured max messages", func(t *testing.T) {
		config := &core.Resource{
			Type:        core.MessageCountBasedMemory,
			MaxMessages: 100,
		}
		// 80 current messages, target is 50 (50% of 100), so remove 30
		toRemove := strategy.calculateMessagesToRemove(80, config)
		assert.Equal(t, 30, toRemove)
	})

	t.Run("Should return 0 when already under target", func(t *testing.T) {
		config := &core.Resource{
			Type:        core.MessageCountBasedMemory,
			MaxMessages: 100,
		}
		// 40 current messages, target is 50, so remove 0
		toRemove := strategy.calculateMessagesToRemove(40, config)
		assert.Equal(t, 0, toRemove)
	})

	t.Run("Should use default 25% when no max messages configured", func(t *testing.T) {
		config := &core.Resource{
			Type:      core.TokenBasedMemory,
			MaxTokens: 1000,
		}
		// 40 current messages, remove 25% = 10
		toRemove := strategy.calculateMessagesToRemove(40, config)
		assert.Equal(t, 10, toRemove)
	})
}
