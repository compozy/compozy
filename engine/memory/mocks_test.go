package memory

import (
	"context"
	"time"

	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/llm"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/mock"
)

// MockMemoryLockManager is a mock implementation of MemoryLockManager
type MockMemoryLockManager struct {
	mock.Mock
}

func (m *MockMemoryLockManager) Acquire(ctx context.Context, key string, ttl time.Duration) (cache.Lock, error) {
	args := m.Called(ctx, key, ttl)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(cache.Lock), args.Error(1)
}

func (m *MockMemoryLockManager) TryAcquire(ctx context.Context, key string, ttl time.Duration) (cache.Lock, error) {
	args := m.Called(ctx, key, ttl)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(cache.Lock), args.Error(1)
}

func (m *MockMemoryLockManager) GetKeyPrefix() string {
	args := m.Called()
	return args.String(0)
}

// MockTokenMemoryManager is a mock implementation of TokenMemoryManager
type MockTokenMemoryManager struct {
	mock.Mock
}

func (m *MockTokenMemoryManager) CalculateTotalTokens(ctx context.Context, messages []llm.Message) (int, error) {
	args := m.Called(ctx, messages)
	return args.Int(0), args.Error(1)
}

func (m *MockTokenMemoryManager) EnforceTokenLimits(
	ctx context.Context,
	messages []llm.Message,
) ([]llm.Message, error) {
	args := m.Called(ctx, messages)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]llm.Message), args.Error(1)
}

func (m *MockTokenMemoryManager) GetTokenLimits() (maxTokens int, maxMessages int) {
	args := m.Called()
	return args.Int(0), args.Int(1)
}

// MockHybridFlushingStrategy is a mock implementation of HybridFlushingStrategy
type MockHybridFlushingStrategy struct {
	mock.Mock
}

func (m *MockHybridFlushingStrategy) ShouldFlush(ctx context.Context, messages []llm.Message, tokenCount int) bool {
	args := m.Called(ctx, messages, tokenCount)
	return args.Bool(0)
}

func (m *MockHybridFlushingStrategy) ExecuteFlush(ctx context.Context, messages []llm.Message) ([]llm.Message, error) {
	args := m.Called(ctx, messages)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]llm.Message), args.Error(1)
}

func (m *MockHybridFlushingStrategy) GetThresholdTokens() int {
	args := m.Called()
	return args.Int(0)
}

// MockLockManager is a mock implementation of cache.LockManager
type MockLockManager struct {
	mock.Mock
}

func (m *MockLockManager) CreateLock(key string, ttl time.Duration) (cache.Lock, error) {
	args := m.Called(key, ttl)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(cache.Lock), args.Error(1)
}

func (m *MockLockManager) Acquire(ctx context.Context, resource string, ttl time.Duration) (cache.Lock, error) {
	args := m.Called(ctx, resource, ttl)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(cache.Lock), args.Error(1)
}

// MockRedisClient is a mock implementation of cache.RedisInterface
type MockRedisClient struct {
	mock.Mock
}

func (m *MockRedisClient) Ping(ctx context.Context) *redis.StatusCmd {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return redis.NewStatusCmd(ctx)
	}
	return args.Get(0).(*redis.StatusCmd)
}

func (m *MockRedisClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockRedisClient) Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd {
	args := m.Called(ctx, key, value, expiration)
	if args.Get(0) == nil {
		return redis.NewStatusCmd(ctx)
	}
	return args.Get(0).(*redis.StatusCmd)
}

func (m *MockRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return redis.NewStringCmd(ctx)
	}
	return args.Get(0).(*redis.StringCmd)
}

func (m *MockRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	args := m.Called(ctx, keys)
	if args.Get(0) == nil {
		return redis.NewIntCmd(ctx)
	}
	return args.Get(0).(*redis.IntCmd)
}

func (m *MockRedisClient) SetNX(ctx context.Context, key string, value any, expiration time.Duration) *redis.BoolCmd {
	args := m.Called(ctx, key, value, expiration)
	if args.Get(0) == nil {
		return redis.NewBoolCmd(ctx)
	}
	return args.Get(0).(*redis.BoolCmd)
}

func (m *MockRedisClient) GetEx(ctx context.Context, key string, expiration time.Duration) *redis.StringCmd {
	args := m.Called(ctx, key, expiration)
	if args.Get(0) == nil {
		return redis.NewStringCmd(ctx)
	}
	return args.Get(0).(*redis.StringCmd)
}

func (m *MockRedisClient) MGet(ctx context.Context, keys ...string) *redis.SliceCmd {
	args := m.Called(ctx, keys)
	if args.Get(0) == nil {
		return redis.NewSliceCmd(ctx)
	}
	return args.Get(0).(*redis.SliceCmd)
}

func (m *MockRedisClient) Exists(ctx context.Context, keys ...string) *redis.IntCmd {
	args := m.Called(ctx, keys)
	if args.Get(0) == nil {
		return redis.NewIntCmd(ctx)
	}
	return args.Get(0).(*redis.IntCmd)
}

