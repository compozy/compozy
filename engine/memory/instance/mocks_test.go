package instance

import (
	"context"
	"time"

	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory/core"
	"github.com/stretchr/testify/mock"
)

type mockStore struct {
	mock.Mock
}

func (m *mockStore) AppendMessage(ctx context.Context, key string, msg llm.Message) error {
	args := m.Called(ctx, key, msg)
	return args.Error(0)
}

func (m *mockStore) AppendMessages(ctx context.Context, key string, msgs []llm.Message) error {
	args := m.Called(ctx, key, msgs)
	return args.Error(0)
}

func (m *mockStore) ReplaceMessages(ctx context.Context, key string, messages []llm.Message) error {
	args := m.Called(ctx, key, messages)
	return args.Error(0)
}

func (m *mockStore) DeleteMessages(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *mockStore) ReadMessages(ctx context.Context, key string) ([]llm.Message, error) {
	args := m.Called(ctx, key)
	return args.Get(0).([]llm.Message), args.Error(1)
}

func (m *mockStore) GetMessageCount(ctx context.Context, key string) (int, error) {
	args := m.Called(ctx, key)
	return args.Int(0), args.Error(1)
}

func (m *mockStore) GetTokenCount(ctx context.Context, key string) (int, error) {
	args := m.Called(ctx, key)
	return args.Int(0), args.Error(1)
}

func (m *mockStore) IncrementTokenCount(ctx context.Context, key string, delta int) error {
	args := m.Called(ctx, key, delta)
	return args.Error(0)
}

func (m *mockStore) SetTokenCount(ctx context.Context, key string, count int) error {
	args := m.Called(ctx, key, count)
	return args.Error(0)
}

func (m *mockStore) MarkFlushPending(ctx context.Context, key string, pending bool) error {
	args := m.Called(ctx, key, pending)
	return args.Error(0)
}

func (m *mockStore) IsFlushPending(ctx context.Context, key string) (bool, error) {
	args := m.Called(ctx, key)
	return args.Bool(0), args.Error(1)
}

func (m *mockStore) SetLastFlushed(ctx context.Context, key string, timestamp time.Time) error {
	args := m.Called(ctx, key, timestamp)
	return args.Error(0)
}

func (m *mockStore) GetLastFlushed(ctx context.Context, key string) (time.Time, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(time.Time), args.Error(1)
}

func (m *mockStore) GetExpiration(ctx context.Context, key string) (time.Time, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(time.Time), args.Error(1)
}

func (m *mockStore) SetExpiration(ctx context.Context, key string, ttl time.Duration) error {
	args := m.Called(ctx, key, ttl)
	return args.Error(0)
}

func (m *mockStore) GetKeyTTL(ctx context.Context, key string) (time.Duration, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(time.Duration), args.Error(1)
}

func (m *mockStore) GetMetadata(ctx context.Context, key string) (map[string]any, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(map[string]any), args.Error(1)
}

func (m *mockStore) SetMetadata(ctx context.Context, key string, metadata map[string]any) error {
	args := m.Called(ctx, key, metadata)
	return args.Error(0)
}

func (m *mockStore) AppendMessageWithTokenCount(
	ctx context.Context,
	key string,
	msg llm.Message,
	tokenCount int,
) error {
	args := m.Called(ctx, key, msg, tokenCount)
	return args.Error(0)
}

func (m *mockStore) TrimMessagesWithMetadata(ctx context.Context, key string, keepCount int, newTokenCount int) error {
	args := m.Called(ctx, key, keepCount, newTokenCount)
	return args.Error(0)
}

func (m *mockStore) ReplaceMessagesWithMetadata(
	ctx context.Context,
	key string,
	messages []llm.Message,
	totalTokens int,
) error {
	args := m.Called(ctx, key, messages, totalTokens)
	return args.Error(0)
}

func (m *mockStore) CountMessages(ctx context.Context, key string) (int, error) {
	args := m.Called(ctx, key)
	return args.Int(0), args.Error(1)
}

type mockLockManager struct {
	mock.Mock
}

func (m *mockLockManager) AcquireAppendLock(ctx context.Context, key string) (UnlockFunc, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	// Use type assertion that handles the interface conversion properly
	if fn, ok := args.Get(0).(func() error); ok {
		return UnlockFunc(fn), args.Error(1)
	}
	return args.Get(0).(UnlockFunc), args.Error(1)
}

func (m *mockLockManager) AcquireClearLock(ctx context.Context, key string) (UnlockFunc, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	// Use type assertion that handles the interface conversion properly
	if fn, ok := args.Get(0).(func() error); ok {
		return UnlockFunc(fn), args.Error(1)
	}
	return args.Get(0).(UnlockFunc), args.Error(1)
}

func (m *mockLockManager) AcquireFlushLock(ctx context.Context, key string) (UnlockFunc, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	// Use type assertion that handles the interface conversion properly
	if fn, ok := args.Get(0).(func() error); ok {
		return UnlockFunc(fn), args.Error(1)
	}
	return args.Get(0).(UnlockFunc), args.Error(1)
}

type mockTokenCounter struct {
	mock.Mock
}

func (m *mockTokenCounter) CountTokens(ctx context.Context, text string) (int, error) {
	args := m.Called(ctx, text)
	return args.Int(0), args.Error(1)
}

func (m *mockTokenCounter) EncodeTokens(ctx context.Context, text string) ([]int, error) {
	args := m.Called(ctx, text)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]int), args.Error(1)
}

func (m *mockTokenCounter) DecodeTokens(ctx context.Context, tokens []int) (string, error) {
	args := m.Called(ctx, tokens)
	return args.String(0), args.Error(1)
}

func (m *mockTokenCounter) GetEncoding() string {
	return "test-encoding"
}

type mockFlushStrategy struct {
	mock.Mock
}

func (m *mockFlushStrategy) ShouldFlush(tokenCount, messageCount int, config *core.Resource) bool {
	args := m.Called(tokenCount, messageCount, config)
	return args.Bool(0)
}

func (m *mockFlushStrategy) PerformFlush(
	ctx context.Context,
	messages []llm.Message,
	config *core.Resource,
) (*core.FlushMemoryActivityOutput, error) {
	args := m.Called(ctx, messages, config)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.FlushMemoryActivityOutput), args.Error(1)
}

func (m *mockFlushStrategy) GetType() core.FlushingStrategyType {
	args := m.Called()
	return args.Get(0).(core.FlushingStrategyType)
}

// NOTE: Temporal client mocking is complex due to the extensive interface.
// For unit tests, we focus on builder validation logic.
// Full temporal client integration is tested in integration tests.
