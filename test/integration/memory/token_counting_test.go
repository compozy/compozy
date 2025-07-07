package memory

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
)

// TestTokenCountingWithMemoryIntegration tests token counting integrated with memory
func TestTokenCountingWithMemoryIntegration(t *testing.T) {
	t.Run("Should track token counts accurately in memory operations", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()
		// Create memory instance
		memRef := core.MemoryReference{
			ID:  "customer-support",
			Key: "token-tracking-{{.test.id}}",
		}
		workflowContext := map[string]any{
			"project": map[string]any{"id": "test-project"},
			"test":    map[string]any{"id": fmt.Sprintf("token-%d", time.Now().Unix())},
		}
		instance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
		require.NoError(t, err)
		// Test messages with known content
		messages := []llm.Message{
			{
				Role:    "system",
				Content: "You are a helpful assistant.",
			},
			{
				Role:    "user",
				Content: "What is the weather like today?",
			},
			{
				Role:    "assistant",
				Content: "I don't have access to real-time weather data, but I'd be happy to help you find weather information if you tell me your location.",
			},
		}
		// Add messages and track token counts
		runningTokenCount := 0
		for i, msg := range messages {
			// Get token count before adding
			tokensBefore, err := instance.GetTokenCount(ctx)
			require.NoError(t, err)
			// Append message
			err = instance.Append(ctx, msg)
			require.NoError(t, err)
			// Get token count after adding
			tokensAfter, err := instance.GetTokenCount(ctx)
			require.NoError(t, err)
			// Calculate tokens added
			tokensAdded := tokensAfter - tokensBefore
			runningTokenCount += tokensAdded
			t.Logf("Message %d (%s): Added %d tokens, total now %d",
				i+1, msg.Role, tokensAdded, tokensAfter)
			// Verify tokens were added
			assert.Greater(t, tokensAdded, 0, "Should add tokens for message %d", i+1)
			assert.Equal(t, runningTokenCount, tokensAfter, "Running count should match actual")
		}
		// Verify final health metrics
		health, err := instance.GetMemoryHealth(ctx)
		require.NoError(t, err)
		assert.Equal(t, len(messages), health.MessageCount)
		assert.Equal(t, runningTokenCount, health.TokenCount)
		// Test token counting after clear
		err = instance.Clear(ctx)
		require.NoError(t, err)
		finalTokens, err := instance.GetTokenCount(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, finalTokens, "Token count should be 0 after clear")
	})
}

// TestTokenCountingConsistency tests token counting consistency across operations
func TestTokenCountingConsistency(t *testing.T) {
	t.Run("Should maintain consistent token counts across operations", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()
		memRef := core.MemoryReference{
			ID:  "customer-support",
			Key: "token-consistency-{{.test.id}}",
		}
		workflowContext := map[string]any{
			"project": map[string]any{"id": "test-project"},
			"test":    map[string]any{"id": fmt.Sprintf("consistency-%d", time.Now().Unix())},
		}
		instance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
		require.NoError(t, err)
		// Add multiple messages
		messages := []llm.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there! How can I help you today?"},
			{Role: "user", Content: "Tell me a joke"},
			{Role: "assistant", Content: "Why don't scientists trust atoms? Because they make up everything!"},
		}
		for _, msg := range messages {
			err := instance.Append(ctx, msg)
			require.NoError(t, err)
		}
		// Get initial counts
		health1, err := instance.GetMemoryHealth(ctx)
		require.NoError(t, err)
		tokenCount1, err := instance.GetTokenCount(ctx)
		require.NoError(t, err)
		// Verify counts match
		assert.Equal(t, health1.TokenCount, tokenCount1)
		assert.Equal(t, len(messages), health1.MessageCount)
		// Read messages and verify count remains consistent
		readMessages, err := instance.Read(ctx)
		require.NoError(t, err)
		assert.Len(t, readMessages, len(messages))
		// Token count should not change after read
		tokenCount2, err := instance.GetTokenCount(ctx)
		require.NoError(t, err)
		assert.Equal(t, tokenCount1, tokenCount2)
	})
}

