package llm

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock memory
type mockMemory struct {
	mock.Mock
	id string
}

func (m *mockMemory) Append(ctx context.Context, msg Message) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *mockMemory) AppendMany(ctx context.Context, msgs []Message) error {
	args := m.Called(ctx, msgs)
	return args.Error(0)
}

func (m *mockMemory) Read(ctx context.Context) ([]Message, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]Message), args.Error(1)
}

func (m *mockMemory) GetID() string {
	return m.id
}

func TestPrepareMemoryContext(t *testing.T) {
	ctx := context.Background()

	t.Run("Should handle empty memories", func(t *testing.T) {
		messages := []llmadapter.Message{
			{Role: "user", Content: "Hello"},
		}

		result := PrepareMemoryContext(ctx, nil, messages)

		assert.Equal(t, messages, result)
	})

	t.Run("Should prepend memory messages", func(t *testing.T) {
		mockMemory := new(mockMemory)
		memMessages := []Message{
			{Role: MessageRoleUser, Content: "Previous question"},
			{Role: MessageRoleAssistant, Content: "Previous answer"},
		}
		mockMemory.On("Read", ctx).Return(memMessages, nil)

		memories := map[string]Memory{
			"memory1": mockMemory,
		}

		currentMessages := []llmadapter.Message{
			{Role: "user", Content: "Current question"},
		}

		result := PrepareMemoryContext(ctx, memories, currentMessages)

		assert.Len(t, result, 3)
		assert.Equal(t, "user", result[0].Role)
		assert.Equal(t, "Previous question", result[0].Content)
		assert.Equal(t, "assistant", result[1].Role)
		assert.Equal(t, "Previous answer", result[1].Content)
		assert.Equal(t, "user", result[2].Role)
		assert.Equal(t, "Current question", result[2].Content)

		mockMemory.AssertExpectations(t)
	})

	t.Run("Should handle read errors gracefully", func(t *testing.T) {
		mockMemory1 := new(mockMemory)
		mockMemory1.On("Read", ctx).Return(nil, assert.AnError)

		mockMemory2 := new(mockMemory)
		memMessages := []Message{
			{Role: MessageRoleAssistant, Content: "From memory 2"},
		}
		mockMemory2.On("Read", ctx).Return(memMessages, nil)

		memories := map[string]Memory{
			"memory1": mockMemory1,
			"memory2": mockMemory2,
		}

		currentMessages := []llmadapter.Message{
			{Role: "user", Content: "Current"},
		}

		result := PrepareMemoryContext(ctx, memories, currentMessages)

		// Should only have messages from memory2 and current
		assert.Len(t, result, 2)
		assert.Equal(t, "assistant", result[0].Role)
		assert.Equal(t, "From memory 2", result[0].Content)
		assert.Equal(t, "user", result[1].Role)
		assert.Equal(t, "Current", result[1].Content)

		mockMemory1.AssertExpectations(t)
		mockMemory2.AssertExpectations(t)
	})
}

func TestStoreResponseInMemory(t *testing.T) {
	ctx := context.Background()

	t.Run("Should handle empty memories", func(t *testing.T) {
		msg := llmadapter.Message{}
		err := StoreResponseInMemory(ctx, nil, nil, &msg, &msg)
		assert.NoError(t, err)
	})

	t.Run("Should error on nil message pointers", func(t *testing.T) {
		err := StoreResponseInMemory(ctx, nil, nil, nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nil message pointer")
	})

	t.Run("Should store messages in read-write memory", func(t *testing.T) {
		mockMemory := new(mockMemory)
		mockMemory.id = "memory1"

		userMsg := Message{Role: MessageRoleUser, Content: "Question"}
		assistantMsg := Message{Role: MessageRoleAssistant, Content: "Answer"}
		expectedMsgs := []Message{userMsg, assistantMsg}
		mockMemory.On("AppendMany", ctx, expectedMsgs).Return(nil)

		memories := map[string]Memory{
			"memory1": mockMemory,
		}

		memoryRefs := []core.MemoryReference{
			{ID: "memory1", Mode: "read-write"},
		}

		userMessage := llmadapter.Message{Role: "user", Content: "Question"}
		assistantResponse := llmadapter.Message{Role: "assistant", Content: "Answer"}

		err := StoreResponseInMemory(ctx, memories, memoryRefs, &assistantResponse, &userMessage)

		assert.NoError(t, err)
		mockMemory.AssertExpectations(t)
	})

	t.Run("Should skip read-only memories", func(t *testing.T) {
		mockMemory := new(mockMemory)
		mockMemory.id = "memory1"

		// No expectations set since read-only memories should be skipped

		memories := map[string]Memory{
			"memory1": mockMemory,
		}

		memoryRefs := []core.MemoryReference{
			{ID: "memory1", Mode: core.MemoryModeReadOnly},
		}

		userMessage := llmadapter.Message{Role: "user", Content: "Question"}
		assistantResponse := llmadapter.Message{Role: "assistant", Content: "Answer"}

		err := StoreResponseInMemory(ctx, memories, memoryRefs, &assistantResponse, &userMessage)

		assert.NoError(t, err)
		// Should not have called AppendMany since it's read-only
		mockMemory.AssertNotCalled(t, "AppendMany")
	})

	t.Run("Should handle storage errors", func(t *testing.T) {
		mockMemory1 := new(mockMemory)
		mockMemory1.id = "memory1"
		mockMemory2 := new(mockMemory)
		mockMemory2.id = "memory2"

		// First memory fails
		mockMemory1.On("AppendMany", ctx, mock.Anything).Return(assert.AnError)
		// Second memory succeeds
		mockMemory2.On("AppendMany", ctx, mock.Anything).Return(nil)

		memories := map[string]Memory{
			"memory1": mockMemory1,
			"memory2": mockMemory2,
		}

		memoryRefs := []core.MemoryReference{
			{ID: "memory1", Mode: "read-write"},
			{ID: "memory2", Mode: "read-write"},
		}

		userMessage := llmadapter.Message{Role: "user", Content: "Question"}
		assistantResponse := llmadapter.Message{Role: "assistant", Content: "Answer"}

		err := StoreResponseInMemory(ctx, memories, memoryRefs, &assistantResponse, &userMessage)

		// Should now return an error when any memory fails
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "memory storage errors")
		assert.Contains(t, err.Error(), "memory1")
		// Verify control flow: exactly one atomic append attempted per memory
		mockMemory1.AssertNumberOfCalls(t, "AppendMany", 1)
		mockMemory2.AssertNumberOfCalls(t, "AppendMany", 1)
		mockMemory1.AssertExpectations(t)
		mockMemory2.AssertExpectations(t)
	})
}

