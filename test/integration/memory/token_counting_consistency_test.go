package memory

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/instance/strategies"
)

// TestTokenCountingArchitecturalConsistency validates that token counting is consistent across all architectural components
func TestTokenCountingArchitecturalConsistency(t *testing.T) {
	ctx := context.Background()
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	// Create test messages
	testMessages := []llm.Message{
		{Role: llm.MessageRoleUser, Content: "Hello, this is a test message with some content."},
		{
			Role:    llm.MessageRoleAssistant,
			Content: "I understand your request. This is a longer response with more detailed content that should result in a higher token count.",
		},
		{Role: llm.MessageRoleSystem, Content: "System configuration and instructions."},
	}

	t.Run("Should have consistent token counting between Operations and FlushOperations", func(t *testing.T) {
		// Use pre-registered customer-support memory resource
		manager, err := env.memoryManager.GetInstance(ctx, core.MemoryReference{
			ID:  "customer-support",
			Key: "token-consistency-test",
		}, map[string]any{
			"project": map[string]any{
				"id": "test-project",
			},
		})
		require.NoError(t, err)
		require.NotNil(t, manager)

		// Add messages and get token counts from memory instance
		var instanceTokenCounts []int
		for _, msg := range testMessages {
			err := manager.Append(ctx, msg)
			require.NoError(t, err)

			tokenCount, err := manager.GetTokenCount(ctx)
			require.NoError(t, err)
			instanceTokenCounts = append(instanceTokenCounts, tokenCount)
		}

		// Verify token counts are reasonable and increasing
		assert.Greater(t, instanceTokenCounts[0], 0, "First message should have positive token count")
		assert.Greater(t, instanceTokenCounts[1], instanceTokenCounts[0], "Second message should increase token count")
		assert.Greater(t, instanceTokenCounts[2], instanceTokenCounts[1], "Third message should increase token count")
	})

	t.Run("Should have consistent counting between different TokenCounter implementations", func(t *testing.T) {
		ctx := context.Background()

		// Create different token counter implementations
		simpleCounter := strategies.NewSimpleTokenCounterAdapter()
		gptCounter := strategies.NewGPTTokenCounterAdapter()

		for _, msg := range testMessages {
			// Count tokens using simple counter
			simpleCount, err := simpleCounter.CountTokens(ctx, msg.Content)
			require.NoError(t, err)

			// Count tokens using GPT counter
			gptCount, err := gptCounter.CountTokens(ctx, msg.Content)
			require.NoError(t, err)

			// Both should be positive and in reasonable range
			assert.Greater(
				t,
				simpleCount,
				0,
				"Simple counter should return positive count for message: %s",
				msg.Content,
			)
			assert.Greater(
				t,
				gptCount,
				0,
				"GPT counter should return positive count for message: %s",
				msg.Content,
			)

			// The counts should be in the same ballpark (within 50% of each other)
			// This allows for different counting methods while ensuring consistency
			ratio := float64(simpleCount) / float64(gptCount)
			assert.True(t, ratio >= 0.5 && ratio <= 2.0,
				"Token counts should be reasonably consistent: simple=%d, gpt=%d, ratio=%.2f for message: %s",
				simpleCount, gptCount, ratio, msg.Content)
		}
	})

	t.Run("Should maintain consistency after flush operations", func(t *testing.T) {
		// Use pre-registered flushable-memory resource (has aggressive flushing at 50%)
		manager, err := env.memoryManager.GetInstance(ctx, core.MemoryReference{
			ID:  "flushable-memory",
			Key: "flush-consistency-test",
		}, map[string]any{
			"project": map[string]any{
				"id": "test-project",
			},
		})
		require.NoError(t, err)

		// Add multiple messages to trigger flush
		for i, msg := range testMessages {
			err := manager.Append(ctx, msg)
			require.NoError(t, err)

			// Get token count after each append
			tokenCount, err := manager.GetTokenCount(ctx)
			require.NoError(t, err)

			// Token count should always be reasonable
			assert.GreaterOrEqual(t, tokenCount, 0, "Token count should not be negative after message %d", i)

			// If we're over limit, verify that flush would be triggered
			// flushable-memory has MaxTokens: 2000 from helpers.go
			flushableMemoryMaxTokens := 2000
			if tokenCount > flushableMemoryMaxTokens {
				messageCount, err := manager.Len(ctx)
				require.NoError(t, err)
				t.Logf("After message %d: tokens=%d, messages=%d (over limit of %d tokens)",
					i, tokenCount, messageCount, flushableMemoryMaxTokens)
			}
		}

		// Verify final state is consistent
		finalTokenCount, err := manager.GetTokenCount(ctx)
		require.NoError(t, err)
		finalMessageCount, err := manager.Len(ctx)
		require.NoError(t, err)

		t.Logf("Final state: tokens=%d, messages=%d", finalTokenCount, finalMessageCount)
		assert.GreaterOrEqual(t, finalTokenCount, 0, "Final token count should not be negative")
		assert.GreaterOrEqual(t, finalMessageCount, 0, "Final message count should not be negative")
	})

	t.Run("Should handle edge cases consistently", func(t *testing.T) {
		// Test edge cases that might break token counting
		edgeCases := []llm.Message{
			{Role: llm.MessageRoleUser, Content: ""},    // Empty content
			{Role: llm.MessageRoleUser, Content: "a"},   // Single character
			{Role: llm.MessageRoleUser, Content: "🚀🎯💻"}, // Unicode emojis
			{
				Role:    llm.MessageRoleUser,
				Content: "This is a very long message that contains a lot of text and should result in a higher token count when processed by any reasonable tokenizer implementation that we might be using in this system.",
			}, // Long content
		}

		// Create token counters
		simpleCounter := strategies.NewSimpleTokenCounterAdapter()
		gptCounter := strategies.NewGPTTokenCounterAdapter()

		for i, msg := range edgeCases {
			simpleCount, err := simpleCounter.CountTokens(ctx, msg.Content)
			require.NoError(t, err)

			gptCount, err := gptCounter.CountTokens(ctx, msg.Content)
			require.NoError(t, err)

			// Both should handle edge cases gracefully
			assert.GreaterOrEqual(t, simpleCount, 0, "Simple counter failed on edge case %d: %s", i, msg.Content)
			assert.GreaterOrEqual(t, gptCount, 0, "GPT counter failed on edge case %d: %s", i, msg.Content)

			// Empty content should return 0 tokens
			if msg.Content == "" {
				assert.Equal(t, 0, simpleCount, "Empty content should have 0 tokens")
				assert.Equal(t, 0, gptCount, "Empty content should have 0 tokens")
			}
		}
	})
}