// TestTokenCountingWithFlush tests token counting during flush operations
func TestTokenCountingWithFlush(t *testing.T) {
	t.Run("Should update token counts correctly during flush", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()
		memRef := core.MemoryReference{
			ID:  "flushable-memory",
			Key: "token-flush-{{.test.id}}",
		}
		workflowContext := map[string]any{
			"project": map[string]any{"id": "test-project"},
			"test":    map[string]any{"id": fmt.Sprintf("flush-%d", time.Now().Unix())},
		}
		instance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
		require.NoError(t, err)
		// Add messages to trigger flush
		for i := 0; i < 30; i++ {
			msg := llm.Message{
				Role:    "user",
				Content: fmt.Sprintf("Message %d - adding content to reach flush threshold", i),
			}
			err := instance.Append(ctx, msg)
			require.NoError(t, err)
		}
		// Get token count before flush
		tokensBefore, err := instance.GetTokenCount(ctx)
		require.NoError(t, err)
		healthBefore, err := instance.GetMemoryHealth(ctx)
		require.NoError(t, err)
		t.Logf("Before flush: %d messages, %d tokens", healthBefore.MessageCount, tokensBefore)
		// Perform flush if supported
		if flushable, ok := instance.(interface {
			PerformFlush(context.Context) (*struct {
				Success      bool
				MessageCount int
				TokenCount   int
			}, error)
		}); ok {
			result, err := flushable.PerformFlush(ctx)
			require.NoError(t, err)
			require.NotNil(t, result)
			// Get token count after flush
			tokensAfter, err := instance.GetTokenCount(ctx)
			require.NoError(t, err)
			healthAfter, err := instance.GetMemoryHealth(ctx)
			require.NoError(t, err)
			t.Logf("After flush: %d messages, %d tokens", healthAfter.MessageCount, tokensAfter)
			t.Logf("Flush removed: %d messages, %d tokens", result.MessageCount, result.TokenCount)
			// Verify token count decreased
			assert.Less(t, tokensAfter, tokensBefore)
			assert.Equal(t, healthAfter.TokenCount, tokensAfter)
			// Verify the reduction matches what was reported
			expectedTokens := tokensBefore - result.TokenCount
			assert.GreaterOrEqual(t, tokensAfter, expectedTokens) // May have summary overhead
		}
	})
}

// TestTokenCountingEdgeCases tests edge cases in token counting
func TestTokenCountingEdgeCases(t *testing.T) {
	t.Run("Should handle edge cases in token counting", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()
		memRef := core.MemoryReference{
			ID:  "customer-support",
			Key: "edge-case-{{.test.id}}",
		}
		workflowContext := map[string]any{
			"project": map[string]any{"id": "test-project"},
			"test":    map[string]any{"id": fmt.Sprintf("edge-%d", time.Now().Unix())},
		}
		instance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
		require.NoError(t, err)
		// Test cases
		testCases := []struct {
			name      string
			messages  []llm.Message
			minTokens int
		}{
			{
				name:      "Empty message",
				messages:  []llm.Message{{Role: "user", Content: ""}},
				minTokens: 1, // Role still counts
			},
			{
				name: "Very long message",
				messages: []llm.Message{{
					Role:    "user",
					Content: string(make([]byte, 1000)), // 1KB of null bytes
				}},
				minTokens: 100, // Long content
			},
			{
				name: "Special characters",
				messages: []llm.Message{{
					Role:    "user",
					Content: "Hello 👋 世界 🌍 مرحبا 🎉",
				}},
				minTokens: 5,
			},
			{
				name: "Multiple languages",
				messages: []llm.Message{{
					Role:    "user",
					Content: "English, 中文, العربية, हिन्दी, 日本語",
				}},
				minTokens: 5,
			},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Clear memory first
				err := instance.Clear(ctx)
				require.NoError(t, err)
				// Add messages
				for _, msg := range tc.messages {
					err := instance.Append(ctx, msg)
					require.NoError(t, err)
				}
				// Get token count
				tokens, err := instance.GetTokenCount(ctx)
				require.NoError(t, err)
				t.Logf("%s: %d tokens", tc.name, tokens)
				// Verify minimum tokens
				assert.GreaterOrEqual(t, tokens, tc.minTokens)
			})
		}
	})
}

// TestTokenCountingConcurrency tests concurrent token counting operations
func TestTokenCountingConcurrency(t *testing.T) {
	t.Run("Should handle concurrent token counting operations", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()
		memRef := core.MemoryReference{
			ID:  "shared-memory",
			Key: "token-concurrent-{{.test.id}}",
		}
		workflowContext := map[string]any{
			"project": map[string]any{"id": "test-project"},
			"test":    map[string]any{"id": fmt.Sprintf("concurrent-%d", time.Now().Unix())},
		}
		instance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
		require.NoError(t, err)
		// Add initial messages
		for i := 0; i < 10; i++ {
			msg := llm.Message{
				Role:    "user",
				Content: fmt.Sprintf("Initial message %d", i),
			}
			err := instance.Append(ctx, msg)
			require.NoError(t, err)
		}
		// Launch concurrent readers
		const numReaders = 10
		tokenCounts := make(chan int, numReaders)
		errors := make(chan error, numReaders)
		for i := 0; i < numReaders; i++ {
			go func(_ int) {
				// Get token count
				count, err := instance.GetTokenCount(ctx)
				if err != nil {
					errors <- err
					return
				}
				tokenCounts <- count
				errors <- nil
			}(i)
		}
		// Collect results
		var counts []int
		for i := 0; i < numReaders; i++ {
			select {
			case err := <-errors:
				require.NoError(t, err)
			case count := <-tokenCounts:
				counts = append(counts, count)
			}
		}
		// All readers should see the same token count
		if len(counts) > 0 {
			expectedCount := counts[0]
			for i, count := range counts {
				assert.Equal(t, expectedCount, count, "Reader %d saw different count", i)
			}
			t.Logf("All %d readers saw consistent token count: %d", numReaders, expectedCount)
		}
	})
}
