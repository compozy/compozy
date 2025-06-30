package testutil

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/mocks"

	"github.com/compozy/compozy/engine/autoload"
	enginecore "github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/memory"
	"github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/privacy"
	"github.com/compozy/compozy/engine/memory/store"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
)

// RedisTestClient wraps a redis.Client to implement the cache.RedisInterface
type RedisTestClient struct {
	*redis.Client
}

func (r *RedisTestClient) Pipeline() redis.Pipeliner {
	return r.Client.Pipeline()
}

// TestRedisSetup provides a complete Redis test environment
type TestRedisSetup struct {
	Server         *miniredis.Miniredis
	Client         *RedisTestClient
	Store          core.Store
	Manager        *memory.Manager
	ConfigRegistry *autoload.ConfigRegistry
}

// Cleanup cleans up all test resources
func (s *TestRedisSetup) Cleanup() {
	if s.Client != nil {
		s.Client.Close()
	}
	if s.Server != nil {
		s.Server.Close()
	}
}

// SetupTestRedis creates a complete Redis test environment with miniredis
func SetupTestRedis(t *testing.T) *TestRedisSetup {
	// Start miniredis server
	server, err := miniredis.Run()
	require.NoError(t, err)

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr: server.Addr(),
	})

	// Create test client wrapper
	testClient := &RedisTestClient{Client: client}

	// Create Redis store
	redisStore := store.NewRedisMemoryStore(testClient, "test")

	// Create lock manager
	lockManager, err := cache.NewRedisLockManager(testClient)
	require.NoError(t, err)

	// Create logger for tests
	log := logger.NewForTests()

	// Create template engine
	tplEngine := tplengine.NewEngine(tplengine.FormatText)

	// Create config registry
	configRegistry := autoload.NewConfigRegistry()

	// Create privacy manager
	privacyManager := privacy.NewManager()

	// Create mock temporal client
	mockClient := &mocks.Client{}

	// Create memory manager options
	opts := &memory.ManagerOptions{
		ResourceRegistry:  configRegistry,
		TplEngine:         tplEngine,
		BaseLockManager:   lockManager,
		BaseRedisClient:   testClient,
		TemporalClient:    mockClient,
		TemporalTaskQueue: "test-memory-queue",
		PrivacyManager:    privacyManager,
		Logger:            log,
	}

	// Create memory manager
	manager, err := memory.NewManager(opts)
	require.NoError(t, err)

	return &TestRedisSetup{
		Server:         server,
		Client:         testClient,
		Store:          redisStore,
		Manager:        manager,
		ConfigRegistry: configRegistry,
	}
}

// CreateTestMemoryInstance creates a memory instance for testing
func (s *TestRedisSetup) CreateTestMemoryInstance(t *testing.T, instanceID string) core.Memory {
	// First register a test memory configuration
	testConfig := &memory.Config{
		Resource:    "memory",
		ID:          instanceID,
		Type:        core.TokenBasedMemory,
		Description: "Test memory instance",
		MaxTokens:   4000,
		MaxMessages: 100,
		Persistence: core.PersistenceConfig{
			Type: core.RedisPersistence,
			TTL:  "24h",
		},
	}

	// Validate config to set ParsedTTL
	err := testConfig.Validate()
	require.NoError(t, err)

	// Register the config
	err = s.ConfigRegistry.Register(testConfig, "test")
	require.NoError(t, err)

	// Create memory reference
	memRef := enginecore.MemoryReference{
		ID:          instanceID,
		Key:         instanceID,
		ResolvedKey: instanceID,
		Mode:        "read-write",
	}

	// Get instance from manager
	memInstance, err := s.Manager.GetInstance(context.Background(), memRef, map[string]any{})
	require.NoError(t, err)

	return memInstance
}

// CreateTestMemoryInstanceWithFlush creates a flushable memory instance for testing
func (s *TestRedisSetup) CreateTestMemoryInstanceWithFlush(t *testing.T, instanceID string) core.FlushableMemory {
	memInstance := s.CreateTestMemoryInstance(t, instanceID)

	flushableInstance, ok := memInstance.(core.FlushableMemory)
	require.True(t, ok, "Memory instance should implement FlushableMemory")

	return flushableInstance
}

// SetupTestMemoryManager creates a lightweight memory manager for testing
func SetupTestMemoryManager(t *testing.T) (*memory.Manager, func()) {
	setup := SetupTestRedis(t)

	return setup.Manager, setup.Cleanup
}

// SetupTestStore creates just a Redis store for testing
func SetupTestStore(t *testing.T) (core.Store, func()) {
	setup := SetupTestRedis(t)

	return setup.Store, setup.Cleanup
}

// SetupTestStoreWithPrefix creates a Redis store with a custom prefix
func SetupTestStoreWithPrefix(t *testing.T, prefix string) (core.Store, func()) {
	// Start miniredis server
	server, err := miniredis.Run()
	require.NoError(t, err)

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr: server.Addr(),
	})

	// Create test client wrapper
	testClient := &RedisTestClient{Client: client}

	// Create Redis store with custom prefix
	redisStore := store.NewRedisMemoryStore(testClient, prefix)

	cleanup := func() {
		client.Close()
		server.Close()
	}

	return redisStore, cleanup
}
