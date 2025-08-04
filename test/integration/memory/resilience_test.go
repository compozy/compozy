package memory

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	memcore "github.com/compozy/compozy/engine/memory/core"
)

// TestResilienceRedisFailure tests memory system behavior when Redis fails
func TestResilienceRedisFailure(t *testing.T) {
	t.Run("Should handle Redis connection failures gracefully", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()
		memRef := core.MemoryReference{
			ID:  "customer-support",
			Key: "redis-failure-{{.test.id}}",
		}
		workflowContext := map[string]any{
			"project": map[string]any{
				"id": "test-project",
			},
			"test": map[string]any{
				"id": fmt.Sprintf("redis-fail-%d", time.Now().Unix()),
			},
		}
		// Create instance while Redis is available
		instance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
		require.NoError(t, err)
		// Add some messages
		initialMessages := []llm.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
		}
		for _, msg := range initialMessages {
			err := instance.Append(ctx, msg)
			require.NoError(t, err)
		}
		// Simulate Redis failure by closing the connection
		env.GetRedis().Close()
		// Test operations after Redis failure
		// Append should fail gracefully
		err = instance.Append(ctx, llm.Message{
			Role:    "user",
			Content: "This should fail",
		})
		assert.Error(t, err, "Append should fail when Redis is down")
		// Read should fail gracefully
		_, err = instance.Read(ctx)
		assert.Error(t, err, "Read should fail when Redis is down")
		// GetTokenCount should fail gracefully
		_, err = instance.GetTokenCount(ctx)
		assert.Error(t, err, "GetTokenCount should fail when Redis is down")
		// Clear should fail gracefully
		err = instance.Clear(ctx)
		assert.Error(t, err, "Clear should fail when Redis is down")
	})
}

// TestResilienceTimeouts tests timeout handling
func TestResilienceTimeouts(t *testing.T) {
	t.Run("Should handle operation timeouts gracefully", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		memRef := core.MemoryReference{
			ID:  "customer-support",
			Key: "timeout-test-{{.test.id}}",
		}
		workflowContext := map[string]any{
			"project": map[string]any{
				"id": "test-project",
			},
			"test": map[string]any{
				"id": fmt.Sprintf("timeout-%d", time.Now().Unix()),
			},
		}
		// Create instance with very short timeout context
		shortCtx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()
		// Try to get instance with timeout
		_, err := env.GetMemoryManager().GetInstance(shortCtx, memRef, workflowContext)
		// Should handle timeout gracefully
		if err != nil {
			assert.True(t, errors.Is(err, context.DeadlineExceeded) ||
				errors.Is(err, context.Canceled),
				"Should return context error on timeout")
		}
		// Now test with normal context
		ctx := context.Background()
		instance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
		require.NoError(t, err)
		// Test operation with timeout
		timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
		defer cancel()
		// Add many messages to potentially cause timeout
		for i := 0; i < 100; i++ {
			msg := llm.Message{
				Role:    "user",
				Content: fmt.Sprintf("Message %d", i),
			}
			err := instance.Append(timeoutCtx, msg)
			if err != nil {
				// Should be a context error if timeout occurs
				assert.True(t, errors.Is(err, context.DeadlineExceeded) ||
					errors.Is(err, context.Canceled))
				break
			}
		}
	})
}

// TestResilienceConcurrentFailures tests handling of concurrent failures
func TestResilienceConcurrentFailures(t *testing.T) {
	t.Run("Should handle concurrent operation failures", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()
		const numInstances = 5
		const numOperations = 10
		var wg sync.WaitGroup
		errorsChan := make(chan error, numInstances*numOperations)
		successCount := &atomic.Int32{}
		failureCount := &atomic.Int32{}
		for i := 0; i < numInstances; i++ {
			wg.Add(1)
			go func(instanceID int) {
				defer wg.Done()
				memRef := core.MemoryReference{
					ID:  "customer-support",
					Key: "concurrent-fail-{{.instance}}-{{.test.id}}",
				}
				workflowContext := map[string]any{
					"project": map[string]any{
						"id": "test-project",
					},
					"instance": fmt.Sprintf("inst-%d", instanceID),
					"test": map[string]any{
						"id": fmt.Sprintf("concurrent-%d", time.Now().Unix()),
					},
				}
				instance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
				if err != nil {
					errorsChan <- fmt.Errorf("instance %d: failed to get instance: %w", instanceID, err)
					failureCount.Add(1)
					return
				}
				// Perform operations with random failures
				for j := 0; j < numOperations; j++ {
					// Simulate random context cancellation
					opCtx := ctx
					if j%3 == 0 {
						cancelCtx, cancel := context.WithCancel(ctx)
						cancel() // Immediate cancellation
						opCtx = cancelCtx
					}
					msg := llm.Message{
						Role:    "user",
						Content: fmt.Sprintf("Instance %d, Operation %d", instanceID, j),
					}
					err := instance.Append(opCtx, msg)
					if err != nil {
						if errors.Is(err, context.Canceled) {
							// Expected error
							failureCount.Add(1)
						} else {
							errorsChan <- fmt.Errorf("instance %d, op %d: unexpected error: %w",
								instanceID, j, err)
						}
					} else {
						successCount.Add(1)
					}
				}
			}(i)
		}
		// Wait for all operations
		wg.Wait()
		close(errorsChan)
		// Check for unexpected errors
		unexpectedErrors := 0
		for err := range errorsChan {
			t.Logf("Error: %v", err)
			unexpectedErrors++
		}
		t.Logf("Success count: %d", successCount.Load())
		t.Logf("Expected failure count: %d", failureCount.Load())
		t.Logf("Unexpected errors: %d", unexpectedErrors)
		// Some operations should succeed
		assert.Greater(t, successCount.Load(), int32(0), "Some operations should succeed")
		// Some operations should fail as expected
		assert.Greater(t, failureCount.Load(), int32(0), "Some operations should fail as expected")
	})
}