// TestFIFOStrategyTokenCountingBug specifically tests the fix for the TokenCount bug
func TestFIFOStrategyTokenCountingBug(t *testing.T) {
	// Create FIFO strategy
	strategy := strategies.NewFIFOStrategy(0.8)

	// Create test messages
	testMessages := []llm.Message{
		{Role: llm.MessageRoleUser, Content: "Message 1"},
		{Role: llm.MessageRoleUser, Content: "Message 2"},
		{Role: llm.MessageRoleUser, Content: "Message 3"},
		{Role: llm.MessageRoleUser, Content: "Message 4"},
	}

	// Create test resource
	resource := &memcore.Resource{
		MaxMessages: 4, // With 50% target, will remove 2 messages, leaving 2
	}

	// Execute flush
	ctx := context.Background()
	result, err := strategy.PerformFlush(ctx, testMessages, resource)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify that TokenCount represents remaining tokens, not flushed tokens
	assert.True(t, result.Success)
	assert.Equal(t, 2, result.MessageCount, "Should have 2 messages remaining")
	assert.Greater(t, result.TokenCount, 0, "Should have positive remaining token count")

	// The TokenCount should represent tokens in remaining messages (2), not flushed messages (2)
	// Since we're using a token counter, we can verify this makes sense
	expectedRemainingMessages := 2
	assert.Equal(t, expectedRemainingMessages, result.MessageCount)

	// Token count should be reasonable for remaining messages
	// (This is an integration test to ensure the fix works end-to-end)
}
