---
status: pending
---

<task_context>
<domain>engine/memory</domain>
<type>testing</type>
<scope>integration</scope>
<complexity>high</complexity>
<dependencies>temporal</dependencies>
</task_context>

# Task 27.0: End-to-End Integration Tests

## Overview

Create comprehensive end-to-end integration tests that validate the complete memory system functionality including Temporal workflow integration, concurrent access scenarios, flush and cleanup workflows, and performance benchmarks.

## Subtasks

- [ ] 27.1 Create E2E test with Temporal workflow integration
- [ ] 27.2 Test concurrent memory access scenarios with distributed locking
- [ ] 27.3 Test flush and cleanup workflows end-to-end
- [ ] 27.4 Add performance benchmarks for memory operations
- [ ] 27.5 Test multi-provider token counting integration
- [ ] 27.6 Test resilience patterns under failure conditions
- [ ] 27.7 **NEW**: Set up stable test environment with mock services
- [ ] 27.8 **NEW**: Create test data management and cleanup procedures
- [ ] 27.9 **NEW**: Implement test environment health monitoring
- [ ] 27.10 **NEW**: Add test suite maintenance and debugging tools

## Implementation Details

### E2E Test with Temporal Integration

```go
// test/integration/memory/e2e_test.go
package memory_test

import (
    "context"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "go.temporal.io/sdk/testsuite"

    "github.com/compozy/compozy/engine/memory"
    "github.com/compozy/compozy/engine/memory/core"
    "github.com/compozy/compozy/engine/llm"
)

type E2ETestSuite struct {
    testsuite.WorkflowTestSuite
    memoryManager *memory.Manager
    redis         *redis.Client
}

func TestE2EMemoryWorkflow(t *testing.T) {
    suite := &E2ETestSuite{}
    suite.SetLogger(t)

    // Setup test environment
    ctx := context.Background()
    suite.setupTestEnvironment(ctx, t)
    defer suite.cleanupTestEnvironment(ctx)

    t.Run("Should complete full memory lifecycle with Temporal", func(t *testing.T) {
        suite.testCompleteMemoryLifecycle(t)
    })

    t.Run("Should handle concurrent agent memory access", func(t *testing.T) {
        suite.testConcurrentAgentAccess(t)
    })

    t.Run("Should execute flush workflow properly", func(t *testing.T) {
        suite.testFlushWorkflow(t)
    })
}

func (s *E2ETestSuite) testCompleteMemoryLifecycle(t *testing.T) {
    ctx := context.Background()

    // Step 1: Create memory instance through manager
    memRef := core.MemoryReference{
        ID:  "customer-support",
        Key: "support-conversation-123",
    }

    workflowContext := map[string]any{
        "project.id":         "test-project",
        "conversation.id":    "123",
        "user.id":           "user-456",
    }

    memoryInstance, err := s.memoryManager.GetInstance(ctx, memRef, workflowContext)
    require.NoError(t, err)
    require.NotNil(t, memoryInstance)

    // Step 2: Add messages to memory
    messages := []llm.Message{
        {Role: "system", Content: "You are a helpful customer support agent."},
        {Role: "user", Content: "I need help with my account."},
        {Role: "assistant", Content: "I'd be happy to help with your account. What specific issue are you experiencing?"},
    }

    for _, msg := range messages {
        err := memoryInstance.Append(ctx, msg)
        require.NoError(t, err)
    }

    // Step 3: Read messages back
    retrievedMessages, err := memoryInstance.Read(ctx)
    require.NoError(t, err)
    assert.Len(t, retrievedMessages, len(messages))

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
}
```

### Concurrent Access Tests

```go
func (s *E2ETestSuite) testConcurrentAgentAccess(t *testing.T) {
    ctx := context.Background()

    memRef := core.MemoryReference{
        ID:  "shared-memory",
        Key: "concurrent-test-{{.session.id}}",
    }

    workflowContext := map[string]any{
        "project.id": "test-project",
        "session.id": "concurrent-session",
    }

    const numWorkers = 10
    const messagesPerWorker = 5

    results := make(chan error, numWorkers)

    // Launch concurrent workers
    for i := 0; i < numWorkers; i++ {
        go func(workerID int) {
            defer func() {
                if r := recover(); r != nil {
                    results <- fmt.Errorf("worker %d panicked: %v", workerID, r)
                    return
                }
            }()

            // Each worker gets its own memory instance
            memoryInstance, err := s.memoryManager.GetInstance(ctx, memRef, workflowContext)
            if err != nil {
                results <- fmt.Errorf("worker %d failed to get instance: %w", workerID, err)
                return
            }

            // Add messages concurrently
            for j := 0; j < messagesPerWorker; j++ {
                msg := llm.Message{
                    Role:    "user",
                    Content: fmt.Sprintf("Message from worker %d, iteration %d", workerID, j),
                }

                if err := memoryInstance.Append(ctx, msg); err != nil {
                    results <- fmt.Errorf("worker %d failed to append: %w", workerID, err)
                    return
                }

                // Small delay to increase chance of contention
                time.Sleep(time.Millisecond * 10)
            }

            results <- nil
        }(i)
    }

    // Wait for all workers to complete
    for i := 0; i < numWorkers; i++ {
        select {
        case err := <-results:
            require.NoError(t, err)
        case <-time.After(30 * time.Second):
            t.Fatal("Test timed out waiting for workers")
        }
    }

    // Verify final state
    finalInstance, err := s.memoryManager.GetInstance(ctx, memRef, workflowContext)
    require.NoError(t, err)

    finalMessages, err := finalInstance.Read(ctx)
    require.NoError(t, err)

    // Should have all messages from all workers
    expectedMessageCount := numWorkers * messagesPerWorker
    assert.Len(t, finalMessages, expectedMessageCount)

    // Verify no duplicate or corrupted messages
    messageContents := make(map[string]bool)
    for _, msg := range finalMessages {
        assert.False(t, messageContents[msg.Content], "Duplicate message found: %s", msg.Content)
        messageContents[msg.Content] = true
    }
}
```

