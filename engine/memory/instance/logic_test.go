package instance

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Test core business logic without complex external dependencies
func TestMemoryInstance_BusinessLogic(t *testing.T) {
	t.Run("Should append message and update token count", func(t *testing.T) {
		// Create simple mocks
		mockStore := &mockStore{}
		mockTokenCounter := &mockTokenCounter{}
		mockLockManager := &mockLockManager{}
		mockFlushStrategy := &mockFlushStrategy{}
		unlockFunc := func() error { return nil }

		// Create synchronization channel for async operations
		flushCheckDone := make(chan bool, 1)

		// Create instance with minimal setup
		instance := &memoryInstance{
			id:               "test-id",
			store:            mockStore,
			tokenCounter:     mockTokenCounter,
			lockManager:      mockLockManager,
			flushingStrategy: mockFlushStrategy,
			logger:           logger.NewForTests(),
			metrics:          NewDefaultMetrics(logger.NewForTests()),
		}

		ctx := context.Background()
		msg := llm.Message{Role: "user", Content: "Hello world"}

		// Setup expectations for main flow
		mockLockManager.On("AcquireAppendLock", ctx, "test-id").Return(unlockFunc, nil)
		mockTokenCounter.On("CountTokens", ctx, "Hello world").Return(5, nil)
		mockTokenCounter.On("CountTokens", ctx, "user").Return(1, nil)
		mockStore.On("AppendMessageWithTokenCount", ctx, "test-id", msg, 8).Return(nil)

		// Setup expectations for async checkFlushTrigger goroutine with channel notification
		mockStore.On("GetTokenCount", mock.Anything, "test-id").Return(8, nil).Run(func(_ mock.Arguments) {
			// Signal that the async operation has started
			select {
			case flushCheckDone <- true:
			default:
			}
		})
		mockStore.On("GetMessageCount", mock.Anything, "test-id").Return(1, nil)
		mockFlushStrategy.On("ShouldFlush", 8, 1, (*core.Resource)(nil)).Return(false)

		// Execute
		err := instance.Append(ctx, msg)

		// Wait for async goroutine to complete using channel
		select {
		case <-flushCheckDone:
			// Async operation completed
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Async flush check did not complete within timeout")
		}

		// Verify
		assert.NoError(t, err)
		mockStore.AssertExpectations(t)
		mockTokenCounter.AssertExpectations(t)
		mockLockManager.AssertExpectations(t)
		mockFlushStrategy.AssertExpectations(t)
	})

	t.Run("Should read messages from store", func(t *testing.T) {
		mockStore := &mockStore{}
		instance := &memoryInstance{
			id:      "test-id",
			store:   mockStore,
			logger:  logger.NewForTests(),
			metrics: NewDefaultMetrics(logger.NewForTests()),
		}

		ctx := context.Background()
		expectedMessages := []llm.Message{
			{Role: "user", Content: "message 1"},
			{Role: "assistant", Content: "response 1"},
		}

		mockStore.On("ReadMessages", ctx, "test-id").Return(expectedMessages, nil)

		messages, err := instance.Read(ctx)

		assert.NoError(t, err)
		assert.Equal(t, expectedMessages, messages)
		mockStore.AssertExpectations(t)
	})

	t.Run("Should get token count from store", func(t *testing.T) {
		mockStore := &mockStore{}
		instance := &memoryInstance{
			id:      "test-id",
			store:   mockStore,
			logger:  logger.NewForTests(),
			metrics: NewDefaultMetrics(logger.NewForTests()),
		}

		ctx := context.Background()
		expectedCount := 150

		mockStore.On("GetTokenCount", ctx, "test-id").Return(expectedCount, nil)

		count, err := instance.GetTokenCount(ctx)

		assert.NoError(t, err)
		assert.Equal(t, expectedCount, count)
		mockStore.AssertExpectations(t)
	})

	t.Run("Should get message count from store", func(t *testing.T) {
		mockStore := &mockStore{}
		instance := &memoryInstance{
			id:      "test-id",
			store:   mockStore,
			logger:  logger.NewForTests(),
			metrics: NewDefaultMetrics(logger.NewForTests()),
		}

		ctx := context.Background()
		expectedCount := 10

		mockStore.On("GetMessageCount", ctx, "test-id").Return(expectedCount, nil)

		count, err := instance.Len(ctx)

		assert.NoError(t, err)
		assert.Equal(t, expectedCount, count)
		mockStore.AssertExpectations(t)
	})

	t.Run("Should clear all messages", func(t *testing.T) {
		mockStore := &mockStore{}
		mockLockManager := &mockLockManager{}
		unlockFunc := func() error { return nil }

		instance := &memoryInstance{
			id:          "test-id",
			store:       mockStore,
			lockManager: mockLockManager,
			logger:      logger.NewForTests(),
			metrics:     NewDefaultMetrics(logger.NewForTests()),
		}

		ctx := context.Background()

		mockLockManager.On("AcquireClearLock", ctx, "test-id").Return(unlockFunc, nil)
		mockStore.On("DeleteMessages", ctx, "test-id").Return(nil)

		err := instance.Clear(ctx)

		assert.NoError(t, err)
		mockStore.AssertExpectations(t)
		mockLockManager.AssertExpectations(t)
	})

	t.Run("Should return health information", func(t *testing.T) {
		mockStore := &mockStore{}
		mockFlushStrategy := &mockFlushStrategy{}

		instance := &memoryInstance{
			id:               "test-id",
			store:            mockStore,
			flushingStrategy: mockFlushStrategy,
			logger:           logger.NewForTests(),
			metrics:          NewDefaultMetrics(logger.NewForTests()),
		}

		ctx := context.Background()

		mockStore.On("GetMessageCount", ctx, "test-id").Return(5, nil)
		mockStore.On("GetTokenCount", ctx, "test-id").Return(100, nil)
		mockFlushStrategy.On("GetType").Return(core.HybridSummaryFlushing)

		health, err := instance.GetMemoryHealth(ctx)

		assert.NoError(t, err)
		assert.NotNil(t, health)
		assert.Equal(t, 5, health.MessageCount)
		assert.Equal(t, 100, health.TokenCount)
		assert.Equal(t, "hybrid_summary", health.FlushStrategy)
		mockStore.AssertExpectations(t)
		mockFlushStrategy.AssertExpectations(t)
	})
}

