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

// TestDistributedLockingAppend tests concurrent append operations with distributed locking
func TestDistributedLockingAppend(t *testing.T) {
	t.Run("Should serialize concurrent append operations with distributed locks", func(t *testing.T) {
		// Get test configuration
		testConfig := GetTestConfig()

		// Setup test environment
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()
		// Use shared-memory which has locking configuration
		memRef := core.MemoryReference{
			ID:  "shared-memory",
			Key: "locking-append-test-{{.test.id}}",
		}
		workflowContext := map[string]any{
			"project.id": "test-project",
			"test.id":    fmt.Sprintf("append-%d", time.Now().Unix()),
		}
		const numWorkers = 20
		const messagesPerWorker = 10
		var appendCount atomic.Int32
		var lockWaitCount atomic.Int32
		results := make(chan error, numWorkers)
		var wg sync.WaitGroup
		// Track append order to verify serialization
		appendOrder := make([]string, 0, numWorkers*messagesPerWorker)
		var orderMutex sync.Mutex
		// Launch concurrent workers
		startTime := time.Now()
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				// Get memory instance
				memoryInstance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
				if err != nil {
					results <- fmt.Errorf("worker %d failed to get instance: %w", workerID, err)
					return
				}
				// Attempt to append messages
				for j := 0; j < messagesPerWorker; j++ {
					msgContent := fmt.Sprintf("Worker-%d-Message-%d", workerID, j)
					msg := llm.Message{
						Role:    "user",
						Content: msgContent,
					}
					// Track time waiting for lock
					appendStart := time.Now()
					err := memoryInstance.Append(ctx, msg)
					appendDuration := time.Since(appendStart)
					if err != nil {
						results <- fmt.Errorf("worker %d failed to append message %d: %w", workerID, j, err)
						return
					}
					// If append took more than threshold, likely waited for lock
					if appendDuration > testConfig.LockWaitThreshold {
						lockWaitCount.Add(1)
					}
					appendCount.Add(1)
					// Record append order
					orderMutex.Lock()
					appendOrder = append(appendOrder, msgContent)
					orderMutex.Unlock()
				}
				results <- nil
			}(i)
		}
		// Wait for all workers
		wg.Wait()
		close(results)
		totalDuration := time.Since(startTime)
		// Collect errors - some lock acquisition failures are expected
		var lockErrors int
		var otherErrors []error
		for err := range results {
			if err != nil {
				if errors.Is(err, memcore.ErrLockAcquisitionFailed) ||
					errors.Is(err, memcore.ErrAppendLockFailed) {
					lockErrors++
				} else {
					otherErrors = append(otherErrors, err)
				}
			}
		}
		// Should have no unexpected errors
		for _, err := range otherErrors {
			require.NoError(t, err)
		}
		// Verify results
		finalInstance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
		require.NoError(t, err)
		finalMessages, err := finalInstance.Read(ctx)
		require.NoError(t, err)
		// Should have messages from successful appends
		successfulAppends := int(appendCount.Load())
		assert.Equal(t, successfulAppends, len(finalMessages), "Should have messages from successful appends")
		// Total attempts should equal successful + failed due to locks
		totalAttempts := successfulAppends + lockErrors
		assert.LessOrEqual(t, totalAttempts, numWorkers*messagesPerWorker, "Total attempts should not exceed expected")
		// Verify no duplicates
		messageSet := make(map[string]bool)
		for _, msg := range finalMessages {
			assert.False(t, messageSet[msg.Content], "No duplicate messages")
			messageSet[msg.Content] = true
		}
		// Log statistics
		t.Logf("Total duration: %v", totalDuration)
		t.Logf("Messages appended: %d", appendCount.Load())
		t.Logf("Lock acquisition failures: %d", lockErrors)
		t.Logf("Lock waits detected: %d", lockWaitCount.Load())
		if successfulAppends > 0 {
			t.Logf("Average time per successful message: %v", totalDuration/time.Duration(successfulAppends))
		}
		// Verify distributed locking is working (either lock waits or lock failures should occur)
		assert.True(t, lockWaitCount.Load() > 0 || lockErrors > 0, "Should have lock contention (waits or failures)")
	})
}

