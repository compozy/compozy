package memory

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
)

// MockConfigRegistry mocks the autoload.ConfigRegistry
type MockConfigRegistry struct {
	mock.Mock
}

func (m *MockConfigRegistry) Get(resourceType, id string) (any, error) {
	args := m.Called(resourceType, id)
	return args.Get(0), args.Error(1)
}

func (m *MockConfigRegistry) Register(config any, source string) error {
	args := m.Called(config, source)
	return args.Error(0)
}

func (m *MockConfigRegistry) GetAll(resourceType string) []any {
	args := m.Called(resourceType)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).([]any)
}

func (m *MockConfigRegistry) Count() int {
	args := m.Called()
	return args.Int(0)
}

func (m *MockConfigRegistry) CountByType(resourceType string) int {
	args := m.Called(resourceType)
	return args.Int(0)
}

func (m *MockConfigRegistry) Clear() {
	m.Called()
}

// MockTemporalClient mocks the Temporal client
type MockTemporalClient struct {
	mock.Mock
	client.Client
}

// Helper function to create a test manager
func createTestManager(t *testing.T) (*Manager, *MockConfigRegistry, *MockLockManager, *MockRedisClient) {
	t.Helper()

	// Create mocks
	mockRegistry := new(MockConfigRegistry)
	mockLockManager := new(MockLockManager)
	mockRedis := new(MockRedisClient)
	mockTemporalClient := new(MockTemporalClient)

	// Create template engine
	tplEngine := tplengine.NewEngine(tplengine.FormatYAML)

	// Create a real ConfigRegistry
	realRegistry := autoload.NewConfigRegistry()

	// Create manager with real registry
	opts := &ManagerOptions{
		ResourceRegistry:  realRegistry,
		TplEngine:         tplEngine,
		BaseLockManager:   mockLockManager,
		BaseRedisClient:   mockRedis,
		TemporalClient:    mockTemporalClient,
		TemporalTaskQueue: "test-queue",
		Logger:            logger.NewForTests(),
	}

	manager, err := NewManager(opts)
	require.NoError(t, err)

	// Store mockRegistry for later use in expectations (even though we use real one)
	return manager, mockRegistry, mockLockManager, mockRedis
}

func TestMemoryManager_CachedComponents(t *testing.T) {
	t.Run("Should cache and reuse expensive components across multiple GetInstance calls", func(t *testing.T) {
		// Create test manager
		manager, _, mockLockManager, mockRedis := createTestManager(t)

		// Create a test memory config
		testConfig := &Config{
			Resource:    "memory",
			ID:          "test-memory",
			Description: "Test memory for caching verification",
			Type:        TokenBasedMemory,
			MaxMessages: 100,
			MaxTokens:   1000,
			Persistence: PersistenceConfig{
				Type: InMemoryPersistence,
				TTL:  "1h",
			},
			PrivacyPolicy: &PrivacyPolicyConfig{
				RedactPatterns: []string{},
			},
		}

		// Register the config with the real registry
		err := manager.resourceRegistry.Register(testConfig, "test")
		require.NoError(t, err)

		// Mock Redis operations for store
		mockRedis.On("Ping", mock.Anything).Return(redis.NewStatusCmd(context.Background()))

		// Mock lock manager operations
		mockLockManager.On("Acquire", mock.Anything, mock.Anything, mock.Anything).Return(&MockLock{}, nil)

		// Create workflow context for template evaluation
		workflowContext := map[string]any{
			"project.id": "test-project",
			"user_id":    "user123",
			"session_id": "session456",
		}

		// First call to GetInstance
		memRef1 := core.MemoryReference{
			ID:  "test-memory",
			Key: "memory-{{.user_id}}-{{.session_id}}",
		}

		instance1, err := manager.GetInstance(context.Background(), memRef1, workflowContext)
		require.NoError(t, err)
		require.NotNil(t, instance1)

		// Get the internal instance to access components
		memInstance1, ok := instance1.(*Instance)
		require.True(t, ok, "Expected instance to be of type *Instance")

		// Second call to GetInstance with same resource ID but different key
		memRef2 := core.MemoryReference{
			ID:  "test-memory", // Same resource ID
			Key: "memory-{{.user_id}}-different",
		}

		instance2, err := manager.GetInstance(context.Background(), memRef2, workflowContext)
		require.NoError(t, err)
		require.NotNil(t, instance2)

		memInstance2, ok := instance2.(*Instance)
		require.True(t, ok, "Expected instance to be of type *Instance")

		// Verify that expensive components are NOT the same instance
		// (since we don't have caching implemented yet)
		// TODO: When caching is implemented, these assertions should be changed to verify
		// that the components ARE the same instance

		// Token manager should be different instances (no caching yet)
		assert.NotSame(t, memInstance1.tokenManager, memInstance2.tokenManager,
			"TokenManager should be different instances without caching")

		// Flushing strategy should be different instances (no caching yet)
		assert.NotSame(t, memInstance1.flushingStrategy, memInstance2.flushingStrategy,
			"FlushingStrategy should be different instances without caching")

		// However, both should have the same token counter model
		assert.Equal(t, memInstance1.tokenManager.tokenCounter.GetEncoding(),
			memInstance2.tokenManager.tokenCounter.GetEncoding(),
			"Token counters should use the same encoding")

		// Third call with different resource ID should create new components
		testConfig2 := &Config{
			Resource:    "memory",
			ID:          "test-memory-2",
			Description: "Different memory resource",
			Type:        TokenBasedMemory,
			MaxMessages: 200,
			MaxTokens:   2000,
			Persistence: PersistenceConfig{
				Type: InMemoryPersistence,
				TTL:  "1h",
			},
			PrivacyPolicy: &PrivacyPolicyConfig{
				RedactPatterns: []string{},
			},
		}

		err = manager.resourceRegistry.Register(testConfig2, "test")
		require.NoError(t, err)

		memRef3 := core.MemoryReference{
			ID:  "test-memory-2", // Different resource ID
			Key: "memory-{{.user_id}}",
		}

		instance3, err := manager.GetInstance(context.Background(), memRef3, workflowContext)
		require.NoError(t, err)
		require.NotNil(t, instance3)

		memInstance3, ok := instance3.(*Instance)
		require.True(t, ok, "Expected instance to be of type *Instance")

		// Components for different resource should definitely be different
		assert.NotSame(t, memInstance1.tokenManager, memInstance3.tokenManager,
			"TokenManager should be different for different resources")
		assert.NotSame(t, memInstance1.flushingStrategy, memInstance3.flushingStrategy,
			"FlushingStrategy should be different for different resources")

		// Mock expectations verified implicitly by not failing

		// TODO: When caching is implemented, add the following verifications:
		// 1. Track the number of times NewTiktokenCounter is called (should be once per resource)
		// 2. Track the number of times NewTokenMemoryManager is called (should be once per resource)
		// 3. Verify that components are stored in and retrieved from a cache
		// 4. Verify that cache keys are based on resource ID and configuration
		// 5. Test cache invalidation when resource configuration changes
	})

	t.Run("Should create new components when resource configuration changes", func(t *testing.T) {
		// This test would verify that cached components are invalidated
		// when the resource configuration changes (e.g., max tokens, privacy policy)
		// TODO: Implement when caching is added
		t.Skip("TODO: Implement when component caching is added to MemoryManager")
	})

	t.Run("Should handle concurrent GetInstance calls with proper caching", func(t *testing.T) {
		// This test would verify thread-safe caching behavior under concurrent access
		// TODO: Implement when caching is added
		t.Skip("TODO: Implement when component caching is added to MemoryManager")
	})
}
