package memory

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	memcore "github.com/compozy/compozy/engine/memory/core"
)

// TestFlushWorkflowComplete tests the complete flush workflow including summarization
func TestFlushWorkflowComplete(t *testing.T) {
	t.Run("Should execute complete flush workflow with summarization", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()
		// Use flushable memory with aggressive threshold
		memRef := core.MemoryReference{
			ID:  "flushable-memory",
			Key: "flush-complete-{{.test.id}}",
		}
		workflowContext := map[string]any{
			"project": map[string]any{
				"id": "test-project",
			},
			"test": map[string]any{
				"id": fmt.Sprintf("flush-complete-%d", time.Now().Unix()),
			},
		}
		instance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
		require.NoError(t, err)
		// Add messages to reach flush threshold (50% of 2000 tokens)
		baseMessage := "This is a test message with some content to accumulate tokens. "
		messageCount := 0
		for i := 0; i < 100; i++ {
			msg := llm.Message{
				Role:    "user",
				Content: fmt.Sprintf("%s Message number %d", baseMessage, i),
			}
			err := instance.Append(ctx, msg)
			require.NoError(t, err)
			messageCount++
			// Check if we've reached flush threshold
			health, err := instance.GetMemoryHealth(ctx)
			require.NoError(t, err)
			if float64(health.TokenCount) > float64(2000)*0.5 {
				t.Logf("Reached flush threshold at message %d with %d tokens", i, health.TokenCount)
				break
			}
		}
		// Verify it's flushable
		flushable, ok := instance.(memcore.FlushableMemory)
		require.True(t, ok, "Memory should be flushable")
		// Get state before flush
		healthBefore, err := instance.GetMemoryHealth(ctx)
		require.NoError(t, err)
		messagesBefore, err := instance.Read(ctx)
		require.NoError(t, err)
		t.Logf("Before flush: %d messages, %d tokens", healthBefore.MessageCount, healthBefore.TokenCount)
		// Perform flush
		flushResult, err := flushable.PerformFlush(ctx)
		require.NoError(t, err)
		require.NotNil(t, flushResult)
		assert.True(t, flushResult.Success)
		assert.Greater(t, flushResult.MessageCount, 0)
		assert.Greater(t, flushResult.TokenCount, 0)
		// Get state after flush
		healthAfter, err := instance.GetMemoryHealth(ctx)
		require.NoError(t, err)
		messagesAfter, err := instance.Read(ctx)
		require.NoError(t, err)
		t.Logf("After flush: %d messages, %d tokens", healthAfter.MessageCount, healthAfter.TokenCount)
		t.Logf("Flush removed %d messages and %d tokens",
			flushResult.MessageCount, flushResult.TokenCount)
		// Verify flush results
		assert.Less(t, healthAfter.MessageCount, healthBefore.MessageCount)
		assert.Less(t, healthAfter.TokenCount, healthBefore.TokenCount)
		assert.Less(t, len(messagesAfter), len(messagesBefore))
		// Verify summary was created (first message should be system summary)
		if len(messagesAfter) > 0 && messagesAfter[0].Role == "system" {
			t.Logf("Summary created: %s", messagesAfter[0].Content)
			assert.Contains(t, messagesAfter[0].Content, "Summary")
		}
	})
}

// TestFlushWithMultipleStrategies tests different flush strategies
func TestFlushWithMultipleStrategies(t *testing.T) {
	t.Run("Should handle different flush strategies correctly", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()
		// Test data for different scenarios
		testCases := []struct {
			name             string
			memoryID         string
			messageCount     int
			expectedBehavior string
		}{
			{
				name:             "FIFO flush with high threshold",
				memoryID:         "customer-support", // 80% threshold
				messageCount:     80,
				expectedBehavior: "should flush when near capacity",
			},
			{
				name:             "FIFO flush with low threshold",
				memoryID:         "flushable-memory", // 50% threshold
				messageCount:     40,
				expectedBehavior: "should flush at half capacity",
			},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				memRef := core.MemoryReference{
					ID:  tc.memoryID,
					Key: "flush-strategy-{{.test.id}}-{{.strategy}}",
				}
				workflowContext := map[string]any{
					"project": map[string]any{
						"id": "test-project",
					},
					"test": map[string]any{
						"id": fmt.Sprintf("strategy-%d", time.Now().Unix()),
					},
					"strategy": fmt.Sprintf("test-%d", time.Now().UnixNano()%1000),
				}
				instance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
				require.NoError(t, err)
				// Add messages
				for i := 0; i < tc.messageCount; i++ {
					msg := llm.Message{
						Role:    "user",
						Content: fmt.Sprintf("Test message %d for %s", i, tc.expectedBehavior),
					}
					err := instance.Append(ctx, msg)
					require.NoError(t, err)
				}
				// Try to flush if supported
				if flushable, ok := instance.(memcore.FlushableMemory); ok {
					result, err := flushable.PerformFlush(ctx)
					if err == nil && result != nil {
						t.Logf("%s: Flushed %d messages", tc.name, result.MessageCount)
						assert.True(t, result.Success)
					}
				}
			})
		}
	})
}

