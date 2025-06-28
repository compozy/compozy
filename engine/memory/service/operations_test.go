package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
)

// Test doubles - minimal implementations for testing
type testMemoryManager struct {
	getInstance func(ctx context.Context, memRef core.MemoryReference, workflowContext map[string]any) (memcore.Memory, error)
}

func (m *testMemoryManager) GetInstance(
	ctx context.Context,
	memRef core.MemoryReference,
	workflowContext map[string]any,
) (memcore.Memory, error) {
	if m.getInstance != nil {
		return m.getInstance(ctx, memRef, workflowContext)
	}
	return nil, errors.New("not implemented")
}

type testMemory struct {
	mu       sync.RWMutex
	messages []llm.Message
	appendFn func(ctx context.Context, msg llm.Message) error
	clearFn  func(ctx context.Context) error
}

func (m *testMemory) Append(ctx context.Context, msg llm.Message) error {
	if m.appendFn != nil {
		return m.appendFn(ctx, msg)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msg)
	return nil
}

func (m *testMemory) Read(_ context.Context) ([]llm.Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// Return a copy to prevent data races
	msgCopy := make([]llm.Message, len(m.messages))
	copy(msgCopy, m.messages)
	return msgCopy, nil
}

func (m *testMemory) Len(_ context.Context) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.messages), nil
}

func (m *testMemory) GetTokenCount(_ context.Context) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// Simple approximation for testing
	return len(m.messages) * 50, nil
}

func (m *testMemory) GetMemoryHealth(_ context.Context) (*memcore.Health, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return &memcore.Health{
		TokenCount:    len(m.messages) * 50,
		MessageCount:  len(m.messages),
		FlushStrategy: "fifo",
	}, nil
}