### Performance Benchmarks

```go
// test/integration/memory/benchmark_test.go
package memory_test

import (
    "context"
    "testing"

    "github.com/compozy/compozy/engine/memory/core"
    "github.com/compozy/compozy/engine/llm"
)

func BenchmarkMemoryOperations(b *testing.B) {
    suite := setupBenchmarkSuite(b)
    defer suite.cleanup()

    b.Run("Append", func(b *testing.B) {
        suite.benchmarkAppend(b)
    })

    b.Run("Read", func(b *testing.B) {
        suite.benchmarkRead(b)
    })

    b.Run("TokenCount", func(b *testing.B) {
        suite.benchmarkTokenCount(b)
    })

    b.Run("ConcurrentAppend", func(b *testing.B) {
        suite.benchmarkConcurrentAppend(b)
    })
}

func (s *BenchmarkSuite) benchmarkAppend(b *testing.B) {
    ctx := context.Background()
    memoryInstance := s.getTestMemoryInstance(ctx, b)

    msg := llm.Message{
        Role:    "user",
        Content: "This is a test message for benchmarking append operations.",
    }

    b.ResetTimer()
    b.ReportAllocs()

    for i := 0; i < b.N; i++ {
        err := memoryInstance.Append(ctx, msg)
        if err != nil {
            b.Fatalf("Append failed: %v", err)
        }
    }

    // Verify performance target: <50ms per operation
    if b.Elapsed()/time.Duration(b.N) > 50*time.Millisecond {
        b.Errorf("Append operation too slow: %v per operation", b.Elapsed()/time.Duration(b.N))
    }
}

func (s *BenchmarkSuite) benchmarkConcurrentAppend(b *testing.B) {
    ctx := context.Background()
    memoryInstance := s.getTestMemoryInstance(ctx, b)

    msg := llm.Message{
        Role:    "user",
        Content: "Concurrent append test message.",
    }

    b.ResetTimer()
    b.ReportAllocs()

    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            err := memoryInstance.Append(ctx, msg)
            if err != nil {
                b.Errorf("Concurrent append failed: %v", err)
            }
        }
    })
}
```

### Flush Workflow Tests

```go
func (s *E2ETestSuite) testFlushWorkflow(t *testing.T) {
    ctx := context.Background()

    // Create memory instance with flush configuration
    memRef := core.MemoryReference{
        ID:  "flushable-memory",
        Key: "flush-test-{{.test.id}}",
    }

    workflowContext := map[string]any{
        "project.id": "test-project",
        "test.id":    "flush-workflow",
    }

    memoryInstance, err := s.memoryManager.GetInstance(ctx, memRef, workflowContext)
    require.NoError(t, err)

    // Add many messages to trigger flush
    for i := 0; i < 100; i++ {
        msg := llm.Message{
            Role:    "user",
            Content: fmt.Sprintf("Message %d - this is a longer message to accumulate tokens and trigger flush behavior", i),
        }
        err := memoryInstance.Append(ctx, msg)
        require.NoError(t, err)
    }

    // Check if flush should be triggered
    flushableMemory, ok := memoryInstance.(core.FlushableMemory)
    require.True(t, ok)

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
    assert.Less(t, len(postFlushMessages), 100)
    assert.Equal(t, len(postFlushMessages), flushResult.MessageCount)
}
```

**Key Implementation Notes:**

- Uses `github.com/stretchr/testify` for comprehensive assertions
- Temporal test suite integration for workflow testing
- Concurrent access validation with race condition detection
- Performance benchmarks with clear targets (<50ms per operation)
- Real Redis integration for distributed locking validation

**⚠️ COMPLEXITY WARNING**: E2E test suites are notoriously complex to implement and maintain:

- Test environment setup often requires significant infrastructure investment
- Mock service coordination and data management is complex
- Test suite maintenance complexity can exceed the features being tested
- Flaky tests are common and require sophisticated debugging and monitoring
- **CRITICAL**: This task should be developed in parallel with features, not sequentially

## Success Criteria

- ✅ Complete memory lifecycle works end-to-end with Temporal integration
- ✅ Concurrent access scenarios pass without race conditions or data corruption
- ✅ Flush and cleanup workflows execute properly and maintain data integrity
- ✅ Performance benchmarks meet targets (<50ms per operation, <10MB memory usage)
- ✅ Multi-provider token counting integration validated
- ✅ Resilience patterns behave correctly under failure conditions
- ✅ All tests pass consistently in CI/CD environment
- ✅ Test coverage exceeds 85% for critical memory operations

<critical>
**MANDATORY REQUIREMENTS:**

- **MUST** use existing test infrastructure (Temporal test suite, Redis test containers)
- **MUST** include race condition testing with `go test -race`
- **MUST** validate performance targets (<50ms overhead, <10MB memory)
- **MUST** test all major code paths and error scenarios
- **MUST** include cleanup procedures to prevent test pollution
- **MUST** follow established testing patterns with `t.Run("Should...")` format
- **MUST** run `make lint` and `make test` before completion
- **MUST** follow `.cursor/rules/task-review.mdc` workflow for completion
  </critical>
