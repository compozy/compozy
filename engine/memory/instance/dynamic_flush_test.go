package instance

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/instance/strategies"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestDynamicFlushableMemory tests the DynamicFlushableMemory interface implementation
func TestDynamicFlushableMemory(t *testing.T) {
	t.Run("Should implement DynamicFlushableMemory interface", func(_ *testing.T) {
		instance := &memoryInstance{}
		// Verify the instance implements the interface
		var _ core.DynamicFlushableMemory = instance
	})
	t.Run("Should get configured strategy type", func(t *testing.T) {
		tests := []struct {
			name           string
			resourceConfig *core.Resource
			expectedType   core.FlushingStrategyType
		}{
			{
				name: "returns configured LRU strategy",
				resourceConfig: &core.Resource{
					FlushingStrategy: &core.FlushingStrategyConfig{
						Type: core.LRUFlushing,
					},
				},
				expectedType: core.LRUFlushing,
			},
			{
				name: "returns configured token aware LRU strategy",
				resourceConfig: &core.Resource{
					FlushingStrategy: &core.FlushingStrategyConfig{
						Type: core.TokenAwareLRUFlushing,
					},
				},
				expectedType: core.TokenAwareLRUFlushing,
			},
			{
				name: "returns default FIFO when no strategy configured",
				resourceConfig: &core.Resource{
					FlushingStrategy: nil,
				},
				expectedType: core.SimpleFIFOFlushing,
			},
			{
				name:           "returns default FIFO when no resource config",
				resourceConfig: nil,
				expectedType:   core.SimpleFIFOFlushing,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				instance := &memoryInstance{
					resourceConfig: tt.resourceConfig,
				}
				assert.Equal(t, tt.expectedType, instance.GetConfiguredStrategy())
			})
		}
	})
	t.Run("Should perform flush with default strategy when strategy type is empty", func(t *testing.T) {
		mockStore := &mockStore{}
		mockLockManager := &mockLockManager{}
		mockFlushStrategy := &mockFlushStrategy{}
		factory := strategies.NewStrategyFactory()
		unlockFunc := func() error { return nil }
		ctx := context.Background()
		messages := []llm.Message{
			{Role: "user", Content: "test message"},
		}
		expectedOutput := &core.FlushMemoryActivityOutput{
			Success:      true,
			MessageCount: 1,
			TokenCount:   10,
		}
		// Setup expectations
		mockLockManager.On("AcquireFlushLock", ctx, "test-id").Return(unlockFunc, nil)
		mockStore.On("ReadMessages", ctx, "test-id").Return(messages, nil)
		mockFlushStrategy.On("PerformFlush", ctx, messages, (*core.Resource)(nil)).Return(expectedOutput, nil)
		instance := &memoryInstance{
			id:               "test-id",
			projectID:        "project-123",
			store:            mockStore,
			lockManager:      mockLockManager,
			flushingStrategy: mockFlushStrategy,
			strategyFactory:  factory,
			logger:           logger.NewForTests(),
			metrics:          NewDefaultMetrics(logger.NewForTests()),
		}
		// Execute
		result, err := instance.PerformFlushWithStrategy(ctx, core.FlushingStrategyType(""))
		// Verify
		assert.NoError(t, err)
		assert.Equal(t, expectedOutput, result)
		mockStore.AssertExpectations(t)
		mockLockManager.AssertExpectations(t)
		mockFlushStrategy.AssertExpectations(t)
	})
	t.Run("Should validate strategy type when provided", func(t *testing.T) {
		factory := strategies.NewStrategyFactory()
		instance := &memoryInstance{
			strategyFactory: factory,
			logger:          logger.NewForTests(),
		}
		ctx := context.Background()
		// Test invalid strategy type
		result, err := instance.PerformFlushWithStrategy(ctx, core.FlushingStrategyType("invalid_strategy"))
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid strategy type")
		// Test empty string (should be valid - uses default)
		// This will fail because we need other dependencies, but we're just testing validation
		err = instance.validateStrategyType("")
		assert.NoError(t, err)
		// Test valid strategy types
		validTypes := []string{"fifo", "simple_fifo", "lru", "token_aware_lru"}
		for _, strategyType := range validTypes {
			err := instance.validateStrategyType(strategyType)
			assert.NoError(t, err, "Strategy type %s should be valid", strategyType)
		}
	})
	t.Run("Should use PerformFlushWithStrategy in PerformFlush", func(t *testing.T) {
		// This test verifies that PerformFlush delegates to PerformFlushWithStrategy
		mockStore := &mockStore{}
		mockLockManager := &mockLockManager{}
		mockFlushStrategy := &mockFlushStrategy{}
		factory := strategies.NewStrategyFactory()
		unlockFunc := func() error { return nil }
		ctx := context.Background()
		messages := []llm.Message{
			{Role: "user", Content: "test message"},
		}
		expectedOutput := &core.FlushMemoryActivityOutput{
			Success:      true,
			MessageCount: 1,
			TokenCount:   10,
		}
		// Setup expectations
		mockLockManager.On("AcquireFlushLock", ctx, "test-id").Return(unlockFunc, nil)
		mockStore.On("ReadMessages", ctx, "test-id").Return(messages, nil)
		mockFlushStrategy.On("PerformFlush", ctx, messages, (*core.Resource)(nil)).Return(expectedOutput, nil)
		instance := &memoryInstance{
			id:               "test-id",
			projectID:        "project-123",
			store:            mockStore,
			lockManager:      mockLockManager,
			flushingStrategy: mockFlushStrategy,
			strategyFactory:  factory,
			logger:           logger.NewForTests(),
			metrics:          NewDefaultMetrics(logger.NewForTests()),
		}
		// Execute using the regular PerformFlush method
		result, err := instance.PerformFlush(ctx)
		// Verify
		assert.NoError(t, err)
		assert.Equal(t, expectedOutput, result)
		mockStore.AssertExpectations(t)
		mockLockManager.AssertExpectations(t)
		mockFlushStrategy.AssertExpectations(t)
	})
}

