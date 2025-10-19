package memory

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	memcore "github.com/compozy/compozy/engine/memory/core"
)

// TestCompleteMemoryLifecycle tests the full lifecycle of a memory instance
func TestCompleteMemoryLifecycle(t *testing.T) {
	t.Run("Should complete full memory lifecycle with Temporal integration", func(t *testing.T) {
		// Setup test environment
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := t.Context()
		// Step 1: Create memory instance through manager
		memRef := core.MemoryReference{
			ID:  "customer-support",
			Key: "support-conversation-{{.conversation.id}}",
		}
		workflowContext := map[string]any{
			"project":      map[string]any{"id": "test-project"},
			"conversation": map[string]any{"id": "123"},
			"user":         map[string]any{"id": "user-456"},
		}
		memoryInstance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
		require.NoError(t, err)
		require.NotNil(t, memoryInstance)
		// Step 2: Add messages to memory
		messages := []llm.Message{
			{Role: "system", Content: "You are a helpful customer support agent."},
			{Role: "user", Content: "I need help with my account."},
			{
				Role:    "assistant",
				Content: "I'd be happy to help with your account. What specific issue are you experiencing?",
			},
		}
		for _, msg := range messages {
			err := memoryInstance.Append(ctx, msg)
			require.NoError(t, err)
		}
		// Step 3: Read messages back
		retrievedMessages, err := memoryInstance.Read(ctx)
		require.NoError(t, err)
		assert.Len(t, retrievedMessages, len(messages))
		// Verify message content
		for i, msg := range retrievedMessages {
			assert.Equal(t, messages[i].Role, msg.Role)
			assert.Equal(t, messages[i].Content, msg.Content)
		}
		// Step 4: Check token count
		tokenCount, err := memoryInstance.GetTokenCount(ctx)
		require.NoError(t, err)
		assert.Greater(t, tokenCount, 0)
		// Step 5: Check memory health
		health, err := memoryInstance.GetMemoryHealth(ctx)
		require.NoError(t, err)
		assert.Equal(t, len(messages), health.MessageCount)
		assert.Equal(t, tokenCount, health.TokenCount)
		// Step 6: Clear memory
		err = memoryInstance.Clear(ctx)
		require.NoError(t, err)
		// Verify empty
		finalMessages, err := memoryInstance.Read(ctx)
		require.NoError(t, err)
		assert.Empty(t, finalMessages)
		// Verify health after clear
		healthAfterClear, err := memoryInstance.GetMemoryHealth(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, healthAfterClear.MessageCount)
		assert.Equal(t, 0, healthAfterClear.TokenCount)
	})
}

