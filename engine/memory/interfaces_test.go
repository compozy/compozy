package memory

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/llm" // Assuming llm.Message is here
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockMemory is a mock implementation of the Memory interface.
type MockMemory struct {
	mock.Mock
}

func (m *MockMemory) Append(ctx context.Context, msg llm.Message) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *MockMemory) Read(ctx context.Context) ([]llm.Message, error) {
	args := m.Called(ctx)
	return args.Get(0).([]llm.Message), args.Error(1)
}

func (m *MockMemory) Len(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (m *MockMemory) GetTokenCount(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (m *MockMemory) GetMemoryHealth(ctx context.Context) (*Health, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Health), args.Error(1)
}

func (m *MockMemory) Clear(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
func (m *MockMemory) GetID() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockMemory) AppendWithPrivacy(ctx context.Context, msg llm.Message, metadata PrivacyMetadata) error {
	args := m.Called(ctx, msg, metadata)
	return args.Error(0)
}

// MockMemoryStore is a mock implementation of the MemoryStore interface.
type MockMemoryStore struct {
	mock.Mock
}

func (m *MockMemoryStore) AppendMessage(ctx context.Context, key string, msg llm.Message) error {
	args := m.Called(ctx, key, msg)
	return args.Error(0)
}

func (m *MockMemoryStore) AppendMessages(ctx context.Context, key string, msgs []llm.Message) error {
	args := m.Called(ctx, key, msgs)
	return args.Error(0)
}

func (m *MockMemoryStore) ReadMessages(ctx context.Context, key string) ([]llm.Message, error) {
	args := m.Called(ctx, key)
	return args.Get(0).([]llm.Message), args.Error(1)
}

func (m *MockMemoryStore) CountMessages(ctx context.Context, key string) (int, error) {
	args := m.Called(ctx, key)
	return args.Int(0), args.Error(1)
}

func (m *MockMemoryStore) TrimMessagesWithMetadata(
	ctx context.Context,
	key string,
	keepCount int,
	newTokenCount int,
) error {
	args := m.Called(ctx, key, keepCount, newTokenCount)
	return args.Error(0)
}

func (m *MockMemoryStore) ReplaceMessages(ctx context.Context, key string, messages []llm.Message) error {
	args := m.Called(ctx, key, messages)
	return args.Error(0)
}

func (m *MockMemoryStore) SetExpiration(ctx context.Context, key string, ttl time.Duration) error {
	args := m.Called(ctx, key, ttl)
	return args.Error(0)
}

func (m *MockMemoryStore) DeleteMessages(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockMemoryStore) GetKeyTTL(ctx context.Context, key string) (time.Duration, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(time.Duration), args.Error(1)
}

func (m *MockMemoryStore) GetTokenCount(ctx context.Context, key string) (int, error) {
	args := m.Called(ctx, key)
	return args.Int(0), args.Error(1)
}

func (m *MockMemoryStore) IncrementTokenCount(ctx context.Context, key string, delta int) error {
	args := m.Called(ctx, key, delta)
	return args.Error(0)
}

func (m *MockMemoryStore) SetTokenCount(ctx context.Context, key string, count int) error {
	args := m.Called(ctx, key, count)
	return args.Error(0)
}

// MockTokenCounter is a mock implementation of the TokenCounter interface.
type MockTokenCounter struct {
	mock.Mock
}

func (m *MockTokenCounter) CountTokens(ctx context.Context, text string) (int, error) {
	args := m.Called(ctx, text)
	return args.Int(0), args.Error(1)
}

func (m *MockTokenCounter) GetEncoding() string {
	args := m.Called()
	return args.String(0)
}

func TestMockMemory(t *testing.T) {
	mockMem := new(MockMemory)
	ctx := context.Background()
	msg1 := llm.Message{Role: "user", Content: "Hello"}
	expectedMsgs := []llm.Message{msg1}

	// Set up expectations
	mockMem.On("GetID").Return("test-id")
	mockMem.On("Append", ctx, msg1).Return(nil)
	mockMem.On("Read", ctx).Return(expectedMsgs, nil)
	mockMem.On("Len", ctx).Return(1, nil)
	mockMem.On("GetTokenCount", ctx).Return(10, nil)
	mockMem.On("GetMemoryHealth", ctx).Return(&Health{TokenCount: 10, MessageCount: 1}, nil)
	mockMem.On("Clear", ctx).Return(nil)

	// Call methods
	assert.Equal(t, "test-id", mockMem.GetID())
	assert.NoError(t, mockMem.Append(ctx, msg1))
	actualMsgs, err := mockMem.Read(ctx)
	assert.NoError(t, err)
	assert.Equal(t, expectedMsgs, actualMsgs)
	length, err := mockMem.Len(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, length)
	tokens, err := mockMem.GetTokenCount(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 10, tokens)
	health, err := mockMem.GetMemoryHealth(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 10, health.TokenCount)
	assert.NoError(t, mockMem.Clear(ctx))

	// Assert that all expectations were met
	mockMem.AssertExpectations(t)
}

func TestMockMemoryStore(t *testing.T) {
	mockStore := new(MockMemoryStore)
	ctx := context.Background()
	key := "testKey"
	msg1 := llm.Message{Role: "user", Content: "Test"}
	msgs := []llm.Message{msg1}

	mockStore.On("AppendMessage", ctx, key, msg1).Return(nil)
	mockStore.On("AppendMessages", ctx, key, msgs).Return(nil)
	mockStore.On("ReadMessages", ctx, key).Return(msgs, nil)
	mockStore.On("CountMessages", ctx, key).Return(1, nil)
	mockStore.On("TrimMessagesWithMetadata", ctx, key, 5, 0).Return(nil)
	mockStore.On("ReplaceMessages", ctx, key, msgs).Return(nil)
	mockStore.On("SetExpiration", ctx, key, time.Hour).Return(nil)
	mockStore.On("GetKeyTTL", ctx, key).Return(time.Hour, nil)
	mockStore.On("DeleteMessages", ctx, key).Return(nil)

	assert.NoError(t, mockStore.AppendMessage(ctx, key, msg1))
	assert.NoError(t, mockStore.AppendMessages(ctx, key, msgs))
	retMsgs, err := mockStore.ReadMessages(ctx, key)
	assert.NoError(t, err)
	assert.Equal(t, msgs, retMsgs)
	count, err := mockStore.CountMessages(ctx, key)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.NoError(t, mockStore.TrimMessagesWithMetadata(ctx, key, 5, 0))
	assert.NoError(t, mockStore.ReplaceMessages(ctx, key, msgs))
	assert.NoError(t, mockStore.SetExpiration(ctx, key, time.Hour))
	ttl, err := mockStore.GetKeyTTL(ctx, key)
	assert.NoError(t, err)
	assert.Equal(t, time.Hour, ttl)
	assert.NoError(t, mockStore.DeleteMessages(ctx, key))

	mockStore.AssertExpectations(t)
}

func TestMockTokenCounter(t *testing.T) {
	mockCounter := new(MockTokenCounter)
	ctx := context.Background()
	text := "hello world"

	mockCounter.On("CountTokens", ctx, text).Return(2, nil)
	mockCounter.On("GetEncoding").Return("test_encoding")

	count, err := mockCounter.CountTokens(ctx, text)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)
	encoding := mockCounter.GetEncoding()
	assert.Equal(t, "test_encoding", encoding)

	mockCounter.AssertExpectations(t)
}