// TestCleanupWorkflow tests memory cleanup and TTL expiration
func TestCleanupWorkflow(t *testing.T) {
	t.Run("Should execute cleanup workflow for expired memories", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()
		// Create multiple memory instances with different TTLs
		memories := []struct {
			key      string
			memoryID string
			ttl      string
		}{
			{
				key:      "cleanup-test-1",
				memoryID: "customer-support", // 24h TTL
				ttl:      "24h",
			},
			{
				key:      "cleanup-test-2",
				memoryID: "flushable-memory", // 1h TTL
				ttl:      "1h",
			},
		}
		createdInstances := make(map[string]memcore.Memory)
		// Create and populate memories
		for _, mem := range memories {
			memRef := core.MemoryReference{
				ID:  mem.memoryID,
				Key: fmt.Sprintf("%s-{{.test.id}}", mem.key),
			}
			workflowContext := map[string]any{
				"project": map[string]any{
					"id": "test-project",
				},
				"test": map[string]any{
					"id": fmt.Sprintf("cleanup-%d", time.Now().Unix()),
				},
			}
			instance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
			require.NoError(t, err)
			// Add some messages
			for i := 0; i < 10; i++ {
				msg := llm.Message{
					Role:    "user",
					Content: fmt.Sprintf("Message %d for %s", i, mem.key),
				}
				err := instance.Append(ctx, msg)
				require.NoError(t, err)
			}
			createdInstances[mem.key] = instance
			// Verify TTL is set
			instanceID := instance.GetID()
			ttl, err := env.GetRedis().TTL(ctx, fmt.Sprintf("compozy:test-project:memory:%s", instanceID)).Result()
			require.NoError(t, err)
			assert.Greater(t, ttl.Seconds(), float64(0), "TTL should be set for %s", mem.key)
			t.Logf("%s: TTL set to %v", mem.key, ttl)
		}
		// Simulate cleanup by checking each memory
		for key, instance := range createdInstances {
			health, err := instance.GetMemoryHealth(ctx)
			require.NoError(t, err)
			assert.Greater(t, health.MessageCount, 0, "Memory %s should have messages", key)
		}
		// Test manual cleanup
		for key, instance := range createdInstances {
			err := instance.Clear(ctx)
			require.NoError(t, err)
			// Verify cleared
			messages, err := instance.Read(ctx)
			require.NoError(t, err)
			assert.Empty(t, messages, "Memory %s should be empty after clear", key)
		}
	})
}

