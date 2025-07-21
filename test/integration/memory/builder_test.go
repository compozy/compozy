package memory

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/mocks"

	coreTypes "github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/instance"
	"github.com/compozy/compozy/engine/memory/store"
)

func TestBuilder_Integration(t *testing.T) {
	t.Run("Should successfully build memory instance through manager", func(t *testing.T) {
		// This test verifies successful builder operation through the manager
		// which is the typical integration pattern
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()
		// Use the manager to get a memory instance, which internally uses the builder
		memRef := coreTypes.MemoryReference{
			ID:  "customer-support", // This references a pre-configured memory resource
			Key: "test-builder-{{.test.id}}",
		}
		workflowContext := map[string]any{
			"project": map[string]any{
				"id": "test-project",
			},
			"test": map[string]any{
				"id": "builder-123",
			},
		}
		memInstance, err := env.GetMemoryManager().GetInstance(ctx, memRef, workflowContext)
		require.NoError(t, err)
		require.NotNil(t, memInstance)
		// Verify the instance is functional
		msg := llm.Message{
			Role:    "user",
			Content: "Test message from builder integration test",
		}
		err = memInstance.Append(ctx, msg)
		assert.NoError(t, err)
		// Read back the message
		messages, err := memInstance.Read(ctx)
		assert.NoError(t, err)
		assert.Len(t, messages, 1)
		assert.Equal(t, "Test message from builder integration test", messages[0].Content)
	})
	t.Run("Should successfully build instance with direct builder", func(t *testing.T) {
		// This test verifies the builder can be used directly with minimal components
		// Note: Full integration with all components is tested via the manager
		ctx := context.Background()
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		// Create a simple memory resource configuration
		resource := &memcore.Resource{
			ID:          "test-memory-direct",
			Type:        memcore.MessageCountBasedMemory,
			MaxMessages: 10,
			Persistence: memcore.PersistenceConfig{
				TTL: "1h",
			},
		}
		// Create all required components using test doubles
		memoryStore := store.NewRedisMemoryStore(env.redisClient, "test:builder:direct")
		lockManager := instance.NewLockManager(&simpleLocker{})
		// Create minimal token counter
		tokenCounter := &minimalTokenCounter{}
		// Create minimal flushing strategy
		flushStrategy := &minimalFlushStrategy{}
		// Create minimal eviction policy
		evictionPolicy := &minimalEvictionPolicy{}
		// Build memory instance directly
		memInstance, err := instance.NewBuilder().
			WithInstanceID("direct-instance-123").
			WithResourceID("test-memory-direct").
			WithProjectID("test-project").
			WithResourceConfig(resource).
			WithStore(memoryStore).
			WithLockManager(lockManager).
			WithTokenCounter(tokenCounter).
			WithFlushingStrategy(flushStrategy).
			WithEvictionPolicy(evictionPolicy).
			WithTemporalClient(env.temporalClient).
			WithTemporalTaskQueue("test-queue").
			Build(ctx)
		// Verify successful creation
		require.NoError(t, err)
		require.NotNil(t, memInstance)
		// Verify instance properties
		assert.Equal(t, "direct-instance-123", memInstance.GetID())
		assert.Equal(t, resource, memInstance.GetResource())
		assert.Equal(t, memoryStore, memInstance.GetStore())
		assert.NotNil(t, memInstance.GetTokenCounter())
		assert.NotNil(t, memInstance.GetMetrics())
		assert.Equal(t, lockManager, memInstance.GetLockManager())
		// Test basic operations
		msg := llm.Message{
			Role:    "assistant",
			Content: "Hello from direct builder test!",
		}
		err = memInstance.Append(ctx, msg)
		assert.NoError(t, err)
		// Verify message was stored
		messages, err := memInstance.Read(ctx)
		assert.NoError(t, err)
		assert.Len(t, messages, 1)
		assert.Equal(t, "Hello from direct builder test!", messages[0].Content)
		// Test token count
		tokenCount, err := memInstance.GetTokenCount(ctx)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, tokenCount, 0)
	})
	t.Run("Should work with mock temporal client", func(t *testing.T) {
		// This test specifically verifies that the builder works with
		// the standard temporal mock client used in integration tests
		ctx := context.Background()
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		// Create minimal configuration
		resource := &memcore.Resource{
			ID:        "test-memory-mock-temporal",
			Type:      memcore.TokenBasedMemory,
			MaxTokens: 500,
			Persistence: memcore.PersistenceConfig{
				TTL: "30m",
			},
		}
		// Use mocks.Client directly
		mockTemporalClient := &mocks.Client{}
		// Build with mock client and minimal dependencies
		memInstance, err := instance.NewBuilder().
			WithInstanceID("mock-temporal-instance").
			WithResourceConfig(resource).
			WithStore(store.NewRedisMemoryStore(env.redisClient, "mock:temporal")).
			WithLockManager(instance.NewLockManager(&simpleLocker{})).
			WithTokenCounter(&minimalTokenCounter{}).
			WithFlushingStrategy(&minimalFlushStrategy{}).
			WithTemporalClient(mockTemporalClient).
			Build(ctx)
		require.NoError(t, err)
		require.NotNil(t, memInstance)
		// Verify the instance works with the mock client
		assert.Equal(t, "mock-temporal-instance", memInstance.GetID())
	})
}

// Minimal test doubles for builder testing

type minimalTokenCounter struct{}

func (tc *minimalTokenCounter) CountTokens(_ context.Context, text string) (int, error) {
	// Simple estimation: ~4 chars per token
	return len(text) / 4, nil
}

func (tc *minimalTokenCounter) EncodeTokens(_ context.Context, _ string) ([]int, error) {
	return []int{}, nil
}

func (tc *minimalTokenCounter) DecodeTokens(_ context.Context, _ []int) (string, error) {
	return "", nil
}

func (tc *minimalTokenCounter) GetEncoding() string {
	return "test-encoding"
}

type minimalFlushStrategy struct{}

func (fs *minimalFlushStrategy) ShouldFlush(_ int, _ int, _ *memcore.Resource) bool {
	return false
}

func (fs *minimalFlushStrategy) PerformFlush(
	_ context.Context,
	messages []llm.Message,
	_ *memcore.Resource,
) (*memcore.FlushMemoryActivityOutput, error) {
	return &memcore.FlushMemoryActivityOutput{
		Success:      true,
		MessageCount: len(messages),
	}, nil
}

func (fs *minimalFlushStrategy) GetType() memcore.FlushingStrategyType {
	return "minimal"
}

type minimalEvictionPolicy struct{}

func (ep *minimalEvictionPolicy) SelectMessagesToEvict(messages []llm.Message, targetCount int) []llm.Message {
	if len(messages) <= targetCount {
		return messages
	}
	return messages[:targetCount]
}

func (ep *minimalEvictionPolicy) GetType() string {
	return "minimal"
}

// simpleLocker is a basic in-memory locker for testing
type simpleLocker struct{}

func (l *simpleLocker) Lock(_ context.Context, _ string, _ time.Duration) (instance.Lock, error) {
	return &simpleLock{}, nil
}

type simpleLock struct{}

func (l *simpleLock) Unlock(_ context.Context) error {
	return nil
}