// TestFlushHandlerDynamicStrategy tests the FlushHandler's dynamic strategy selection
func TestFlushHandlerDynamicStrategy(t *testing.T) {
	t.Run("Should select requested strategy when provided", func(t *testing.T) {
		mockStore := &mockStore{}
		mockLockManager := &mockLockManager{}
		mockTokenCounter := &mockTokenCounter{}
		factory := strategies.NewStrategyFactoryWithTokenCounter(mockTokenCounter)
		unlockFunc := func() error { return nil }
		ctx := context.Background()
		messages := []llm.Message{
			{Role: "user", Content: "message 1"},
			{Role: "assistant", Content: "response 1"},
			{Role: "user", Content: "message 2"},
		}
		resourceConfig := &core.Resource{
			MaxTokens: 1000,
			FlushingStrategy: &core.FlushingStrategyConfig{
				Type:               core.SimpleFIFOFlushing,
				SummarizeThreshold: 0.9,
			},
		}
		// Setup expectations
		mockLockManager.On("AcquireFlushLock", ctx, "test-instance").Return(unlockFunc, nil)
		mockStore.On("ReadMessages", ctx, "test-instance").Return(messages, nil)
		// For LRU strategy execution
		mockTokenCounter.On("CountTokens", ctx, mock.Anything).Return(10, nil).Maybe()
		// The LRU strategy will trim messages and update the store
		mockStore.On("TrimMessagesWithMetadata", ctx, "test-instance", mock.Anything, mock.Anything).Return(nil).Maybe()
		handler := &FlushHandler{
			instanceID:        "test-instance",
			projectID:         "project-123",
			store:             mockStore,
			lockManager:       mockLockManager,
			strategyFactory:   factory,
			requestedStrategy: "lru", // Request LRU strategy
			tokenCounter:      mockTokenCounter,
			logger:            logger.NewForTests(),
			metrics:           NewDefaultMetrics(logger.NewForTests()),
			resourceConfig:    resourceConfig,
		}
		// Execute
		result, err := handler.PerformFlush(ctx)
		// Verify - LRU strategy should have been created and used
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)
		mockStore.AssertExpectations(t)
		mockLockManager.AssertExpectations(t)
	})
	t.Run("Should use default strategy when no strategy requested", func(t *testing.T) {
		mockStore := &mockStore{}
		mockLockManager := &mockLockManager{}
		mockFlushStrategy := &mockFlushStrategy{}
		factory := strategies.NewStrategyFactory()
		unlockFunc := func() error { return nil }
		ctx := context.Background()
		messages := []llm.Message{
			{Role: "user", Content: "test message"},
		}
		expectedOutput := &core.FlushMemoryActivityOutput{
			Success:      true,
			MessageCount: 1,
			TokenCount:   10,
		}
		// Setup expectations
		mockLockManager.On("AcquireFlushLock", ctx, "test-instance").Return(unlockFunc, nil)
		mockStore.On("ReadMessages", ctx, "test-instance").Return(messages, nil)
		mockFlushStrategy.On("PerformFlush", ctx, messages, (*core.Resource)(nil)).Return(expectedOutput, nil)
		handler := &FlushHandler{
			instanceID:        "test-instance",
			projectID:         "project-123",
			store:             mockStore,
			lockManager:       mockLockManager,
			flushingStrategy:  mockFlushStrategy, // Default strategy
			strategyFactory:   factory,
			requestedStrategy: "", // No requested strategy
			logger:            logger.NewForTests(),
			metrics:           NewDefaultMetrics(logger.NewForTests()),
		}
		// Execute
		result, err := handler.PerformFlush(ctx)
		// Verify
		assert.NoError(t, err)
		assert.Equal(t, expectedOutput, result)
		mockStore.AssertExpectations(t)
		mockLockManager.AssertExpectations(t)
		mockFlushStrategy.AssertExpectations(t)
	})
	t.Run("Should use threshold from resource config", func(t *testing.T) {
		resourceConfig := &core.Resource{
			FlushingStrategy: &core.FlushingStrategyConfig{
				SummarizeThreshold: 0.85,
			},
		}
		handler := &FlushHandler{
			resourceConfig: resourceConfig,
		}
		threshold := handler.getThreshold()
		assert.Equal(t, 0.85, threshold)
	})
	t.Run("Should use default threshold when no config", func(t *testing.T) {
		handler := &FlushHandler{
			resourceConfig: nil,
		}
		threshold := handler.getThreshold()
		assert.Equal(t, 0.8, threshold)
	})
}

// Mock implementations are in mocks_test.go
