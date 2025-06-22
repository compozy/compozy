package memory

import (
	"context"
	"time"

	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/llm"
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
