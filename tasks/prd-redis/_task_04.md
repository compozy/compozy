## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine/memory/store, test/integration/standalone</domain>
<type>integration, testing</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>Task 1.0, Task 2.0, Task 3.0</dependencies>
</task_context>

# Task 4.0: Memory Store Integration

## Overview

Verify that the memory store works seamlessly with miniredis by testing all Lua script operations, concurrent message appends, metadata preservation, and conversation history consistency. This task validates FR-3 from the PRD (Memory Store Compatibility).

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the technical docs from this PRD before start
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha, and support old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility
- **MUST** use `config.FromContext(ctx)` - never store config
- **MUST** use `logger.FromContext(ctx)` - never pass logger as parameter
- **NEVER** use `context.Background()` in tests, use `t.Context()` instead
- **NEVER** modify memory store implementation - only write tests to verify compatibility
</critical>

<research>
# When you need information about memory store implementation:
- Read `engine/memory/store/redis.go` to understand the RedisMemoryStore
- Read `engine/memory/store/scripts.go` for Lua script definitions
- Identify AppendAndTrimWithMetadataScript and other Lua scripts
- Review existing memory store tests for patterns
</research>

<requirements>
- Verify memory store works with miniredis (zero code changes to memory store)
- Test AppendAndTrimWithMetadataScript Lua script execution
- Test concurrent message appends
- Test message metadata preservation
- Test conversation history trim at max length
- Test conversation history consistency
- Create integration tests in `test/integration/standalone/memory_store_test.go`
- All tests must use `t.Context()` and follow project standards
</requirements>

## Subtasks

- [ ] 4.1 Read existing memory store implementation to understand operations
- [ ] 4.2 Identify all Lua scripts used by memory store
- [ ] 4.3 Create test/integration/standalone/memory_store_test.go
- [ ] 4.4 Create test helper to setup memory store with miniredis
- [ ] 4.5 Write test for AppendAndTrimWithMetadataScript execution
- [ ] 4.6 Write test for concurrent message appends
- [ ] 4.7 Write test for message metadata preservation
- [ ] 4.8 Write test for conversation history consistency
- [ ] 4.9 Write test for conversation history trimming
- [ ] 4.10 Write test for message retrieval with pagination

## Implementation Details

### Memory Store Test Structure

Create `test/integration/standalone/memory_store_test.go`:

