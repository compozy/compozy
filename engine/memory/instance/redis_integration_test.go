package instance

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/mocks"

	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/store"
	"github.com/compozy/compozy/pkg/logger"
)

// redisTestClient wraps a redis.Client to implement cache.RedisInterface
type redisTestClient struct {
	*redis.Client
}

func (r *redisTestClient) Pipeline() redis.Pipeliner {
	return r.Client.Pipeline()
}

// setupMiniredisMemoryInstance creates a memory instance with miniredis for testing
func setupMiniredisMemoryInstance(t *testing.T, instanceID string) (core.Memory, func()) {
	// Start miniredis server
	server, err := miniredis.Run()
	require.NoError(t, err)

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr: server.Addr(),
	})

	// Create test client wrapper
	testClient := &redisTestClient{Client: client}

	// Create Redis store
	redisStore := store.NewRedisMemoryStore(testClient, "test")

	// Create mock lock manager with no-op functions
	mockLockManager := &mockLockManager{}
	unlockFunc := func() error { return nil }
	mockLockManager.On("AcquireAppendLock", mock.Anything, mock.Anything).Return(unlockFunc, nil)
	mockLockManager.On("AcquireClearLock", mock.Anything, mock.Anything).Return(unlockFunc, nil)
	mockLockManager.On("AcquireFlushLock", mock.Anything, mock.Anything).Return(unlockFunc, nil)

	// Create mock token counter
	mockTokenCounter := &mockTokenCounter{}
	mockTokenCounter.On("CountTokens", mock.Anything, mock.Anything).Return(10, nil)

	// Create mock flush strategy
	mockFlushStrategy := &mockFlushStrategy{}
	mockFlushStrategy.On("ShouldFlush", mock.Anything, mock.Anything, mock.Anything).Return(false)
	mockFlushStrategy.On("GetType").Return(core.SimpleFIFOFlushing)

	// Create mock temporal client
	mockClient := &mocks.Client{}

	// Create resource config
	resource := &core.Resource{
		ID:          instanceID,
		Type:        core.TokenBasedMemory,
		MaxTokens:   4000,
		MaxMessages: 100,
	}

	// Create memory instance using builder
	memInstance, err := NewBuilder().
		WithInstanceID(instanceID).
		WithResourceConfig(resource).
		WithStore(redisStore).
		WithLockManager(mockLockManager).
		WithTokenCounter(mockTokenCounter).
		WithFlushingStrategy(mockFlushStrategy).
		WithTemporalClient(mockClient).
		WithLogger(logger.NewForTests()).
		Build(context.Background())
	require.NoError(t, err)

	cleanup := func() {
		client.Close()
		server.Close()
	}

	return memInstance, cleanup
}