func TestStoreResponseInMemory_AtomicStorage(t *testing.T) {
	ctx := context.Background()

	t.Run("Should use AppendMany for atomic storage", func(t *testing.T) {
		mockMemory := new(mockMemory)
		mockMemory.id = "memory1"

		// Mock AppendMany to succeed
		userMsg := Message{Role: MessageRoleUser, Content: "Test question"}
		assistantMsg := Message{Role: MessageRoleAssistant, Content: "Test response"}
		expectedMsgs := []Message{userMsg, assistantMsg}
		mockMemory.On("AppendMany", ctx, expectedMsgs).Return(nil)

		memories := map[string]Memory{
			"memory1": mockMemory,
		}

		memoryRefs := []core.MemoryReference{
			{ID: "memory1", Mode: "read-write"},
		}

		userMessage := llmadapter.Message{Role: "user", Content: "Test question"}
		assistantResponse := llmadapter.Message{Role: "assistant", Content: "Test response"}

		err := StoreResponseInMemory(ctx, memories, memoryRefs, &assistantResponse, &userMessage)

		assert.NoError(t, err)
		mockMemory.AssertExpectations(t)
		// Ensure AppendMany was called instead of individual Append calls
		mockMemory.AssertNotCalled(t, "Append")
	})

	t.Run("Should handle AppendMany failure gracefully", func(t *testing.T) {
		mockMemory := new(mockMemory)
		mockMemory.id = "memory1"

		// Mock AppendMany to fail
		expectedError := assert.AnError
		mockMemory.On("AppendMany", ctx, mock.Anything).Return(expectedError)

		memories := map[string]Memory{
			"memory1": mockMemory,
		}

		memoryRefs := []core.MemoryReference{
			{ID: "memory1", Mode: "read-write"},
		}

		userMessage := llmadapter.Message{Role: "user", Content: "Test question"}
		assistantResponse := llmadapter.Message{Role: "assistant", Content: "Test response"}

		err := StoreResponseInMemory(ctx, memories, memoryRefs, &assistantResponse, &userMessage)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to append messages to memory memory1")
		mockMemory.AssertExpectations(t)
	})

	t.Run("Should maintain atomicity - all messages stored together", func(t *testing.T) {
		mockMemory1 := new(mockMemory)
		mockMemory1.id = "memory1"
		mockMemory2 := new(mockMemory)
		mockMemory2.id = "memory2"

		// First memory succeeds
		mockMemory1.On("AppendMany", ctx, mock.Anything).Return(nil)
		// Second memory fails
		mockMemory2.On("AppendMany", ctx, mock.Anything).Return(assert.AnError)

		memories := map[string]Memory{
			"memory1": mockMemory1,
			"memory2": mockMemory2,
		}

		memoryRefs := []core.MemoryReference{
			{ID: "memory1", Mode: "read-write"},
			{ID: "memory2", Mode: "read-write"},
		}

		userMessage := llmadapter.Message{Role: "user", Content: "Test question"}
		assistantResponse := llmadapter.Message{Role: "assistant", Content: "Test response"}

		err := StoreResponseInMemory(ctx, memories, memoryRefs, &assistantResponse, &userMessage)

		assert.Error(t, err)
		mockMemory1.AssertExpectations(t)
		mockMemory2.AssertExpectations(t)
		// Verify that both memories attempted atomic storage
		mockMemory1.AssertCalled(t, "AppendMany", ctx, mock.Anything)
		mockMemory2.AssertCalled(t, "AppendMany", ctx, mock.Anything)
	})
}
