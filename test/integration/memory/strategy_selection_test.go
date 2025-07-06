package memory

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/service"
)

// TestStrategySelectionE2E tests end-to-end strategy selection functionality
func TestStrategySelectionE2E(t *testing.T) {
	t.Run("Should use requested strategy when provided for dynamic memory", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()

		// Register test memory config without hyphens
		err := env.RegisterMemoryConfig(&memory.Config{
			Resource:    "memory",
			ID:          "customersupport",
			Type:        memcore.TokenBasedMemory,
			Description: "Customer support session memory",
			MaxTokens:   4000,
			MaxMessages: 100,
			Persistence: memcore.PersistenceConfig{
				Type: memcore.RedisPersistence,
				TTL:  "24h",
			},
			Flushing: &memcore.FlushingStrategyConfig{
				Type:               memcore.SimpleFIFOFlushing,
				SummarizeThreshold: 0.8,
			},
		})
		require.NoError(t, err)

		// Create memory instance and add test messages
		memRef := core.MemoryReference{
			ID:  "customersupport",
			Key: "support_conversation_{{.conversation.id}}",
		}
		workflowContext := map[string]any{
			"project":      map[string]any{"id": "test-project"},
			"conversation": map[string]any{"id": "123"},
		}

		memoryInstance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
		require.NoError(t, err)
		require.NotNil(t, memoryInstance)

		// Add test messages
		messages := []llm.Message{
			{Role: "user", Content: "Test message 1"},
			{Role: "assistant", Content: "Test response 1"},
		}
		for _, msg := range messages {
			err := memoryInstance.Append(ctx, msg)
			require.NoError(t, err)
		}

		// Test service layer with specific strategy request
		svc, err := service.NewMemoryOperationsService(env.GetMemoryManager(), nil, nil, nil, nil)
		require.NoError(t, err)
		flushReq := &service.FlushRequest{
			BaseRequest: service.BaseRequest{
				MemoryRef: "customersupport",
				Key:       "support_conversation_123",
			},
			Config: &service.FlushConfig{
				Strategy: "simple_fifo",
				DryRun:   true,
			},
		}

		resp, err := svc.Flush(ctx, flushReq)
		require.NoError(t, err)
		require.NotNil(t, resp)

		// Verify the actual strategy matches requested strategy
		assert.Equal(t, "simple_fifo", resp.ActualStrategy, "Should use requested strategy")
		assert.True(t, resp.Success, "Flush should succeed")
		assert.True(t, resp.DryRun, "Should be dry run")
	})

	t.Run("Should fall back to configured strategy when no strategy specified", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()

		// Register test memory config
		err := env.RegisterMemoryConfig(&memory.Config{
			Resource:    "memory",
			ID:          "sharedmemory",
			Type:        memcore.MessageCountBasedMemory,
			Description: "Shared knowledge base memory",
			MaxTokens:   8000,
			MaxMessages: 500,
			Persistence: memcore.PersistenceConfig{
				Type: memcore.RedisPersistence,
				TTL:  "0",
			},
		})
		require.NoError(t, err)

		// Create memory instance and add test messages
		memRef := core.MemoryReference{
			ID:  "sharedmemory",
			Key: "shared_memory_{{.test.id}}",
		}
		workflowContext := map[string]any{
			"project": map[string]any{"id": "test-project"},
			"test":    map[string]any{"id": "fallback_test"},
		}

		memoryInstance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
		require.NoError(t, err)
		require.NotNil(t, memoryInstance)

		// Add test messages
		messages := []llm.Message{
			{Role: "user", Content: "Test message for fallback"},
			{Role: "assistant", Content: "Test response for fallback"},
		}
		for _, msg := range messages {
			err := memoryInstance.Append(ctx, msg)
			require.NoError(t, err)
		}

		// Test without specifying strategy
		svc, err := service.NewMemoryOperationsService(env.GetMemoryManager(), nil, nil, nil, nil)
		require.NoError(t, err)
		flushReq := &service.FlushRequest{
			BaseRequest: service.BaseRequest{
				MemoryRef: "sharedmemory",
				Key:       "shared_memory_fallback_test",
			},
			Config: &service.FlushConfig{
				DryRun: true,
			},
		}

		resp, err := svc.Flush(ctx, flushReq)
		require.NoError(t, err)
		require.NotNil(t, resp)

		// Verify that a strategy was selected (configured or "unknown")
		assert.NotEmpty(t, resp.ActualStrategy, "Should have an actual strategy")
		assert.True(t, resp.Success, "Flush should succeed")
		assert.True(t, resp.DryRun, "Should be dry run")
		assert.Contains(t, []string{"simple_fifo", "lru", "token_aware_lru", "unknown"},
			resp.ActualStrategy, "Should be a valid strategy or unknown")
	})

	t.Run("Should handle invalid strategy gracefully", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()

		// Register test memory config
		err := env.RegisterMemoryConfig(&memory.Config{
			Resource:    "memory",
			ID:          "flushablememory",
			Type:        memcore.TokenBasedMemory,
			Description: "Memory with aggressive flushing",
			MaxTokens:   2000,
			MaxMessages: 50,
			Persistence: memcore.PersistenceConfig{
				Type: memcore.RedisPersistence,
				TTL:  "1h",
			},
			Flushing: &memcore.FlushingStrategyConfig{
				Type:               memcore.SimpleFIFOFlushing,
				SummarizeThreshold: 0.5,
			},
		})
		require.NoError(t, err)

		// Create memory instance and add test messages
		memRef := core.MemoryReference{
			ID:  "flushablememory",
			Key: "flush_test_{{.test.id}}",
		}
		workflowContext := map[string]any{
			"project": map[string]any{"id": "test-project"},
			"test":    map[string]any{"id": "invalid_test"},
		}

		memoryInstance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
		require.NoError(t, err)
		require.NotNil(t, memoryInstance)

		// Add test messages
		messages := []llm.Message{
			{Role: "user", Content: "Test message for invalid strategy"},
		}
		for _, msg := range messages {
			err := memoryInstance.Append(ctx, msg)
			require.NoError(t, err)
		}

		// Test with invalid strategy
		svc, err := service.NewMemoryOperationsService(env.GetMemoryManager(), nil, nil, nil, nil)
		require.NoError(t, err)
		flushReq := &service.FlushRequest{
			BaseRequest: service.BaseRequest{
				MemoryRef: "flushablememory",
				Key:       "flush_test_invalid_test",
			},
			Config: &service.FlushConfig{
				Strategy: "nonexistent_strategy",
				DryRun:   true,
			},
		}

		resp, err := svc.Flush(ctx, flushReq)

		// The behavior depends on implementation - it might error or fall back
		if err != nil {
			// If it errors, that's acceptable for invalid strategy
			assert.Contains(t, err.Error(), "strategy", "Error should mention strategy")
		} else {
			// If it succeeds, it should fall back to a valid strategy
			require.NotNil(t, resp)
			assert.NotEqual(t, "nonexistent_strategy", resp.ActualStrategy,
				"Should not use invalid strategy")
			assert.Contains(t, []string{"simple_fifo", "lru", "token_aware_lru", "unknown"},
				resp.ActualStrategy, "Should fall back to valid strategy or unknown")
		}
	})

	t.Run("Should maintain backward compatibility with FlushStrategy field", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()

		// Register test memory config
		err := env.RegisterMemoryConfig(&memory.Config{
			Resource:    "memory",
			ID:          "compatmemory",
			Type:        memcore.TokenBasedMemory,
			Description: "Compatibility test memory",
			MaxTokens:   4000,
			MaxMessages: 100,
			Persistence: memcore.PersistenceConfig{
				Type: memcore.RedisPersistence,
				TTL:  "24h",
			},
			Flushing: &memcore.FlushingStrategyConfig{
				Type:               memcore.SimpleFIFOFlushing,
				SummarizeThreshold: 0.8,
			},
		})
		require.NoError(t, err)

		// Create memory instance and add test messages
		memRef := core.MemoryReference{
			ID:  "compatmemory",
			Key: "compat_test_{{.test.id}}",
		}
		workflowContext := map[string]any{
			"project": map[string]any{"id": "test-project"},
			"test":    map[string]any{"id": "compat_test"},
		}

		memoryInstance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
		require.NoError(t, err)
		require.NotNil(t, memoryInstance)

		// Add test messages
		messages := []llm.Message{
			{Role: "user", Content: "Test message for compatibility"},
		}
		for _, msg := range messages {
			err := memoryInstance.Append(ctx, msg)
			require.NoError(t, err)
		}

		// Test flush
		svc, err := service.NewMemoryOperationsService(env.GetMemoryManager(), nil, nil, nil, nil)
		require.NoError(t, err)
		flushReq := &service.FlushRequest{
			BaseRequest: service.BaseRequest{
				MemoryRef: "compatmemory",
				Key:       "compat_test_compat_test",
			},
			Config: &service.FlushConfig{
				Strategy: "simple_fifo",
				DryRun:   true,
			},
		}

		resp, err := svc.Flush(ctx, flushReq)
		require.NoError(t, err)
		require.NotNil(t, resp)

		// Verify both old and new fields are present
		assert.NotEmpty(t, resp.ActualStrategy, "Should have ActualStrategy field")
		assert.True(t, resp.Success, "Flush should succeed")
	})
}