// TestMemoryInstance_WithRealRedis tests memory instances with real Redis behavior
func TestMemoryInstance_WithRealRedis(t *testing.T) {
	ctx := context.Background()

	t.Run("Should append and read messages with real Redis store", func(t *testing.T) {
		// Setup real Redis environment
		memInstance, cleanup := setupMiniredisMemoryInstance(t, "test_instance_append_read")
		defer cleanup()

		// Test messages
		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Hello, how are you?"},
			{Role: llm.MessageRoleAssistant, Content: "I'm doing well, thank you!"},
			{Role: llm.MessageRoleUser, Content: "That's great to hear."},
		}

		// Append messages one by one
		for _, msg := range messages {
			err := memInstance.Append(ctx, msg)
			require.NoError(t, err)
		}

		// Read all messages
		retrievedMessages, err := memInstance.Read(ctx)
		require.NoError(t, err)

		// Verify
		assert.Len(t, retrievedMessages, 3)
		for i, expected := range messages {
			assert.Equal(t, expected.Role, retrievedMessages[i].Role)
			assert.Equal(t, expected.Content, retrievedMessages[i].Content)
		}

		// Test message count
		count, err := memInstance.Len(ctx)
		require.NoError(t, err)
		assert.Equal(t, 3, count)

		// Test token count (should be > 0)
		tokenCount, err := memInstance.GetTokenCount(ctx)
		require.NoError(t, err)
		assert.Greater(t, tokenCount, 0)
	})

	t.Run("Should handle paginated reading with real Redis store", func(t *testing.T) {
		// Setup real Redis environment
		memInstance, cleanup := setupMiniredisMemoryInstance(t, "test_instance_paginated")
		defer cleanup()

		// Add 10 messages
		messages := make([]llm.Message, 10)
		for i := 0; i < 10; i++ {
			messages[i] = llm.Message{
				Role:    llm.MessageRoleUser,
				Content: fmt.Sprintf("Message %d", i+1),
			}
			err := memInstance.Append(ctx, messages[i])
			require.NoError(t, err)
		}

		// Test paginated reading
		page1, total1, err := memInstance.ReadPaginated(ctx, 0, 3)
		require.NoError(t, err)
		assert.Equal(t, 10, total1)
		assert.Len(t, page1, 3)
		assert.Equal(t, "Message 1", page1[0].Content)
		assert.Equal(t, "Message 2", page1[1].Content)
		assert.Equal(t, "Message 3", page1[2].Content)

		// Test second page
		page2, total2, err := memInstance.ReadPaginated(ctx, 3, 3)
		require.NoError(t, err)
		assert.Equal(t, 10, total2)
		assert.Len(t, page2, 3)
		assert.Equal(t, "Message 4", page2[0].Content)
		assert.Equal(t, "Message 5", page2[1].Content)
		assert.Equal(t, "Message 6", page2[2].Content)

		// Test partial last page
		page3, total3, err := memInstance.ReadPaginated(ctx, 8, 5)
		require.NoError(t, err)
		assert.Equal(t, 10, total3)
		assert.Len(t, page3, 2) // Only 2 messages left
		assert.Equal(t, "Message 9", page3[0].Content)
		assert.Equal(t, "Message 10", page3[1].Content)

		// Test offset beyond data
		page4, total4, err := memInstance.ReadPaginated(ctx, 15, 5)
		require.NoError(t, err)
		assert.Equal(t, 10, total4)
		assert.Empty(t, page4)
	})

	t.Run("Should handle clear operation with real Redis store", func(t *testing.T) {
		// Setup real Redis environment
		memInstance, cleanup := setupMiniredisMemoryInstance(t, "test_instance_clear")
		defer cleanup()

		// Add some messages
		for i := 0; i < 5; i++ {
			msg := llm.Message{
				Role:    llm.MessageRoleUser,
				Content: fmt.Sprintf("Message %d", i+1),
			}
			err := memInstance.Append(ctx, msg)
			require.NoError(t, err)
		}

		// Verify messages exist
		count, err := memInstance.Len(ctx)
		require.NoError(t, err)
		assert.Equal(t, 5, count)

		// Clear messages
		err = memInstance.Clear(ctx)
		require.NoError(t, err)

		// Verify messages are cleared
		count, err = memInstance.Len(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, count)

		// Verify reading returns empty
		messages, err := memInstance.Read(ctx)
		require.NoError(t, err)
		assert.Empty(t, messages)

		// Verify token count is reset
		tokenCount, err := memInstance.GetTokenCount(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, tokenCount)
	})

	t.Run("Should handle memory health reporting with real Redis store", func(t *testing.T) {
		// Setup real Redis environment
		memInstance, cleanup := setupMiniredisMemoryInstance(t, "test_instance_health")
		defer cleanup()

		// Add some messages
		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "How's the weather?"},
			{Role: llm.MessageRoleAssistant, Content: "It's sunny today!"},
		}

		for _, msg := range messages {
			err := memInstance.Append(ctx, msg)
			require.NoError(t, err)
		}

		// Get health status
		health, err := memInstance.GetMemoryHealth(ctx)
		require.NoError(t, err)

		// Verify health data
		assert.Equal(t, 2, health.MessageCount)
		assert.Greater(t, health.TokenCount, 0)
		assert.NotEmpty(t, health.FlushStrategy)
	})

	t.Run("Should handle concurrent access with real Redis store", func(t *testing.T) {
		// Setup real Redis environment
		memInstance, cleanup := setupMiniredisMemoryInstance(t, "test_instance_concurrent")
		defer cleanup()

		// Use channels to coordinate goroutines
		numGoroutines := 10
		done := make(chan bool, numGoroutines)
		errors := make(chan error, numGoroutines)

		// Launch multiple goroutines that append messages concurrently
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer func() { done <- true }()

				// Each goroutine appends 3 messages
				for j := 0; j < 3; j++ {
					msg := llm.Message{
						Role:    llm.MessageRoleUser,
						Content: fmt.Sprintf("Goroutine %d Message %d", id, j+1),
					}
					if err := memInstance.Append(ctx, msg); err != nil {
						errors <- err
						return
					}
				}
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < numGoroutines; i++ {
			select {
			case <-done:
				// Goroutine completed successfully
			case err := <-errors:
				t.Fatalf("Goroutine failed: %v", err)
			case <-time.After(10 * time.Second):
				t.Fatal("Test timed out")
			}
		}

		// Verify final state
		count, err := memInstance.Len(ctx)
		require.NoError(t, err)
		assert.Equal(t, numGoroutines*3, count) // 10 goroutines * 3 messages each

		// Verify all messages are stored
		messages, err := memInstance.Read(ctx)
		require.NoError(t, err)
		assert.Len(t, messages, numGoroutines*3)

		// Verify token count is reasonable
		tokenCount, err := memInstance.GetTokenCount(ctx)
		require.NoError(t, err)
		assert.Greater(t, tokenCount, 0)
	})
}