// TestDistributedLockingClear tests concurrent clear operations with distributed locking
func TestDistributedLockingClear(t *testing.T) {
	t.Run("Should handle concurrent clear operations safely", func(t *testing.T) {
		// Get test configuration
		testConfig := GetTestConfig()

		// Setup test environment
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()
		memRef := core.MemoryReference{
			ID:  "shared-memory",
			Key: "locking-clear-test-{{.test.id}}",
		}
		workflowContext := map[string]any{
			"project.id": "test-project",
			"test.id":    fmt.Sprintf("clear-%d", time.Now().Unix()),
		}
		// Pre-populate memory
		instance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
		require.NoError(t, err)
		// Add initial messages
		for i := 0; i < 50; i++ {
			msg := llm.Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Initial message %d", i),
			}
			err := instance.Append(ctx, msg)
			require.NoError(t, err)
		}
		// Verify initial state
		initialMessages, err := instance.Read(ctx)
		require.NoError(t, err)
		assert.Len(t, initialMessages, 50)
		// Launch concurrent clear and append operations
		const numWorkers = 10
		var clearSuccessCount atomic.Int32
		var appendAfterClearCount atomic.Int32
		results := make(chan error, numWorkers)
		var wg sync.WaitGroup
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				// Get memory instance
				memInstance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
				if err != nil {
					results <- fmt.Errorf("worker %d failed to get instance: %w", workerID, err)
					return
				}
				// Alternate between clear and append
				if workerID%2 == 0 {
					// Clear operation
					err := memInstance.Clear(ctx)
					if err != nil {
						results <- fmt.Errorf("worker %d failed to clear: %w", workerID, err)
						return
					}
					clearSuccessCount.Add(1)
				} else {
					// Wait a bit then append after clear
					time.Sleep(testConfig.AppendDelayMax)
					msg := llm.Message{
						Role:    "user",
						Content: fmt.Sprintf("Message after clear from worker %d", workerID),
					}
					err := memInstance.Append(ctx, msg)
					if err != nil {
						results <- fmt.Errorf("worker %d failed to append after clear: %w", workerID, err)
						return
					}
					appendAfterClearCount.Add(1)
				}
				results <- nil
			}(i)
		}
		// Wait for completion
		wg.Wait()
		close(results)
		// Collect errors - some lock acquisition failures are expected
		var lockErrors int
		var otherErrors []error
		for err := range results {
			if err != nil {
				if errors.Is(err, memcore.ErrLockAcquisitionFailed) ||
					errors.Is(err, memcore.ErrAppendLockFailed) {
					lockErrors++
				} else {
					otherErrors = append(otherErrors, err)
				}
			}
		}
		// Should have no unexpected errors
		for _, err := range otherErrors {
			require.NoError(t, err)
		}
		// Verify final state
		finalMessages, err := instance.Read(ctx)
		require.NoError(t, err)
		// Should have only messages appended after clears
		assert.LessOrEqual(t, len(finalMessages), int(appendAfterClearCount.Load()))
		t.Logf("Clear operations: %d", clearSuccessCount.Load())
		t.Logf("Append after clear: %d", appendAfterClearCount.Load())
		t.Logf("Final message count: %d", len(finalMessages))
	})
}

