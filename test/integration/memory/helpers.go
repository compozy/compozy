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
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/llm"
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
	log := logger.NewForTests()
	env := &TestEnvironment{
		ctx:     t.Context(),
		logger:  log,
		cleanup: []func(){},
	}
	env.setupRedis(t)
	// NOTE: Wire a minimal Temporal mock to satisfy manager dependencies during tests.
	env.setupTemporal(t)
	env.setupRegistries(t)
	env.setupTemplateEngine(t)
	env.setupMemoryManager(t)
	return env
}

func (env *TestEnvironment) setupRedis(t *testing.T) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err, "Failed to start miniredis")
	env.miniredis = mr
	env.redis = redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
		DB:   0,
	})
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	err = env.redis.Ping(ctx).Err()
	require.NoError(t, err, "Failed to connect to miniredis")
	env.redisClient = env.redis
	lockManager, err := cache.NewRedisLockManager(env.redisClient)
	require.NoError(t, err)
	env.lockManager = lockManager
	env.cleanup = append(env.cleanup, func() {
		_ = env.redis.Close()
		env.miniredis.Close()
	})
}

func (env *TestEnvironment) setupTemporal(t *testing.T) {
	t.Helper()
	mockClient := &mocks.Client{}
	env.temporalClient = mockClient
	env.cleanup = append(env.cleanup, func() {
	})
}

func (env *TestEnvironment) setupRegistries(t *testing.T) {
	t.Helper()
	env.configRegistry = autoload.NewConfigRegistry()
	env.addTestMemoryConfigs()
}

func (env *TestEnvironment) setupTemplateEngine(t *testing.T) {
	t.Helper()
	env.tplEngine = tplengine.NewEngine(tplengine.FormatText)
}

func (env *TestEnvironment) setupMemoryManager(t *testing.T) {
	t.Helper()
	privacyManager := privacy.NewManager()
	opts := &memory.ManagerOptions{
		ResourceRegistry:  env.configRegistry,
		TplEngine:         env.tplEngine,
		BaseLockManager:   env.lockManager,
		BaseRedisClient:   env.redisClient,
		TemporalClient:    env.temporalClient,
		TemporalTaskQueue: "test-memory-queue",
		PrivacyManager:    privacyManager,
		FallbackProjectID: "basic-memory", // Use the same project ID as the examples
	}
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

// GetConfigRegistry returns the config registry
func (env *TestEnvironment) GetConfigRegistry() *autoload.ConfigRegistry {
	return env.configRegistry
}

// RegisterMemoryConfig registers a memory configuration for testing
func (env *TestEnvironment) RegisterMemoryConfig(ctx context.Context, config *memory.Config) error {
	if err := config.Validate(ctx); err != nil {
		return fmt.Errorf("failed to validate config %s: %w", config.ID, err)
	}
	return env.configRegistry.Register(config, "test")
}

// addTestMemoryConfigs adds test-specific memory configurations
func (env *TestEnvironment) addTestMemoryConfigs() {
	configs := []*memory.Config{
		newCustomerSupportMemoryConfig(),
		newSharedMemoryConfig(),
		newFlushableMemoryConfig(),
	}
	for _, cfg := range configs {
		env.mustRegisterTestConfig(cfg)
	}
}

func newCustomerSupportMemoryConfig() *memory.Config {
	return &memory.Config{
		Resource:    "memory",
		ID:          "customer-support",
		Type:        memcore.TokenBasedMemory,
		Description: "Customer support session memory",
		MaxTokens:   4000,
		MaxMessages: 100,
		Persistence: memcore.PersistenceConfig{Type: memcore.RedisPersistence, TTL: "24h"},
		Flushing: &memcore.FlushingStrategyConfig{
			Type:               memcore.SimpleFIFOFlushing,
			SummarizeThreshold: 0.8,
		},
	}
}

func newSharedMemoryConfig() *memory.Config {
	return &memory.Config{
		Resource:    "memory",
		ID:          "shared-memory",
		Type:        memcore.MessageCountBasedMemory,
		Description: "Shared knowledge base memory",
		MaxTokens:   8000,
		MaxMessages: 500,
		Persistence: memcore.PersistenceConfig{Type: memcore.RedisPersistence, TTL: "0"},
		Locking: &memcore.LockConfig{
			AppendTTL: "30s",
			ClearTTL:  "60s",
			FlushTTL:  "120s",
		},
	}
}

func newFlushableMemoryConfig() *memory.Config {
	return &memory.Config{
		Resource:    "memory",
		ID:          "flushable-memory",
		Type:        memcore.TokenBasedMemory,
		Description: "Memory with aggressive flushing",
		MaxTokens:   2000,
		MaxMessages: 50,
		Persistence: memcore.PersistenceConfig{Type: memcore.RedisPersistence, TTL: "1h"},
		Flushing: &memcore.FlushingStrategyConfig{
			Type:               memcore.SimpleFIFOFlushing,
			SummarizeThreshold: 0.5,
		},
		Locking: &memcore.LockConfig{
			AppendTTL: "30s",
			ClearTTL:  "60s",
			FlushTTL:  "120s",
		},
	}
}

func (env *TestEnvironment) mustRegisterTestConfig(cfg *memory.Config) {
	if err := cfg.Validate(env.ctx); err != nil {
		panic(fmt.Sprintf("Failed to validate test config %s: %v", cfg.ID, err))
	}
	if err := env.configRegistry.Register(cfg, "test"); err != nil {
		panic(fmt.Sprintf("Failed to register test config %s: %v", cfg.ID, err))
	}
}

// CreateTestMemoryRef creates a standardized memory reference for tests
func CreateTestMemoryRef(memoryID, testName string) core.MemoryReference {
	return core.MemoryReference{
		ID:  memoryID,
		Key: fmt.Sprintf("%s-{{.test.id}}", testName),
	}
}

// CreateTestWorkflowContext creates a standardized workflow context for tests
func CreateTestWorkflowContext(testName string) map[string]any {
	return map[string]any{
		"project": map[string]any{
			"id": "test-project",
		},
		"test": map[string]any{
			"id": fmt.Sprintf("%s-%d", testName, time.Now().Unix()),
		},
	}
}

// CreateTestMessage creates a standardized test message
func CreateTestMessage(role llm.MessageRole, content string) llm.Message {
	return llm.Message{
		Role:    role,
		Content: content,
	}
}

// AppendTestMessages appends a series of test messages to a memory instance
func AppendTestMessages(ctx context.Context, instance memcore.Memory, messages []llm.Message) error {
	for _, msg := range messages {
		if err := instance.Append(ctx, msg); err != nil {
			return fmt.Errorf("failed to append message: %w", err)
		}
	}
	return nil
}