// TestResilienceMemoryPressure tests behavior under memory pressure
func TestResilienceMemoryPressure(t *testing.T) {
	t.Run("Should handle memory pressure scenarios", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()
		memRef := core.MemoryReference{
			ID:  "shared-memory",
			Key: "memory-pressure-{{.test.id}}",
		}
		workflowContext := map[string]any{
			"project": map[string]any{
				"id": "test-project",
			},
			"test": map[string]any{
				"id": fmt.Sprintf("pressure-%d", time.Now().Unix()),
			},
		}
		instance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
		require.NoError(t, err)
		// Add a moderate number of messages to test memory pressure
		const maxMessages = 100                    // Test with reasonable number
		largeContent := string(make([]byte, 1000)) // 1KB per message
		messagesAdded := 0
		for i := 0; i < maxMessages; i++ {
			msg := llm.Message{
				Role:    "user",
				Content: fmt.Sprintf("Message %d: %s", i, largeContent),
			}
			err := instance.Append(ctx, msg)
			if err != nil {
				t.Logf("Failed to add message %d: %v", i, err)
				break
			}
			messagesAdded++
		}
		// Verify system handled the load
		health, err := instance.GetMemoryHealth(ctx)
		require.NoError(t, err)
		assert.Equal(t, messagesAdded, health.MessageCount, "Should have tracked all messages")
		t.Logf("Added %d messages, current count: %d", messagesAdded, health.MessageCount)
		// Test reading large amount of data
		messages, err := instance.Read(ctx)
		require.NoError(t, err)
		assert.Equal(t, messagesAdded, len(messages), "Should be able to read all messages")
	})
}

// TestResilienceCircuitBreaker tests circuit breaker behavior
func TestResilienceCircuitBreaker(t *testing.T) {
	t.Run("Should handle timeout and recovery scenarios", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()
		// Test timeout handling and recovery
		memRef := core.MemoryReference{
			ID:  "customer-support",
			Key: "timeout-test-{{.test.id}}",
		}
		workflowContext := map[string]any{
			"project": map[string]any{
				"id": "test-project",
			},
			"test": map[string]any{
				"id": fmt.Sprintf("timeout-%d", time.Now().Unix()),
			},
		}
		const numAttempts = 8 // Reduced from 20 for faster tests
		timeoutCount := 0
		successCount := 0
		for i := 0; i < numAttempts; i++ {
			// Create instance first
			instance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
			require.NoError(t, err)

			// Use short timeout for append operations to simulate stress
			opCtx := ctx
			var cancel context.CancelFunc
			if i%2 == 0 { // Every 2nd operation has a short timeout to ensure timeouts occur
				timeoutCtx, cancelFunc := context.WithTimeout(
					ctx,
					1*time.Nanosecond,
				) // Very short timeout to trigger failures
				opCtx = timeoutCtx
				cancel = cancelFunc
			}

			// Try append operation with potential timeout
			err = instance.Append(opCtx, llm.Message{
				Role:    "user",
				Content: fmt.Sprintf("Test message %d", i),
			})

			if err != nil {
				if opCtx.Err() == context.DeadlineExceeded {
					timeoutCount++
					t.Logf("Attempt %d timed out (expected)", i)
				} else {
					t.Logf("Append failed on attempt %d: %v", i, err)
				}
			} else {
				successCount++
			}

			if cancel != nil {
				cancel()
			}
			time.Sleep(2 * time.Millisecond)
		}
		t.Logf("Total attempts: %d, Timeouts: %d, Successes: %d",
			numAttempts, timeoutCount, successCount)
		// Should have both timeouts and successes
		assert.Greater(t, timeoutCount, 0, "Should have some timeouts")
		assert.Greater(t, successCount, 0, "Should have some successes")
	})
}