```go
package standalone_test

import (
    "context"
    "sync"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/compozy/compozy/engine/infra/cache"
    "github.com/compozy/compozy/engine/memory/store"
    "github.com/compozy/compozy/pkg/config"
    "github.com/compozy/compozy/test/helpers"
)

// setupMemoryStoreWithMiniredis creates a memory store backed by miniredis
func setupMemoryStoreWithMiniredis(ctx context.Context, t *testing.T) store.MemoryStore {
    t.Helper()

    // Setup standalone config
    cfg := &config.Config{
        Mode: "standalone",
        Redis: config.RedisConfig{
            Standalone: config.RedisStandaloneConfig{
                Persistence: config.RedisPersistenceConfig{
                    Enabled: false, // No persistence for tests
                },
            },
        },
    }
    ctx = config.ContextWithManager(ctx, cfg)

    // Create miniredis backend
    standalone, err := cache.NewMiniredisStandalone(ctx)
    require.NoError(t, err)
    t.Cleanup(func() {
        standalone.Close(ctx)
    })

    // Create memory store with miniredis client
    memoryStore := store.NewRedisMemoryStore(standalone.Client())
    return memoryStore
}

func TestMemoryStore_MiniredisCompatibility(t *testing.T) {
    t.Run("Should execute Lua scripts natively", func(t *testing.T) {
        ctx := t.Context()
        ms := setupMemoryStoreWithMiniredis(ctx, t)

        agentID := "test-agent"
        message := &store.Message{
            Role:    "user",
            Content: "Hello, world!",
            Metadata: map[string]interface{}{
                "timestamp": "2025-01-27T10:00:00Z",
                "session":   "test-session",
            },
        }

        // Test AppendAndTrimWithMetadataScript
        err := ms.AppendMessage(ctx, agentID, message)
        require.NoError(t, err)

        // Verify message stored with metadata
        messages, err := ms.GetMessages(ctx, agentID)
        require.NoError(t, err)
        assert.Len(t, messages, 1)
        assert.Equal(t, message.Role, messages[0].Role)
        assert.Equal(t, message.Content, messages[0].Content)
        assert.Equal(t, message.Metadata["timestamp"], messages[0].Metadata["timestamp"])
        assert.Equal(t, message.Metadata["session"], messages[0].Metadata["session"])
    })

    t.Run("Should handle concurrent message appends", func(t *testing.T) {
        ctx := t.Context()
        ms := setupMemoryStoreWithMiniredis(ctx, t)

        agentID := "concurrent-test-agent"
        numMessages := 50
        var wg sync.WaitGroup

        // Append messages concurrently
        for i := 0; i < numMessages; i++ {
            wg.Add(1)
            go func(idx int) {
                defer wg.Done()
                message := &store.Message{
                    Role:    "user",
                    Content: fmt.Sprintf("Message %d", idx),
                }
                err := ms.AppendMessage(ctx, agentID, message)
                assert.NoError(t, err)
            }(i)
        }

        wg.Wait()

        // Verify all messages stored
        messages, err := ms.GetMessages(ctx, agentID)
        require.NoError(t, err)
        assert.Len(t, messages, numMessages)
    })

    t.Run("Should trim conversation history at max length", func(t *testing.T) {
        ctx := t.Context()
        ms := setupMemoryStoreWithMiniredis(ctx, t)

        agentID := "trim-test-agent"
        maxLength := 10

        // Append more than max messages
        for i := 0; i < maxLength+5; i++ {
            message := &store.Message{
                Role:    "user",
                Content: fmt.Sprintf("Message %d", i),
            }
            err := ms.AppendMessage(ctx, agentID, message)
            require.NoError(t, err)
        }

        // Verify only max messages retained
        messages, err := ms.GetMessages(ctx, agentID)
        require.NoError(t, err)
        assert.LessOrEqual(t, len(messages), maxLength)

        // Verify newest messages retained
        lastMessage := messages[len(messages)-1]
        assert.Contains(t, lastMessage.Content, "Message")
    })

    t.Run("Should preserve message metadata across operations", func(t *testing.T) {
        ctx := t.Context()
        ms := setupMemoryStoreWithMiniredis(ctx, t)

        agentID := "metadata-test-agent"
        metadata := map[string]interface{}{
            "timestamp":  "2025-01-27T10:00:00Z",
            "session":    "test-session",
            "user_id":    "user-123",
            "ip_address": "192.168.1.1",
        }

        message := &store.Message{
            Role:     "user",
            Content:  "Test message",
            Metadata: metadata,
        }

        err := ms.AppendMessage(ctx, agentID, message)
        require.NoError(t, err)

        // Retrieve and verify metadata
        messages, err := ms.GetMessages(ctx, agentID)
        require.NoError(t, err)
        require.Len(t, messages, 1)

        retrieved := messages[0]
        assert.Equal(t, metadata["timestamp"], retrieved.Metadata["timestamp"])
        assert.Equal(t, metadata["session"], retrieved.Metadata["session"])
        assert.Equal(t, metadata["user_id"], retrieved.Metadata["user_id"])
        assert.Equal(t, metadata["ip_address"], retrieved.Metadata["ip_address"])
    })

    t.Run("Should maintain conversation history consistency", func(t *testing.T) {
        ctx := t.Context()
        ms := setupMemoryStoreWithMiniredis(ctx, t)

        agentID := "consistency-test-agent"

        // Append multiple messages
        messages := []*store.Message{
            {Role: "user", Content: "Question 1"},
            {Role: "assistant", Content: "Answer 1"},
            {Role: "user", Content: "Question 2"},
            {Role: "assistant", Content: "Answer 2"},
        }

        for _, msg := range messages {
            err := ms.AppendMessage(ctx, agentID, msg)
            require.NoError(t, err)
        }

        // Verify conversation order maintained
        retrieved, err := ms.GetMessages(ctx, agentID)
        require.NoError(t, err)
        require.Len(t, retrieved, len(messages))

        for i, msg := range messages {
            assert.Equal(t, msg.Role, retrieved[i].Role)
            assert.Equal(t, msg.Content, retrieved[i].Content)
        }
    })

    t.Run("Should support message retrieval with pagination", func(t *testing.T) {
        ctx := t.Context()
        ms := setupMemoryStoreWithMiniredis(ctx, t)

        agentID := "pagination-test-agent"
        totalMessages := 25

        // Append messages
        for i := 0; i < totalMessages; i++ {
            message := &store.Message{
                Role:    "user",
                Content: fmt.Sprintf("Message %d", i),
            }
            err := ms.AppendMessage(ctx, agentID, message)
            require.NoError(t, err)
        }

        // Test pagination (if supported)
        messages, err := ms.GetMessages(ctx, agentID)
        require.NoError(t, err)
        assert.Len(t, messages, totalMessages)
    })
}
```

