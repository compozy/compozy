package memory

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/mocks"

	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/memory"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/privacy"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
)

// TestEnvironment provides a complete test environment for memory integration tests
type TestEnvironment struct {
	ctx            context.Context
	logger         logger.Logger
	miniredis      *miniredis.Miniredis
	redis          *redis.Client
	redisClient    cache.RedisInterface
	lockManager    cache.LockManager
	temporalClient client.Client
	memoryManager  *memory.Manager
	configRegistry *autoload.ConfigRegistry
	tplEngine      *tplengine.TemplateEngine
	cleanup        []func()
}

// NewTestEnvironment creates a new test environment with all required dependencies
func NewTestEnvironment(t *testing.T) *TestEnvironment {
	t.Helper()
	ctx := context.Background()
	log := logger.NewForTests()
	env := &TestEnvironment{
		ctx:     ctx,
		logger:  log,
		cleanup: []func(){},
	}
	// Setup Redis
	env.setupRedis(t)
	// Setup Temporal (using test suite for now)
	env.setupTemporal(t)
	// Setup registries
	env.setupRegistries(t)
	// Setup template engine
	env.setupTemplateEngine(t)
	// Setup memory manager
	env.setupMemoryManager(t)
	return env
}

func (env *TestEnvironment) setupRedis(t *testing.T) {
	t.Helper()
	// Create miniredis instance
	mr, err := miniredis.Run()
	require.NoError(t, err, "Failed to start miniredis")
	env.miniredis = mr
	// Create Redis client connected to miniredis
	env.redis = redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
		DB:   0,
	})
	// Test connection
	ctx, cancel := context.WithTimeout(env.ctx, 5*time.Second)
	defer cancel()
	err = env.redis.Ping(ctx).Err()
	require.NoError(t, err, "Failed to connect to miniredis")
	// Redis client already implements RedisInterface
	env.redisClient = env.redis
	// Create lock manager
	lockManager, err := cache.NewRedisLockManager(env.redisClient)
	require.NoError(t, err)
	env.lockManager = lockManager
	// Cleanup function
	env.cleanup = append(env.cleanup, func() {
		_ = env.redis.Close()
		env.miniredis.Close()
	})
}

func (env *TestEnvironment) setupTemporal(t *testing.T) {
	t.Helper()
	// For memory integration tests, we create a minimal mock client
	// Since most memory operations don't actually use Temporal workflows,
	// this provides just enough to satisfy the interface requirement
	mockClient := &mocks.Client{}
	env.temporalClient = mockClient

	// Add cleanup
	env.cleanup = append(env.cleanup, func() {
		// No cleanup needed for mock client
	})
}

func (env *TestEnvironment) setupRegistries(t *testing.T) {
	t.Helper()
	// Create test registries
	env.configRegistry = autoload.NewConfigRegistry()
	// Add test memory configurations
	env.addTestMemoryConfigs()
}

func (env *TestEnvironment) setupTemplateEngine(t *testing.T) {
	t.Helper()
	env.tplEngine = tplengine.NewEngine(tplengine.FormatText)
}

func (env *TestEnvironment) setupMemoryManager(t *testing.T) {
	t.Helper()
	// Create privacy manager
	privacyManager := privacy.NewManager()
	// Create memory manager options
	opts := &memory.ManagerOptions{
		ResourceRegistry:  env.configRegistry,
		TplEngine:         env.tplEngine,
		BaseLockManager:   env.lockManager,
		BaseRedisClient:   env.redisClient,
		TemporalClient:    env.temporalClient,
		TemporalTaskQueue: "test-memory-queue",
		PrivacyManager:    privacyManager,
		Logger:            env.logger,
		ComponentCacheConfig: &memory.ComponentCacheConfig{
			MaxCost:     10 << 20, // 10MB for tests
			NumCounters: 1e5,      // 100k counters
			BufferItems: 64,
		},
	}
	// Create memory manager
	var err error
	env.memoryManager, err = memory.NewManager(opts)
	require.NoError(t, err)
}

// Cleanup cleans up all test resources
func (env *TestEnvironment) Cleanup() {
	for i := len(env.cleanup) - 1; i >= 0; i-- {
		env.cleanup[i]()
	}
}

// GetMemoryManager returns the memory manager instance
func (env *TestEnvironment) GetMemoryManager() *memory.Manager {
	return env.memoryManager
}

// GetRedis returns the Redis client
func (env *TestEnvironment) GetRedis() *redis.Client {
	return env.redis
}

// GetLogger returns the logger
func (env *TestEnvironment) GetLogger() logger.Logger {
	return env.logger
}

// addTestMemoryConfigs adds test-specific memory configurations
func (env *TestEnvironment) addTestMemoryConfigs() {
	// Register test memory resources
	testConfigs := []struct {
		config *memory.Config
	}{
		{
			config: &memory.Config{
				Resource:    "memory",
				ID:          "customer-support",
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
			},
		},
		{
			config: &memory.Config{
				Resource:    "memory",
				ID:          "shared-memory",
				Type:        memcore.MessageCountBasedMemory,
				Description: "Shared knowledge base memory",
				MaxTokens:   8000,
				MaxMessages: 500,
				Persistence: memcore.PersistenceConfig{
					Type: memcore.RedisPersistence,
					TTL:  "0", // No expiration
				},
				Locking: &memcore.LockConfig{
					AppendTTL: "30s",
					ClearTTL:  "60s",
					FlushTTL:  "120s",
				},
			},
		},
		{
			config: &memory.Config{
				Resource:    "memory",
				ID:          "flushable-memory",
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
					SummarizeThreshold: 0.5, // Aggressive flushing at 50%
				},
			},
		},
	}
	for _, tc := range testConfigs {
		// Validate config to set ParsedTTL
		if err := tc.config.Validate(); err != nil {
			panic(fmt.Sprintf("Failed to validate test config %s: %v", tc.config.ID, err))
		}
		err := env.configRegistry.Register(tc.config, "test")
		if err != nil {
			panic(fmt.Sprintf("Failed to register test config: %v", err))
		}
	}
}