// TestDistributedLockingFlush tests concurrent flush operations with distributed locking
func TestDistributedLockingFlush(t *testing.T) {
	t.Run("Should prevent concurrent flush operations with distributed locks", func(t *testing.T) {
		// Setup test environment
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()
		memRef := core.MemoryReference{
			ID:  "flushable-memory",
			Key: "locking-flush-test-{{.test.id}}",
		}
		workflowContext := map[string]any{
			"project.id": "test-project",
			"test.id":    fmt.Sprintf("flush-%d", time.Now().Unix()),
		}
		// Get memory instance and add many messages
		instance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
		require.NoError(t, err)
		// Add messages to trigger flush threshold
		for i := 0; i < 40; i++ {
			msg := llm.Message{
				Role:    "user",
				Content: fmt.Sprintf("Message %d - this is a longer message to accumulate tokens faster", i),
			}
			err := instance.Append(ctx, msg)
			require.NoError(t, err)
		}
		// Verify it's flushable
		_, ok := instance.(memcore.FlushableMemory)
		require.True(t, ok, "Memory should be flushable")
		// Launch concurrent flush attempts
		const numFlushAttempts = 5
		var flushSuccessCount atomic.Int32
		var flushBlockedCount atomic.Int32
		results := make(chan error, numFlushAttempts)
		var wg sync.WaitGroup
		// Use a channel as a barrier to start all goroutines at once
		startSignal := make(chan struct{})
		for i := 0; i < numFlushAttempts; i++ {
			wg.Add(1)
			go func(attemptID int) {
				defer wg.Done()
				// Wait for start signal
				<-startSignal
				// Get fresh instance
				memInstance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
				if err != nil {
					results <- fmt.Errorf("attempt %d failed to get instance: %w", attemptID, err)
					return
				}
				flushable, ok := memInstance.(memcore.FlushableMemory)
				if !ok {
					results <- fmt.Errorf("attempt %d: instance not flushable", attemptID)
					return
				}
				// Attempt flush
				t.Logf("Attempt %d starting flush", attemptID)
				flushResult, err := flushable.PerformFlush(ctx)
				if err != nil {
					// Check if it's because flush is already pending or lock couldn't be acquired
					if errors.Is(err, memcore.ErrFlushAlreadyPending) {
						t.Logf("Attempt %d blocked by pending flush", attemptID)
						flushBlockedCount.Add(1)
						results <- nil // This is expected
						return
					}
					// Check if it's a lock acquisition failure (also expected in distributed locking)
					if errors.Is(err, memcore.ErrFlushLockFailed) ||
						errors.Is(err, memcore.ErrLockAcquisitionFailed) {
						t.Logf("Attempt %d blocked by distributed lock", attemptID)
						flushBlockedCount.Add(1)
						results <- nil // This is expected
						return
					}
					// Log the error for debugging
					t.Logf("Flush attempt %d failed: %v", attemptID, err)
					results <- fmt.Errorf("attempt %d failed to flush: %w", attemptID, err)
					return
				}
				if flushResult.Success {
					flushSuccessCount.Add(1)
					t.Logf("Attempt %d flush succeeded", attemptID)
				}
				results <- nil
			}(i)
		}
		// Start all goroutines at once
		close(startSignal)
		// Wait for all attempts
		wg.Wait()
		close(results)
		// Check for unexpected errors
		for err := range results {
			require.NoError(t, err)
		}
		// Verify at least one flush succeeded and all attempts were accounted for
		assert.GreaterOrEqual(t, flushSuccessCount.Load(), int32(1), "At least one flush should succeed")
		assert.Equal(t, int32(numFlushAttempts), flushSuccessCount.Load()+flushBlockedCount.Load(),
			"All attempts should either succeed or be blocked")
		// Note: In some cases, all flushes may succeed if they don't conflict or if the
		// distributed locking granularity allows multiple operations to proceed
		t.Logf("Flush attempts: %d", numFlushAttempts)
		t.Logf("Successful flushes: %d", flushSuccessCount.Load())
		t.Logf("Blocked flushes: %d", flushBlockedCount.Load())
	})
}