### Relevant Files

- `test/integration/standalone/memory_store_test.go` - NEW - Memory store integration tests
- `engine/memory/store/redis.go` - VERIFY ONLY - Memory store implementation (no changes)
- `engine/memory/store/scripts.go` - VERIFY ONLY - Lua scripts (no changes)

### Dependent Files

- `engine/infra/cache/miniredis_standalone.go` - Uses MiniredisStandalone from Task 2.0
- `pkg/config/config.go` - Uses Config from Task 1.0

## Deliverables

- [ ] test/integration/standalone/memory_store_test.go created
- [ ] setupMemoryStoreWithMiniredis() helper function
- [ ] Test for Lua script execution (AppendAndTrimWithMetadataScript)
- [ ] Test for concurrent message appends
- [ ] Test for message metadata preservation
- [ ] Test for conversation history trimming
- [ ] Test for conversation history consistency
- [ ] Test for message retrieval with pagination
- [ ] All tests use t.Context() (no context.Background())
- [ ] All tests follow "Should..." naming convention

## Tests

All tests are defined in the implementation section above. Summary of test coverage:

### Lua Script Tests
- [ ] Should execute Lua scripts natively (AppendAndTrimWithMetadataScript)
- [ ] Should handle script errors gracefully

### Concurrent Operation Tests
- [ ] Should handle concurrent message appends without data loss
- [ ] Should maintain message ordering under concurrent writes

### Metadata Tests
- [ ] Should preserve message metadata across operations
- [ ] Should preserve complex metadata structures (nested objects, arrays)

### Conversation History Tests
- [ ] Should trim conversation history at max length
- [ ] Should maintain conversation history consistency
- [ ] Should preserve message order in conversation history

### Pagination Tests
- [ ] Should support message retrieval with pagination (if applicable)
- [ ] Should handle edge cases (empty history, single message)

### Edge Cases
- [ ] Should handle empty conversation history
- [ ] Should handle messages with no metadata
- [ ] Should handle messages with large content
- [ ] Should handle special characters in message content

## Success Criteria

- [ ] All memory store integration tests pass
- [ ] Lua scripts execute successfully in miniredis
- [ ] Concurrent appends work without data loss
- [ ] Message metadata preserved correctly
- [ ] Conversation history maintains consistency
- [ ] Conversation trimming works at max length
- [ ] Zero changes required to memory store implementation
- [ ] All tests use t.Context() (no context.Background())
- [ ] `go test ./test/integration/standalone/...` passes
- [ ] `make lint` passes with zero warnings
- [ ] Test coverage demonstrates miniredis compatibility with memory store
