package llm

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock memory provider
type mockMemoryProvider struct {
	mock.Mock
}

func (m *mockMemoryProvider) GetMemory(ctx context.Context, memoryID string, keyTemplate string) (Memory, error) {
	args := m.Called(ctx, memoryID, keyTemplate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(Memory), args.Error(1)
}

// Mock memory
type mockMemory struct {
	mock.Mock
	id string
}

func (m *mockMemory) Append(ctx context.Context, msg Message) error {
	args := m.Called(ctx, msg)
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
		err := StoreResponseInMemory(ctx, nil, nil, llmadapter.Message{}, llmadapter.Message{})
		assert.NoError(t, err)
	})

	t.Run("Should store messages in read-write memory", func(t *testing.T) {
		mockMemory := new(mockMemory)
		userMsg := Message{Role: MessageRoleUser, Content: "Question"}
		assistantMsg := Message{Role: MessageRoleAssistant, Content: "Answer"}

		mockMemory.On("Append", ctx, userMsg).Return(nil)
		mockMemory.On("Append", ctx, assistantMsg).Return(nil)

		memories := map[string]Memory{
			"memory1": mockMemory,
		}
		memoryRefs := []core.MemoryReference{
			{ID: "memory1", Mode: "read-write"},
		}

		err := StoreResponseInMemory(ctx, memories, memoryRefs,
			llmadapter.Message{Role: "assistant", Content: "Answer"},
			llmadapter.Message{Role: "user", Content: "Question"},
		)

		assert.NoError(t, err)
		mockMemory.AssertExpectations(t)
	})

	t.Run("Should skip read-only memories", func(t *testing.T) {
		mockMemory := new(mockMemory)
		// Should not be called for read-only

		memories := map[string]Memory{
			"memory1": mockMemory,
		}
		memoryRefs := []core.MemoryReference{
			{ID: "memory1", Mode: "read-only"},
		}

		err := StoreResponseInMemory(ctx, memories, memoryRefs,
			llmadapter.Message{Role: "assistant", Content: "Answer"},
			llmadapter.Message{Role: "user", Content: "Question"},
		)

		assert.NoError(t, err)
		mockMemory.AssertNotCalled(t, "Append")
	})

	t.Run("Should handle append errors gracefully", func(t *testing.T) {
		mockMemory1 := new(mockMemory)
		mockMemory1.On("Append", ctx, mock.Anything).Return(assert.AnError)

		mockMemory2 := new(mockMemory)
		mockMemory2.On("Append", ctx, mock.Anything).Return(nil)

		memories := map[string]Memory{
			"memory1": mockMemory1,
			"memory2": mockMemory2,
		}
		memoryRefs := []core.MemoryReference{
			{ID: "memory1", Mode: "read-write"},
			{ID: "memory2", Mode: "read-write"},
		}

		err := StoreResponseInMemory(ctx, memories, memoryRefs,
			llmadapter.Message{Role: "assistant", Content: "Answer"},
			llmadapter.Message{Role: "user", Content: "Question"},
		)

		// Should not fail entirely
		assert.NoError(t, err)
		mockMemory1.AssertExpectations(t)
		mockMemory2.AssertExpectations(t)
	})
}