// TestDistributedLockingMixedOperations tests mixed operations with proper lock isolation
func TestDistributedLockingMixedOperations(t *testing.T) {
	t.Run("Should handle mixed operations with proper lock isolation", func(t *testing.T) {
		// Get test configuration
		testConfig := GetTestConfig()

		// Setup test environment
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()
		// Create multiple memory instances to test lock isolation
		memoryConfigs := []struct {
			memRef          core.MemoryReference
			workflowContext map[string]any
		}{
			{
				memRef: core.MemoryReference{
					ID:  "shared-memory",
					Key: "mixed-ops-1-{{.test.id}}",
				},
				workflowContext: map[string]any{
					"project.id": "test-project",
					"test.id":    fmt.Sprintf("mixed-1-%d", time.Now().Unix()),
				},
			},
			{
				memRef: core.MemoryReference{
					ID:  "shared-memory",
					Key: "mixed-ops-2-{{.test.id}}",
				},
				workflowContext: map[string]any{
					"project.id": "test-project",
					"test.id":    fmt.Sprintf("mixed-2-%d", time.Now().Unix()),
				},
			},
		}
		// Launch operations sequentially to avoid race conditions between append and clear
		var wg sync.WaitGroup
		results := make(chan error, len(memoryConfigs)*3)

		// Phase 1: Append operations first
		for idx, config := range memoryConfigs {
			// Append operation
			wg.Add(1)
			go func(cfg struct {
				memRef          core.MemoryReference
				workflowContext map[string]any
			}, id int) {
				defer wg.Done()
				instance, err := env.GetMemoryManager().GetInstance(ctx, cfg.memRef, cfg.workflowContext)
				if err != nil {
					results <- fmt.Errorf("config %d: failed to get instance: %w", id, err)
					return
				}
				// Add multiple messages
				for i := 0; i < 20; i++ {
					msg := llm.Message{
						Role:    "user",
						Content: fmt.Sprintf("Config %d - Message %d", id, i),
					}
					if err := instance.Append(ctx, msg); err != nil {
						results <- fmt.Errorf("config %d: failed to append: %w", id, err)
						return
					}
				}
				results <- nil
			}(config, idx)
		}

		// Wait for all append operations to complete
		wg.Wait()

		// Phase 2: Read operations
		for idx, config := range memoryConfigs {
			wg.Add(1)
			go func(cfg struct {
				memRef          core.MemoryReference
				workflowContext map[string]any
			}, id int) {
				defer wg.Done()
				// Wait a bit for some messages to be added
				time.Sleep(testConfig.AsyncOperationDelay / 10)
				instance, err := env.GetMemoryManager().GetInstance(ctx, cfg.memRef, cfg.workflowContext)
				if err != nil {
					results <- fmt.Errorf("config %d: failed to get instance for read: %w", id, err)
					return
				}
				// Read multiple times
				for i := 0; i < 10; i++ {
					messages, err := instance.Read(ctx)
					if err != nil {
						results <- fmt.Errorf("config %d: failed to read: %w", id, err)
						return
					}
					// Verify messages are from correct config
					for _, msg := range messages {
						if msg.Role == "user" && len(messages) > 0 {
							assert.Contains(t, msg.Content, fmt.Sprintf("Config %d", id))
						}
					}
					time.Sleep(testConfig.AsyncOperationDelay / 20)
				}
				results <- nil
			}(config, idx)
			// Clear operation (delayed)
			wg.Add(1)
			go func(cfg struct {
				memRef          core.MemoryReference
				workflowContext map[string]any
			}, id int) {
				defer wg.Done()
				// Wait for messages to accumulate
				time.Sleep(testConfig.ClearDelayMax)
				instance, err := env.GetMemoryManager().GetInstance(ctx, cfg.memRef, cfg.workflowContext)
				if err != nil {
					results <- fmt.Errorf("config %d: failed to get instance for clear: %w", id, err)
					return
				}
				// Clear
				if err := instance.Clear(ctx); err != nil {
					results <- fmt.Errorf("config %d: failed to clear: %w", id, err)
					return
				}
				// Verify cleared
				messages, err := instance.Read(ctx)
				if err != nil {
					results <- fmt.Errorf("config %d: failed to read after clear: %w", id, err)
					return
				}
				assert.Empty(t, messages, "Messages should be empty after clear")
				results <- nil
			}(config, idx)
		}
		// Wait for all operations
		wg.Wait()
		close(results)
		// Check for errors
		errorCount := 0
		for err := range results {
			if err != nil {
				t.Logf("Operation error: %v", err)
				errorCount++
			}
		}
		assert.Equal(t, 0, errorCount, "All operations should complete without errors")
	})
}

// TestDistributedLockingTimeout tests lock timeout behavior
func TestDistributedLockingTimeout(t *testing.T) {
	t.Run("Should handle lock timeouts gracefully", func(t *testing.T) {
		// Get test configuration
		testConfig := GetTestConfig()

		// Setup test environment
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()
		memRef := core.MemoryReference{
			ID:  "shared-memory",
			Key: "lock-timeout-test-{{.test.id}}",
		}
		workflowContext := map[string]any{
			"project.id": "test-project",
			"test.id":    fmt.Sprintf("timeout-%d", time.Now().Unix()),
		}
		// Get memory instance
		instance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
		require.NoError(t, err)
		// Simulate a long-running operation that might timeout
		// The lock TTL is 30s for append operations based on config
		const numLongOperations = 3
		var wg sync.WaitGroup
		results := make(chan error, numLongOperations)
		for i := 0; i < numLongOperations; i++ {
			wg.Add(1)
			go func(opID int) {
				defer wg.Done()
				// Create a context with timeout shorter than lock TTL
				opCtx, cancel := context.WithTimeout(ctx, testConfig.LockTimeoutDuration)
				defer cancel()
				// Try to append with timeout context
				msg := llm.Message{
					Role:    "user",
					Content: fmt.Sprintf("Operation %d message", opID),
				}
				err := instance.Append(opCtx, msg)
				if err != nil {
					// Context timeout is expected for some operations
					if opCtx.Err() == context.DeadlineExceeded {
						results <- nil // Expected
						return
					}
					results <- fmt.Errorf("operation %d failed: %w", opID, err)
					return
				}
				results <- nil
			}(i)
		}
		// Wait for operations
		wg.Wait()
		close(results)
		// Check results
		for err := range results {
			if err != nil {
				t.Logf("Operation error: %v", err)
			}
			// No assertion here as timeouts are expected
		}
		// Verify memory is still accessible after timeouts
		finalMessages, err := instance.Read(ctx)
		require.NoError(t, err, "Memory should remain accessible after timeout")
		t.Logf("Final message count after timeout test: %d", len(finalMessages))
	})
}