func (m *testMemory) Clear(ctx context.Context) error {
	if m.clearFn != nil {
		return m.clearFn(ctx)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = nil
	return nil
}

func (m *testMemory) GetID() string {
	return "test-memory"
}

func (m *testMemory) AppendWithPrivacy(ctx context.Context, msg llm.Message, _ memcore.PrivacyMetadata) error {
	return m.Append(ctx, msg)
}

// Test validation functions directly
func TestValidationFunctions(t *testing.T) {
	t.Run("ValidateMemoryRef", func(t *testing.T) {
		tests := []struct {
			name    string
			ref     string
			wantErr bool
		}{
			{"valid ref", "valid_memory_123", false},
			{"empty ref", "", true},
			{"invalid chars", "invalid-memory!", true},
			{"too long", string(make([]byte, 101)), true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := ValidateMemoryRef(tt.ref)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("ValidateKey", func(t *testing.T) {
		tests := []struct {
			name    string
			key     string
			wantErr bool
		}{
			{"valid key", "valid_key_123", false},
			{"key with spaces", "key with spaces", false},
			{"empty key", "", true},
			{"control chars", "invalid\x00key", true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := ValidateKey(tt.key)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

// Test conversion functions directly
func TestPayloadToMessages(t *testing.T) {
	tests := []struct {
		name    string
		payload any
		want    []llm.Message
		wantErr bool
	}{
		{
			name:    "string payload",
			payload: "Hello world",
			want:    []llm.Message{{Role: llm.MessageRoleUser, Content: "Hello world"}},
		},
		{
			name: "single message map",
			payload: map[string]any{
				"role":    "assistant",
				"content": "Hello there!",
			},
			want: []llm.Message{{Role: llm.MessageRoleAssistant, Content: "Hello there!"}},
		},
		{
			name: "array of messages",
			payload: []any{
				map[string]any{"role": "user", "content": "Hi"},
				map[string]any{"role": "assistant", "content": "Hello"},
			},
			want: []llm.Message{
				{Role: llm.MessageRoleUser, Content: "Hi"},
				{Role: llm.MessageRoleAssistant, Content: "Hello"},
			},
		},
		{
			name:    "nil payload",
			payload: nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PayloadToMessages(tt.payload)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Test Read operation
func TestMemoryService_Read(t *testing.T) {
	ctx := context.Background()

	t.Run("successful read", func(t *testing.T) {
		// Setup
		memory := &testMemory{
			messages: []llm.Message{
				{Role: llm.MessageRoleUser, Content: "Hello"},
				{Role: llm.MessageRoleAssistant, Content: "Hi there!"},
			},
		}

		manager := &testMemoryManager{
			getInstance: func(_ context.Context, memRef core.MemoryReference, _ map[string]any) (memcore.Memory, error) {
				assert.Equal(t, "test_memory", memRef.ID)
				assert.Equal(t, "test_key", memRef.Key)
				return memory, nil
			},
		}

		service := NewMemoryOperationsService(manager, nil, nil)

		// Execute
		resp, err := service.Read(ctx, &ReadRequest{
			BaseRequest: BaseRequest{
				MemoryRef: "test_memory",
				Key:       "test_key",
			},
		})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 2, resp.Count)
		assert.Equal(t, "test_key", resp.Key)
		assert.Len(t, resp.Messages, 2)
		assert.Equal(t, "Hello", resp.Messages[0]["content"])
		assert.Equal(t, "user", resp.Messages[0]["role"])
	})

	t.Run("validation error", func(t *testing.T) {
		service := NewMemoryOperationsService(nil, nil, nil)

		resp, err := service.Read(ctx, &ReadRequest{
			BaseRequest: BaseRequest{
				MemoryRef: "", // Invalid
				Key:       "test_key",
			},
		})

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "memory reference cannot be empty")
	})
}

// Test Write operation with atomic transactions
func TestMemoryService_Write(t *testing.T) {
	ctx := context.Background()

	t.Run("successful atomic write", func(t *testing.T) {
		memory := &testMemory{}

		manager := &testMemoryManager{
			getInstance: func(_ context.Context, _ core.MemoryReference, _ map[string]any) (memcore.Memory, error) {
				return memory, nil
			},
		}

		service := NewMemoryOperationsService(manager, nil, nil)

		// Execute
		resp, err := service.Write(ctx, &WriteRequest{
			BaseRequest: BaseRequest{
				MemoryRef: "test_memory",
				Key:       "test_key",
			},
			Payload: []map[string]any{
				{"role": "user", "content": "Hello world"},
			},
		})

		// Assert
		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Equal(t, 1, resp.Count)
		assert.Equal(t, "test_key", resp.Key)

		// Verify memory was cleared and new message added
		msgs, _ := memory.Read(context.Background())
		assert.Len(t, msgs, 1)
		assert.Equal(t, "Hello world", msgs[0].Content)
	})

	t.Run("rollback on failure", func(t *testing.T) {
		originalMessages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Original message"},
		}

		memory := &testMemory{
			messages: originalMessages,
			appendFn: func(_ context.Context, _ llm.Message) error {
				// Fail on the write attempt
				return errors.New("append failed")
			},
		}

		manager := &testMemoryManager{
			getInstance: func(_ context.Context, _ core.MemoryReference, _ map[string]any) (memcore.Memory, error) {
				return memory, nil
			},
		}

		service := NewMemoryOperationsService(manager, nil, nil)

		// Execute
		resp, err := service.Write(ctx, &WriteRequest{
			BaseRequest: BaseRequest{
				MemoryRef: "test_memory",
				Key:       "test_key",
			},
			Payload: map[string]any{
				"role":    "user",
				"content": "New message",
			},
		})

		// Assert
		assert.Error(t, err)
		assert.Nil(t, resp)
		// The error message may vary based on the specific failure
		assert.Contains(t, err.Error(), "write failed")
	})
}

// Test template resolution
func TestMemoryService_WriteWithTemplates(t *testing.T) {
	ctx := context.Background()

	t.Run("template resolution in payload", func(t *testing.T) {
		memory := &testMemory{}

		manager := &testMemoryManager{
			getInstance: func(_ context.Context, _ core.MemoryReference, _ map[string]any) (memcore.Memory, error) {
				return memory, nil
			},
		}

		// Create real template engine
		templateEngine := tplengine.NewEngine(tplengine.FormatText)
		service := NewMemoryOperationsService(manager, templateEngine, nil)

		// Create workflow state
		input := core.Input{"name": "John"}
		workflowState := &workflow.State{
			WorkflowID:     "wf_123",
			WorkflowExecID: "exec_456",
			Input:          &input,
			Tasks:          make(map[string]*task.State),
		}

		// Execute
		resp, err := service.Write(ctx, &WriteRequest{
			BaseRequest: BaseRequest{
				MemoryRef: "test_memory",
				Key:       "test_key",
			},
			Payload: map[string]any{
				"role":    "user",
				"content": "Hello {{.workflow.input.name}}!",
			},
			WorkflowState: workflowState,
		})

		// Assert
		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Equal(t, 1, resp.Count)

		// Verify template was resolved
		msgs, _ := memory.Read(ctx)
		assert.Len(t, msgs, 1)
		assert.Equal(t, "Hello John!", msgs[0].Content)
	})
}

// Test Append operation
func TestMemoryService_Append(t *testing.T) {
	ctx := context.Background()

	t.Run("successful append", func(t *testing.T) {
		memory := &testMemory{
			messages: []llm.Message{
				{Role: llm.MessageRoleUser, Content: "Existing message"},
			},
		}

		manager := &testMemoryManager{
			getInstance: func(_ context.Context, _ core.MemoryReference, _ map[string]any) (memcore.Memory, error) {
				return memory, nil
			},
		}

		service := NewMemoryOperationsService(manager, nil, nil)

		// Execute
		resp, err := service.Append(ctx, &AppendRequest{
			BaseRequest: BaseRequest{
				MemoryRef: "test_memory",
				Key:       "test_key",
			},
			Payload: map[string]any{
				"role":    "user",
				"content": "New message",
			},
		})

		// Assert
		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Equal(t, 1, resp.Appended)
		assert.Equal(t, 2, resp.TotalCount)
		assert.Equal(t, "test_key", resp.Key)

		// Verify message was appended
		msgs, _ := memory.Read(ctx)
		assert.Len(t, msgs, 2)
		assert.Equal(t, "New message", msgs[1].Content)
	})
}

// Test Delete operation
func TestMemoryService_Delete(t *testing.T) {
	ctx := context.Background()

	t.Run("successful delete", func(t *testing.T) {
		memory := &testMemory{
			messages: []llm.Message{
				{Role: llm.MessageRoleUser, Content: "Message to delete"},
			},
		}

		manager := &testMemoryManager{
			getInstance: func(_ context.Context, _ core.MemoryReference, _ map[string]any) (memcore.Memory, error) {
				return memory, nil
			},
		}

		service := NewMemoryOperationsService(manager, nil, nil)

		// Execute
		resp, err := service.Delete(ctx, &DeleteRequest{
			BaseRequest: BaseRequest{
				MemoryRef: "test_memory",
				Key:       "test_key",
			},
		})

		// Assert
		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Equal(t, "test_key", resp.Key)

		// Verify memory was cleared
		msgCount, _ := memory.Len(ctx)
		assert.Equal(t, 0, msgCount)
	})
}

// Test Clear operation
func TestMemoryService_Clear(t *testing.T) {
	ctx := context.Background()

	t.Run("successful clear with confirmation", func(t *testing.T) {
		memory := &testMemory{
			messages: []llm.Message{
				{Role: llm.MessageRoleUser, Content: "Message 1"},
				{Role: llm.MessageRoleUser, Content: "Message 2"},
				{Role: llm.MessageRoleUser, Content: "Message 3"},
			},
		}

		manager := &testMemoryManager{
			getInstance: func(_ context.Context, _ core.MemoryReference, _ map[string]any) (memcore.Memory, error) {
				return memory, nil
			},
		}

		service := NewMemoryOperationsService(manager, nil, nil)

		// Execute
		resp, err := service.Clear(ctx, &ClearRequest{
			BaseRequest: BaseRequest{
				MemoryRef: "test_memory",
				Key:       "test_key",
			},
			Config: &ClearConfig{
				Confirm: true,
				Backup:  true,
			},
		})

		// Assert
		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Equal(t, 3, resp.MessagesCleared)
		assert.False(t, resp.BackupCreated) // Backup not implemented yet
		assert.Equal(t, "test_key", resp.Key)
	})

	t.Run("clear requires confirmation", func(t *testing.T) {
		service := NewMemoryOperationsService(nil, nil, nil)

		resp, err := service.Clear(ctx, &ClearRequest{
			BaseRequest: BaseRequest{
				MemoryRef: "test_memory",
				Key:       "test_key",
			},
			Config: &ClearConfig{
				Confirm: false,
			},
		})

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "confirm flag must be true")
	})
}

// Test MemoryTransaction directly
func TestMemoryTransaction(t *testing.T) {
	ctx := context.Background()

	t.Run("successful transaction", func(t *testing.T) {
		memory := &testMemory{
			messages: []llm.Message{
				{Role: llm.MessageRoleUser, Content: "Original"},
			},
		}

		tx := NewMemoryTransaction(memory)

		// Begin transaction
		err := tx.Begin(ctx)
		require.NoError(t, err)

		// Clear and add new messages
		err = tx.Clear(ctx)
		require.NoError(t, err)

		err = tx.ApplyMessages(ctx, []llm.Message{
			{Role: llm.MessageRoleUser, Content: "New message"},
		})
		require.NoError(t, err)

		// Commit
		err = tx.Commit()
		require.NoError(t, err)

		// Verify final state
		msgs, _ := memory.Read(ctx)
		assert.Len(t, msgs, 1)
		assert.Equal(t, "New message", msgs[0].Content)
	})

	t.Run("rollback restores original state", func(t *testing.T) {
		originalMessages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Original 1"},
			{Role: llm.MessageRoleUser, Content: "Original 2"},
		}

		memory := &testMemory{
			messages: append([]llm.Message{}, originalMessages...),
		}

		tx := NewMemoryTransaction(memory)

		// Begin transaction
		err := tx.Begin(ctx)
		require.NoError(t, err)

		// Clear memory
		err = tx.Clear(ctx)
		require.NoError(t, err)
		msgCount, _ := memory.Len(ctx)
		assert.Equal(t, 0, msgCount)

		// Rollback
		err = tx.Rollback(ctx)
		require.NoError(t, err)

		// Verify original state restored
		msgs, _ := memory.Read(ctx)
		assert.Len(t, msgs, 2)
		assert.Equal(t, "Original 1", msgs[0].Content)
		assert.Equal(t, "Original 2", msgs[1].Content)
	})
}