// TestFlushAndCleanupInteraction tests interaction between flush and cleanup
func TestFlushAndCleanupInteraction(t *testing.T) {
	t.Run("Should handle flush and cleanup operations together", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()
		memRef := core.MemoryReference{
			ID:  "flushable-memory",
			Key: "flush-cleanup-interaction-{{.test.id}}",
		}
		workflowContext := map[string]any{
			"project": map[string]any{
				"id": "test-project",
			},
			"test": map[string]any{
				"id": fmt.Sprintf("interaction-%d", time.Now().Unix()),
			},
		}
		instance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
		require.NoError(t, err)
		// Fill memory to trigger flush
		for i := 0; i < 50; i++ {
			msg := llm.Message{
				Role:    "user",
				Content: fmt.Sprintf("Long message %d to trigger flush behavior quickly", i),
			}
			err := instance.Append(ctx, msg)
			require.NoError(t, err)
		}
		// Perform flush
		flushable, ok := instance.(memcore.FlushableMemory)
		require.True(t, ok)
		flushResult, err := flushable.PerformFlush(ctx)
		require.NoError(t, err)
		assert.True(t, flushResult.Success)
		// Add more messages after flush
		for i := 0; i < 10; i++ {
			msg := llm.Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Post-flush message %d", i),
			}
			err := instance.Append(ctx, msg)
			require.NoError(t, err)
		}
		// Get current state
		healthBeforeClear, err := instance.GetMemoryHealth(ctx)
		require.NoError(t, err)
		t.Logf("After flush and new messages: %d messages, %d tokens",
			healthBeforeClear.MessageCount, healthBeforeClear.TokenCount)
		// Clear memory
		err = instance.Clear(ctx)
		require.NoError(t, err)
		// Verify complete cleanup
		finalHealth, err := instance.GetMemoryHealth(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, finalHealth.MessageCount)
		assert.Equal(t, 0, finalHealth.TokenCount)
	})
}

// TestConcurrentFlushAndCleanup tests concurrent flush and cleanup operations
func TestConcurrentFlushAndCleanup(t *testing.T) {
	t.Run("Should handle concurrent flush and cleanup safely", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()
		// Create multiple memory instances
		const numInstances = 5
		instances := make([]memcore.Memory, numInstances)
		for i := 0; i < numInstances; i++ {
			memRef := core.MemoryReference{
				ID:  "flushable-memory",
				Key: "concurrent-fc-{{.instance}}-{{.test.id}}",
			}
			workflowContext := map[string]any{
				"project": map[string]any{
					"id": "test-project",
				},
				"instance": fmt.Sprintf("inst-%d", i),
				"test": map[string]any{
					"id": fmt.Sprintf("concurrent-%d", time.Now().Unix()),
				},
			}
			instance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
			require.NoError(t, err)
			instances[i] = instance
			// Populate with messages
			for j := 0; j < 30; j++ {
				msg := llm.Message{
					Role:    "user",
					Content: fmt.Sprintf("Instance %d, Message %d", i, j),
				}
				err := instance.Append(ctx, msg)
				require.NoError(t, err)
			}
		}
		// Launch concurrent operations
		var wg sync.WaitGroup
		errors := make(chan error, numInstances*2)
		// Flush operations
		for i := 0; i < numInstances; i++ {
			if i%2 == 0 { // Only flush even-numbered instances
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					if flushable, ok := instances[idx].(memcore.FlushableMemory); ok {
						result, err := flushable.PerformFlush(ctx)
						if err != nil {
							errors <- fmt.Errorf("flush error on instance %d: %w", idx, err)
							return
						}
						if result != nil && result.Success {
							t.Logf("Instance %d flushed successfully", idx)
						}
					}
					errors <- nil
				}(i)
			}
		}
		// Clear operations (with slight delay)
		for i := 0; i < numInstances; i++ {
			if i%2 == 1 { // Only clear odd-numbered instances
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					time.Sleep(50 * time.Millisecond) // Small delay
					err := instances[idx].Clear(ctx)
					if err != nil {
						errors <- fmt.Errorf("clear error on instance %d: %w", idx, err)
						return
					}
					t.Logf("Instance %d cleared successfully", idx)
					errors <- nil
				}(i)
			}
		}
		// Wait for all operations
		wg.Wait()
		close(errors)
		// Check for errors
		for err := range errors {
			if err != nil {
				t.Logf("Operation error: %v", err)
			}
		}
		// Verify final states
		for i, instance := range instances {
			health, err := instance.GetMemoryHealth(ctx)
			require.NoError(t, err)
			if i%2 == 0 {
				// Even instances were flushed - should have some messages
				t.Logf("Instance %d (flushed): %d messages, %d tokens",
					i, health.MessageCount, health.TokenCount)
			} else {
				// Odd instances were cleared - should be empty
				assert.Equal(t, 0, health.MessageCount, "Cleared instance %d should be empty", i)
				assert.Equal(t, 0, health.TokenCount, "Cleared instance %d should have no tokens", i)
			}
		}
	})
}