func (m *MockRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	args := m.Called(ctx, key, expiration)
	if args.Get(0) == nil {
		return redis.NewBoolCmd(ctx)
	}
	return args.Get(0).(*redis.BoolCmd)
}

func (m *MockRedisClient) TTL(ctx context.Context, key string) *redis.DurationCmd {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		cmd := redis.NewDurationCmd(ctx, time.Duration(0))
		return cmd
	}
	return args.Get(0).(*redis.DurationCmd)
}

func (m *MockRedisClient) Keys(ctx context.Context, pattern string) *redis.StringSliceCmd {
	args := m.Called(ctx, pattern)
	if args.Get(0) == nil {
		return redis.NewStringSliceCmd(ctx)
	}
	return args.Get(0).(*redis.StringSliceCmd)
}

func (m *MockRedisClient) Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd {
	args := m.Called(ctx, cursor, match, count)
	if args.Get(0) == nil {
		// Create a basic ScanCmd without the cmdable interface
		cmd := &redis.ScanCmd{}
		cmd.SetErr(nil)
		return cmd
	}
	return args.Get(0).(*redis.ScanCmd)
}

func (m *MockRedisClient) Publish(ctx context.Context, channel string, message any) *redis.IntCmd {
	args := m.Called(ctx, channel, message)
	if args.Get(0) == nil {
		return redis.NewIntCmd(ctx)
	}
	return args.Get(0).(*redis.IntCmd)
}

func (m *MockRedisClient) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	args := m.Called(ctx, channels)
	if args.Get(0) == nil {
		return &redis.PubSub{}
	}
	return args.Get(0).(*redis.PubSub)
}

func (m *MockRedisClient) PSubscribe(ctx context.Context, patterns ...string) *redis.PubSub {
	args := m.Called(ctx, patterns)
	if args.Get(0) == nil {
		return &redis.PubSub{}
	}
	return args.Get(0).(*redis.PubSub)
}

func (m *MockRedisClient) Eval(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd {
	mockArgs := m.Called(ctx, script, keys, args)
	if mockArgs.Get(0) == nil {
		return redis.NewCmd(ctx)
	}
	return mockArgs.Get(0).(*redis.Cmd)
}

func (m *MockRedisClient) Pipeline() redis.Pipeliner {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(redis.Pipeliner)
}

func (m *MockRedisClient) LRange(ctx context.Context, key string, start, stop int64) *redis.StringSliceCmd {
	args := m.Called(ctx, key, start, stop)
	if args.Get(0) == nil {
		return redis.NewStringSliceCmd(ctx)
	}
	return args.Get(0).(*redis.StringSliceCmd)
}

func (m *MockRedisClient) LLen(ctx context.Context, key string) *redis.IntCmd {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return redis.NewIntCmd(ctx)
	}
	return args.Get(0).(*redis.IntCmd)
}

func (m *MockRedisClient) LTrim(ctx context.Context, key string, start, stop int64) *redis.StatusCmd {
	args := m.Called(ctx, key, start, stop)
	if args.Get(0) == nil {
		return redis.NewStatusCmd(ctx)
	}
	return args.Get(0).(*redis.StatusCmd)
}

func (m *MockRedisClient) RPush(ctx context.Context, key string, values ...any) *redis.IntCmd {
	args := m.Called(ctx, key, values)
	if args.Get(0) == nil {
		return redis.NewIntCmd(ctx)
	}
	return args.Get(0).(*redis.IntCmd)
}

func (m *MockRedisClient) HSet(ctx context.Context, key string, values ...any) *redis.IntCmd {
	args := m.Called(ctx, key, values)
	if args.Get(0) == nil {
		return redis.NewIntCmd(ctx)
	}
	return args.Get(0).(*redis.IntCmd)
}

func (m *MockRedisClient) HGet(ctx context.Context, key, field string) *redis.StringCmd {
	args := m.Called(ctx, key, field)
	if args.Get(0) == nil {
		return redis.NewStringCmd(ctx)
	}
	return args.Get(0).(*redis.StringCmd)
}

func (m *MockRedisClient) HIncrBy(ctx context.Context, key, field string, incr int64) *redis.IntCmd {
	args := m.Called(ctx, key, field, incr)
	if args.Get(0) == nil {
		return redis.NewIntCmd(ctx)
	}
	return args.Get(0).(*redis.IntCmd)
}

func (m *MockRedisClient) HDel(ctx context.Context, key string, fields ...string) *redis.IntCmd {
	args := m.Called(ctx, key, fields)
	if args.Get(0) == nil {
		return redis.NewIntCmd(ctx)
	}
	return args.Get(0).(*redis.IntCmd)
}

func (m *MockRedisClient) TxPipeline() redis.Pipeliner {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(redis.Pipeliner)
}

// MockLock is a mock implementation of cache.Lock
type MockLock struct {
	mock.Mock
}

func (m *MockLock) Release(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockLock) Refresh(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockLock) Resource() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockLock) IsHeld() bool {
	args := m.Called()
	return args.Bool(0)
}