// TestConcurrentAgentAccess tests concurrent access to shared memory
func TestConcurrentAgentAccess(t *testing.T) {
	t.Run("Should handle concurrent agent memory access", func(t *testing.T) {
		// Setup test environment
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := t.Context()
		memRef := core.MemoryReference{
			ID:  "shared-memory",
			Key: "concurrent-test-{{.session.id}}",
		}
		workflowContext := map[string]any{
			"project": map[string]any{"id": "test-project"},
			"session": map[string]any{"id": "concurrent-session"},
		}
		const numWorkers = 5
		const messagesPerWorker = 10
		results := make(chan error, numWorkers)
		var wg sync.WaitGroup
		// Launch concurrent workers
		for i := range numWorkers {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						results <- fmt.Errorf("worker %d panicked: %v", workerID, r)
						return
					}
				}()
				// Stagger worker start to reduce initial contention
				time.Sleep(time.Duration(workerID) * 10 * time.Millisecond)
				// Each worker gets its own memory instance
				memoryInstance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
				if err != nil {
					results <- fmt.Errorf("worker %d failed to get instance: %w", workerID, err)
					return
				}
				// Add messages concurrently with retry logic for lock contention
				successfulAppends := 0
				for j := range messagesPerWorker {
					msg := llm.Message{
						Role:    "user",
						Content: fmt.Sprintf("Message from worker %d, iteration %d", workerID, j),
					}
					err := memoryInstance.Append(ctx, msg)
					if err != nil {
						// Handle lock acquisition failures gracefully (expected in high contention)
						if strings.Contains(err.Error(), "lock could not be acquired") ||
							strings.Contains(err.Error(), "failed to acquire") {
							t.Logf(
								"Worker %d: Lock contention on message %d (expected in concurrent test)",
								workerID,
								j,
							)
							// Small delay before retry
							time.Sleep(time.Millisecond * 50)
							continue
						}
						results <- fmt.Errorf("worker %d failed to append: %w", workerID, err)
						return
					}
					successfulAppends++
					// Small delay to reduce lock contention
					time.Sleep(time.Millisecond * 20)
				}
				t.Logf("Worker %d successfully appended %d/%d messages", workerID, successfulAppends, messagesPerWorker)
				results <- nil
			}(i)
		}
		// Wait for all workers to complete
		wg.Wait()
		close(results)
		// Check results
		for err := range results {
			require.NoError(t, err)
		}
		// Verify final state
		finalInstance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
		require.NoError(t, err)
		finalMessages, err := finalInstance.Read(ctx)
		require.NoError(t, err)
		// Should have messages from workers (some may have been skipped due to lock contention)
		expectedMessageCount := numWorkers * messagesPerWorker
		actualMessageCount := len(finalMessages)
		assert.GreaterOrEqual(t, actualMessageCount, expectedMessageCount/2,
			"Should have at least half the expected messages due to potential lock contention")
		assert.LessOrEqual(t, actualMessageCount, expectedMessageCount,
			"Should not exceed expected message count")
		t.Logf("Successfully stored %d/%d messages", actualMessageCount, expectedMessageCount)
		// Verify no duplicate or corrupted messages
		messageContents := make(map[string]bool)
		for _, msg := range finalMessages {
			assert.False(t, messageContents[msg.Content], "Duplicate message found: %s", msg.Content)
			messageContents[msg.Content] = true
		}
	})
}

// TestFlushWorkflow tests the flush functionality
func TestFlushWorkflow(t *testing.T) {
	t.Run("Should execute flush workflow properly", func(t *testing.T) {
		// Setup test environment
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := t.Context()
		// Create memory instance with flush configuration
		memRef := core.MemoryReference{
			ID:  "flushable-memory",
			Key: "flush-test-{{.test.id}}",
		}
		workflowContext := map[string]any{
			"project": map[string]any{"id": "test-project"},
			"test":    map[string]any{"id": "flush-workflow"},
		}
		memoryInstance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
		require.NoError(t, err)
		// Add many messages to trigger flush
		numMessages := 50
		for i := range numMessages {
			msg := llm.Message{
				Role: "user",
				Content: fmt.Sprintf(
					"Message %d - this is a longer message to accumulate tokens and trigger flush behavior",
					i,
				),
			}
			err := memoryInstance.Append(ctx, msg)
			require.NoError(t, err)
		}
		// Check if memory is flushable
		flushableMemory, ok := memoryInstance.(memcore.FlushableMemory)
		require.True(t, ok, "Memory instance should implement FlushableMemory interface")
		// Get health before flush
		healthBeforeFlush, err := memoryInstance.GetMemoryHealth(ctx)
		require.NoError(t, err)
		assert.Equal(t, numMessages, healthBeforeFlush.MessageCount)
		// Execute flush
		flushResult, err := flushableMemory.PerformFlush(ctx)
		require.NoError(t, err)
		require.NotNil(t, flushResult)
		assert.True(t, flushResult.Success)
		assert.Greater(t, flushResult.MessageCount, 0)
		assert.Greater(t, flushResult.TokenCount, 0)
		// Verify memory state after flush
		postFlushMessages, err := memoryInstance.Read(ctx)
		require.NoError(t, err)
		// Should have fewer messages after flush
		assert.Less(t, len(postFlushMessages), numMessages)
		// Verify health after flush
		healthAfterFlush, err := memoryInstance.GetMemoryHealth(ctx)
		require.NoError(t, err)
		assert.Equal(t, len(postFlushMessages), healthAfterFlush.MessageCount)
		assert.Less(t, healthAfterFlush.TokenCount, healthBeforeFlush.TokenCount)
	})
}