// Test error handling scenarios
func TestMemoryInstance_ErrorHandling(t *testing.T) {
	t.Run("Should handle lock acquisition failure", func(t *testing.T) {
		mockLockManager := &mockLockManager{}
		instance := &memoryInstance{
			id:          "test-id",
			lockManager: mockLockManager,
			logger:      logger.NewForTests(),
			metrics:     NewDefaultMetrics(logger.NewForTests()),
		}

		ctx := context.Background()
		msg := llm.Message{Role: "user", Content: "test"}

		mockLockManager.On("AcquireAppendLock", ctx, "test-id").Return(nil, assert.AnError)

		err := instance.Append(ctx, msg)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to acquire lock")
		mockLockManager.AssertExpectations(t)
	})

	t.Run("Should handle store operation failure", func(t *testing.T) {
		mockStore := &mockStore{}
		mockTokenCounter := &mockTokenCounter{}
		mockLockManager := &mockLockManager{}
		unlockFunc := func() error { return nil }

		instance := &memoryInstance{
			id:           "test-id",
			store:        mockStore,
			tokenCounter: mockTokenCounter,
			lockManager:  mockLockManager,
			logger:       logger.NewForTests(),
			metrics:      NewDefaultMetrics(logger.NewForTests()),
		}

		ctx := context.Background()
		msg := llm.Message{Role: "user", Content: "test"}

		mockLockManager.On("AcquireAppendLock", ctx, "test-id").Return(unlockFunc, nil)
		mockTokenCounter.On("CountTokens", ctx, "test").Return(3, nil)
		mockTokenCounter.On("CountTokens", ctx, "user").Return(1, nil)
		mockStore.On("AppendMessageWithTokenCount", ctx, "test-id", msg, 6).Return(assert.AnError)

		err := instance.Append(ctx, msg)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to append message")
		mockStore.AssertExpectations(t)
		mockTokenCounter.AssertExpectations(t)
		mockLockManager.AssertExpectations(t)
	})

	t.Run("Should log error when lock release fails", func(t *testing.T) {
		mockStore := &mockStore{}
		mockTokenCounter := &mockTokenCounter{}
		mockLockManager := &mockLockManager{}
		failingUnlockFunc := func() error { return assert.AnError }

		testLogger := logger.NewForTests()

		mockFlushStrategy := &mockFlushStrategy{}

		// Create synchronization channel for async operations
		flushCheckDone := make(chan bool, 1)

		instance := &memoryInstance{
			id:               "test-id",
			store:            mockStore,
			tokenCounter:     mockTokenCounter,
			lockManager:      mockLockManager,
			flushingStrategy: mockFlushStrategy,
			logger:           testLogger,
			metrics:          NewDefaultMetrics(testLogger),
		}

		ctx := context.Background()
		msg := llm.Message{Role: "user", Content: "test"}

		mockLockManager.On("AcquireAppendLock", ctx, "test-id").Return(failingUnlockFunc, nil)
		mockTokenCounter.On("CountTokens", ctx, "test").Return(3, nil)
		mockTokenCounter.On("CountTokens", ctx, "user").Return(1, nil)
		mockStore.On("AppendMessageWithTokenCount", ctx, "test-id", msg, 6).Return(nil)

		// Setup expectations for async checkFlushTrigger goroutine with channel notification
		mockStore.On("GetTokenCount", mock.Anything, "test-id").Return(6, nil).Run(func(_ mock.Arguments) {
			// Signal that the async operation has started
			select {
			case flushCheckDone <- true:
			default:
			}
		})
		mockStore.On("GetMessageCount", mock.Anything, "test-id").Return(1, nil)
		mockFlushStrategy.On("ShouldFlush", 6, 1, (*core.Resource)(nil)).Return(false)

		err := instance.Append(ctx, msg)

		// Should succeed despite lock release failure
		assert.NoError(t, err)

		// Wait for async goroutine to complete using channel
		select {
		case <-flushCheckDone:
			// Async operation completed
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Async flush check did not complete within timeout")
		}

		mockStore.AssertExpectations(t)
		mockTokenCounter.AssertExpectations(t)
		mockLockManager.AssertExpectations(t)
		mockFlushStrategy.AssertExpectations(t)
	})
}

// TODO: Implement TestFlushOperations_ErrorLogging when flush operations are integrated