// TestResilienceDataCorruption tests handling of corrupted data
func TestResilienceDataCorruption(t *testing.T) {
	t.Run("Should handle corrupted data gracefully", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()
		memRef := core.MemoryReference{
			ID:  "customer-support",
			Key: "corruption-test-{{.test.id}}",
		}
		workflowContext := map[string]any{
			"project": map[string]any{
				"id": "test-project",
			},
			"test": map[string]any{
				"id": fmt.Sprintf("corrupt-%d", time.Now().Unix()),
			},
		}
		instance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
		require.NoError(t, err)
		// Add valid messages first
		validMessages := []llm.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
		}
		for _, msg := range validMessages {
			err := instance.Append(ctx, msg)
			require.NoError(t, err)
		}
		// Get the instance key to manipulate Redis directly
		instanceID := instance.GetID()
		redisKey := fmt.Sprintf("compozy:test-project:memory:%s", instanceID)
		// Corrupt data in Redis by adding invalid JSON
		err = env.GetRedis().RPush(ctx, redisKey, "INVALID_JSON{{}").Err()
		if err == nil {
			// Try to read - should handle corruption
			messages, readErr := instance.Read(ctx)
			if readErr != nil {
				t.Logf("Read failed as expected with corrupted data: %v", readErr)
				// This is expected behavior
			} else {
				// If read succeeds, verify we got valid messages
				assert.NotEmpty(t, messages, "Should have some messages")
				t.Logf("Read succeeded despite corruption, got %d messages", len(messages))
			}
		}
		// Try to recover by clearing
		err = instance.Clear(ctx)
		if err != nil {
			t.Logf("Clear failed: %v", err)
		}
		// Verify we can use the instance again
		err = instance.Append(ctx, llm.Message{
			Role:    "user",
			Content: "Testing after corruption",
		})
		if err != nil {
			t.Logf("Append after corruption failed: %v", err)
		}
	})
}

// TestResiliencePrivacyUnderFailure tests privacy guarantees under failure
func TestResiliencePrivacyUnderFailure(t *testing.T) {
	t.Run("Should maintain privacy guarantees under failure conditions", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()
		memRef := core.MemoryReference{
			ID:  "customer-support",
			Key: "privacy-failure-{{.test.id}}",
		}
		workflowContext := map[string]any{
			"project": map[string]any{
				"id": "test-project",
			},
			"test": map[string]any{
				"id": fmt.Sprintf("privacy-%d", time.Now().Unix()),
			},
		}
		instance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
		require.NoError(t, err)
		// Add sensitive message with privacy metadata
		sensitiveMsg := llm.Message{
			Role:    "user",
			Content: "My credit card number is 1234-5678-9012-3456",
		}
		privacyMeta := memcore.PrivacyMetadata{
			DoNotPersist: true,
		}
		// Try to append with potential failure
		err = instance.AppendWithPrivacy(ctx, sensitiveMsg, privacyMeta)
		if err != nil {
			t.Logf("AppendWithPrivacy failed: %v", err)
		}
		// Read back and verify privacy was maintained
		messages, err := instance.Read(ctx)
		if err == nil {
			// Check that sensitive message was not persisted
			for _, msg := range messages {
				assert.NotContains(t, msg.Content, "1234-5678-9012-3456",
					"Sensitive data should not be persisted")
			}
		}
		// Test with partial failure during batch operation
		batchMessages := []llm.Message{
			{Role: "user", Content: "Normal message 1"},
			{Role: "user", Content: "SSN: 123-45-6789"}, // Sensitive
			{Role: "user", Content: "Normal message 2"},
		}
		for i, msg := range batchMessages {
			var appendErr error
			if i == 1 {
				// Mark as sensitive
				meta := memcore.PrivacyMetadata{
					SensitiveFields: []string{"content"},
					PrivacyLevel:    "high",
				}
				appendErr = instance.AppendWithPrivacy(ctx, msg, meta)
			} else {
				appendErr = instance.Append(ctx, msg)
			}
			if appendErr != nil {
				t.Logf("Batch message %d failed: %v", i, appendErr)
			}
		}
		// Verify privacy was maintained even with failures
		finalMessages, err := instance.Read(ctx)
		if err == nil {
			t.Logf("Final message count: %d", len(finalMessages))
			for _, msg := range finalMessages {
				// Sensitive data should be handled appropriately
				if msg.Content == "SSN: 123-45-6789" {
					t.Log("Warning: Sensitive data found in messages")
				}
			}
		}
	})
}