// TestMemoryWithPrivacy tests privacy-aware memory operations
func TestMemoryWithPrivacy(t *testing.T) {
	t.Run("Should handle privacy metadata correctly", func(t *testing.T) {
		// Setup test environment
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := t.Context()
		memRef := core.MemoryReference{
			ID:  "customer-support",
			Key: "privacy-test-{{.test.id}}",
		}
		workflowContext := map[string]any{
			"project": map[string]any{"id": "test-project"},
			"test":    map[string]any{"id": "privacy-test"},
		}
		memoryInstance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
		require.NoError(t, err)
		// Test regular append
		regularMsg := llm.Message{
			Role:    "user",
			Content: "This is a regular message",
		}
		err = memoryInstance.Append(ctx, regularMsg)
		require.NoError(t, err)
		// Test append with privacy metadata
		sensitiveMsg := llm.Message{
			Role:    "user",
			Content: "My SSN is 123-45-6789",
		}
		privacyMetadata := memcore.PrivacyMetadata{
			DoNotPersist:    false,
			SensitiveFields: []string{"content"},
			PrivacyLevel:    "confidential",
		}
		err = memoryInstance.AppendWithPrivacy(ctx, sensitiveMsg, privacyMetadata)
		require.NoError(t, err)
		// Test do-not-persist
		ephemeralMsg := llm.Message{
			Role:    "system",
			Content: "This should not be persisted",
		}
		ephemeralMetadata := memcore.PrivacyMetadata{
			DoNotPersist: true,
		}
		err = memoryInstance.AppendWithPrivacy(ctx, ephemeralMsg, ephemeralMetadata)
		require.NoError(t, err)
		// Read messages and verify
		messages, err := memoryInstance.Read(ctx)
		require.NoError(t, err)
		// Should have 2 messages (ephemeral one should not be persisted)
		assert.Len(t, messages, 2)
		// Verify regular message
		assert.Equal(t, regularMsg.Content, messages[0].Content)
		// The sensitive message content might be redacted based on privacy policy
		// For now, we just verify it exists
		assert.NotEmpty(t, messages[1].Content)
	})
}

// TestMemoryExpiration tests TTL and expiration behavior
func TestMemoryExpiration(t *testing.T) {
	t.Run("Should handle memory expiration correctly", func(t *testing.T) {
		// Setup test environment
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := t.Context()
		// This test uses a shorter TTL for testing
		// Note: In a real test environment, we might need to configure
		// a special memory resource with a very short TTL
		memRef := core.MemoryReference{
			ID:  "customer-support",
			Key: "expiration-test-{{.test.id}}",
		}
		workflowContext := map[string]any{
			"project": map[string]any{"id": "test-project"},
			"test":    map[string]any{"id": fmt.Sprintf("expiration-%d", time.Now().Unix())},
		}
		memoryInstance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
		require.NoError(t, err)
		// Add a message
		msg := llm.Message{
			Role:    "user",
			Content: "This message has a TTL",
		}
		err = memoryInstance.Append(ctx, msg)
		require.NoError(t, err)
		// Verify message exists
		messages, err := memoryInstance.Read(ctx)
		require.NoError(t, err)
		assert.Len(t, messages, 1)
		// Get instance ID for direct Redis operations
		instanceID := memoryInstance.GetID()
		// Check TTL exists on the key
		keyToCheck := fmt.Sprintf("compozy:test-project:memory:%s", instanceID)
		ttl, err := env.GetRedis().TTL(ctx, keyToCheck).Result()
		require.NoError(t, err)
		assert.Greater(t, ttl.Seconds(), float64(0), "Key should have a TTL set")
	})
}
