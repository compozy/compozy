package service

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/testutil"
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

func (m *testMemory) ReadPaginated(_ context.Context, offset, limit int) ([]llm.Message, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	totalCount := len(m.messages)
	if offset >= totalCount {
		return []llm.Message{}, totalCount, nil
	}
	end := offset + limit
	if end > totalCount {
		end = totalCount
	}
	msgCopy := make([]llm.Message, end-offset)
	copy(msgCopy, m.messages[offset:end])
	return msgCopy, totalCount, nil
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
		TokenCount:     len(m.messages) * 50,
		MessageCount:   len(m.messages),
		ActualStrategy: "fifo",
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
			{"Should accept valid ref", "valid_memory_123", false},
			{"Should reject empty ref", "", true},
			{"Should reject invalid chars", "invalid-memory!", true},
			{"Should reject ref that is too long", string(make([]byte, 101)), true},
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
			{"Should accept valid key", "valid_key_123", false},
			{"Should accept key with spaces", "key with spaces", false},
			{"Should reject empty key", "", true},
			{"Should reject key with control chars", "invalid\x00key", true},
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
			name:    "Should convert string payload to user message",
			payload: "Hello world",
			want:    []llm.Message{{Role: llm.MessageRoleUser, Content: "Hello world"}},
		},
		{
			name: "Should convert single message map",
			payload: map[string]any{
				"role":    "assistant",
				"content": "Hello there!",
			},
			want: []llm.Message{{Role: llm.MessageRoleAssistant, Content: "Hello there!"}},
		},
		{
			name: "Should convert array of messages",
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
			name:    "Should return error for nil payload",
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

	t.Run("Should successfully read messages", func(t *testing.T) {
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
		assert.Equal(t, "Hello", resp.Messages[0].Content)
		assert.Equal(t, llm.MessageRoleUser, resp.Messages[0].Role)
	})

	t.Run("Should return validation error for empty memory reference", func(t *testing.T) {
		// Use a minimal manager for validation testing - validation should occur before manager is used
		manager := &testMemoryManager{}
		service := NewMemoryOperationsService(manager, nil, nil)

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

	t.Run("Should reject invalid payload type", func(t *testing.T) {
		// Setup
		manager := &testMemoryManager{}
		service := NewMemoryOperationsService(manager, nil, nil)

		// Execute with invalid payload type
		resp, err := service.Write(ctx, &WriteRequest{
			BaseRequest: BaseRequest{
				MemoryRef: "test_memory",
				Key:       "test_key",
			},
			Payload: 12345, // Invalid type: int
		})

		// Verify
		assert.Nil(t, resp)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid payload type")
		assert.Contains(t, err.Error(), "unsupported payload type: int")
	})

	t.Run("Should successfully perform atomic write", func(t *testing.T) {
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

	t.Run("Should rollback on failure", func(t *testing.T) {
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

	t.Run("Should resolve templates in payload", func(t *testing.T) {
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

	t.Run("Should reject invalid payload type", func(t *testing.T) {
		// Setup
		manager := &testMemoryManager{}
		service := NewMemoryOperationsService(manager, nil, nil)

		// Execute with invalid payload type
		resp, err := service.Append(ctx, &AppendRequest{
			BaseRequest: BaseRequest{
				MemoryRef: "test_memory",
				Key:       "test_key",
			},
			Payload: []int{1, 2, 3}, // Invalid type: []int
		})

		// Verify
		assert.Nil(t, resp)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid payload type")
		assert.Contains(t, err.Error(), "unsupported payload type: []int")
	})

	t.Run("Should successfully append messages", func(t *testing.T) {
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

	t.Run("Should successfully delete memory", func(t *testing.T) {
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

	t.Run("Should successfully clear with confirmation", func(t *testing.T) {
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

	t.Run("Should require confirmation for clear operation", func(t *testing.T) {
		// Use a minimal manager for validation testing - validation should occur before manager is used
		manager := &testMemoryManager{}
		service := NewMemoryOperationsService(manager, nil, nil)

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

	t.Run("Should complete successful transaction", func(t *testing.T) {
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

	t.Run("Should restore original state on rollback", func(t *testing.T) {
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

// Test with real Redis instances using miniredis
func TestMemoryService_WithRealRedis(t *testing.T) {
	ctx := context.Background()

	t.Run("Should read paginated messages from real Redis", func(t *testing.T) {
		// Setup real Redis environment
		setup := testutil.SetupTestRedis(t)
		defer setup.Cleanup()

		memInstance := setup.CreateTestMemoryInstance(t, "test_memory_read_paginated")

		// Add some messages
		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Message 1"},
			{Role: llm.MessageRoleAssistant, Content: "Message 2"},
			{Role: llm.MessageRoleUser, Content: "Message 3"},
			{Role: llm.MessageRoleAssistant, Content: "Message 4"},
			{Role: llm.MessageRoleUser, Content: "Message 5"},
		}

		for _, msg := range messages {
			err := memInstance.Append(ctx, msg)
			require.NoError(t, err)
		}

		// Create service with real memory manager
		service := NewMemoryOperationsService(setup.Manager, nil, nil)

		// Test paginated read
		resp, err := service.ReadPaginated(ctx, &ReadPaginatedRequest{
			BaseRequest: BaseRequest{
				MemoryRef: "test_memory_read_paginated",
				Key:       "test_memory_read_paginated",
			},
			Offset: 1,
			Limit:  2,
		})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 2, resp.Count)
		assert.Equal(t, 5, resp.TotalCount)
		assert.Equal(t, 1, resp.Offset)
		assert.Equal(t, 2, resp.Limit)
		assert.True(t, resp.HasMore)
		assert.Len(t, resp.Messages, 2)
		assert.Equal(t, "Message 2", resp.Messages[0].Content)
		assert.Equal(t, "Message 3", resp.Messages[1].Content)
	})

	t.Run("Should write and read messages with real Redis", func(t *testing.T) {
		// Setup real Redis environment
		setup := testutil.SetupTestRedis(t)
		defer setup.Cleanup()

		// Create memory configuration first
		_ = setup.CreateTestMemoryInstance(t, "test_memory_write")

		// Create service with real memory manager
		service := NewMemoryOperationsService(setup.Manager, nil, nil)

		// Test write operation
		writeResp, err := service.Write(ctx, &WriteRequest{
			BaseRequest: BaseRequest{
				MemoryRef: "test_memory_write",
				Key:       "test_key",
			},
			Payload: []map[string]any{
				{"role": "user", "content": "Hello from real Redis!"},
				{"role": "assistant", "content": "Hi there from Redis!"},
			},
		})

		require.NoError(t, err)
		assert.True(t, writeResp.Success)
		assert.Equal(t, 2, writeResp.Count)

		// Test read operation
		readResp, err := service.Read(ctx, &ReadRequest{
			BaseRequest: BaseRequest{
				MemoryRef: "test_memory_write",
				Key:       "test_key",
			},
		})

		require.NoError(t, err)
		assert.Equal(t, 2, readResp.Count)
		assert.Len(t, readResp.Messages, 2)
		assert.Equal(t, "Hello from real Redis!", readResp.Messages[0].Content)
		assert.Equal(t, "Hi there from Redis!", readResp.Messages[1].Content)
	})

	t.Run("Should append messages with real Redis", func(t *testing.T) {
		// Setup real Redis environment
		setup := testutil.SetupTestRedis(t)
		defer setup.Cleanup()

		memInstance := setup.CreateTestMemoryInstance(t, "test_memory_append")

		// Add initial message
		err := memInstance.Append(ctx, llm.Message{
			Role:    llm.MessageRoleUser,
			Content: "Initial message",
		})
		require.NoError(t, err)

		// Create service with real memory manager
		service := NewMemoryOperationsService(setup.Manager, nil, nil)

		// Test append operation
		appendResp, err := service.Append(ctx, &AppendRequest{
			BaseRequest: BaseRequest{
				MemoryRef: "test_memory_append",
				Key:       "test_memory_append",
			},
			Payload: map[string]any{
				"role":    "assistant",
				"content": "Appended message",
			},
		})

		require.NoError(t, err)
		assert.True(t, appendResp.Success)
		assert.Equal(t, 1, appendResp.Appended)
		assert.Equal(t, 2, appendResp.TotalCount)

		// Verify total messages
		msgs, err := memInstance.Read(ctx)
		require.NoError(t, err)
		assert.Len(t, msgs, 2)
		assert.Equal(t, "Initial message", msgs[0].Content)
		assert.Equal(t, "Appended message", msgs[1].Content)
	})
}

func TestDependencyValidation(t *testing.T) {
	t.Run("Should panic when memoryManager is nil", func(t *testing.T) {
		assert.Panics(t, func() {
			NewMemoryOperationsService(nil, nil, nil)
		}, "Expected panic when memoryManager is nil")
	})

	t.Run("Should allow nil templateEngine and tokenCounter", func(t *testing.T) {
		manager := &testMemoryManager{}
		assert.NotPanics(t, func() {
			service := NewMemoryOperationsService(manager, nil, nil)
			assert.NotNil(t, service)
		}, "Should not panic when only memoryManager is provided")
	})
}

func TestConfigurableContentLimits(t *testing.T) {
	t.Run("Should use default values when environment variables are not set", func(t *testing.T) {
		// Clear environment variables
		os.Unsetenv("MAX_MESSAGE_CONTENT_LENGTH")
		os.Unsetenv("MAX_TOTAL_CONTENT_SIZE")
		config := DefaultConfig()
		assert.Equal(t, 10*1024, config.ValidationLimits.MaxMessageContentLength) // 10KB
		assert.Equal(t, 100*1024, config.ValidationLimits.MaxTotalContentSize)    // 100KB
	})
	t.Run("Should use environment variables when set", func(t *testing.T) {
		// Set environment variables
		os.Setenv("MAX_MESSAGE_CONTENT_LENGTH", "20480") // 20KB
		os.Setenv("MAX_TOTAL_CONTENT_SIZE", "512000")    // 500KB
		defer func() {
			os.Unsetenv("MAX_MESSAGE_CONTENT_LENGTH")
			os.Unsetenv("MAX_TOTAL_CONTENT_SIZE")
		}()
		config := DefaultConfig()
		assert.Equal(t, 20480, config.ValidationLimits.MaxMessageContentLength)
		assert.Equal(t, 512000, config.ValidationLimits.MaxTotalContentSize)
	})
	t.Run("Should fallback to defaults with invalid environment variables", func(t *testing.T) {
		// Set invalid environment variables
		os.Setenv("MAX_MESSAGE_CONTENT_LENGTH", "invalid")
		os.Setenv("MAX_TOTAL_CONTENT_SIZE", "-1000")
		defer func() {
			os.Unsetenv("MAX_MESSAGE_CONTENT_LENGTH")
			os.Unsetenv("MAX_TOTAL_CONTENT_SIZE")
		}()
		config := DefaultConfig()
		assert.Equal(t, 10*1024, config.ValidationLimits.MaxMessageContentLength) // 10KB default
		assert.Equal(t, 100*1024, config.ValidationLimits.MaxTotalContentSize)    // 100KB default
	})
}

func TestTokenCountingNonBlocking(t *testing.T) {
	t.Run("Should not block operations when token counting fails", func(t *testing.T) {
		// Create a service with a nil token counter (won't cause failures)
		setup := testutil.SetupTestRedis(t)
		defer setup.Cleanup()
		svc := NewMemoryOperationsService(setup.Manager, nil, nil)
		// Test that operations continue normally even without token counting
		// This verifies the non-blocking behavior by ensuring the method doesn't panic
		messages := []llm.Message{
			{Role: "user", Content: "Hello world"},
			{Role: "assistant", Content: "Hi there!"},
		}
		ctx := context.Background()
		// Call the helper method to verify it returns 0 tokens when counter is nil
		tokens := svc.(*memoryOperationsService).calculateTokensNonBlocking(ctx, messages)
		assert.Equal(t, 0, tokens, "Should return 0 tokens when token counter is nil")
	})
}

// Test payload type validation
func TestValidatePayloadType(t *testing.T) {
	tests := []struct {
		name    string
		payload any
		wantErr bool
	}{
		{"Should accept string payload", "hello world", false},
		{"Should accept single message map", map[string]any{"role": "user", "content": "hello"}, false},
		{"Should accept array of message maps", []map[string]any{{"role": "user", "content": "hello"}}, false},
		{"Should accept array of any", []any{map[string]any{"role": "user", "content": "hello"}}, false},
		{"Should reject nil payload", nil, true},
		{"Should reject int payload", 123, true},
		{"Should reject bool payload", true, true},
		{"Should reject struct payload", struct{ Name string }{Name: "test"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePayloadType(tt.payload)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test flushable memory for testing flush operations
type testFlushableMemory struct {
	*testMemory
	performFlushFn             func(ctx context.Context) (*memcore.FlushMemoryActivityOutput, error)
	performFlushWithStrategyFn func(ctx context.Context, strategy memcore.FlushingStrategyType) (*memcore.FlushMemoryActivityOutput, error)
	getConfiguredStrategyFn    func() memcore.FlushingStrategyType
}

func (m *testFlushableMemory) PerformFlush(ctx context.Context) (*memcore.FlushMemoryActivityOutput, error) {
	if m.performFlushFn != nil {
		return m.performFlushFn(ctx)
	}
	return &memcore.FlushMemoryActivityOutput{
		Success:      true,
		MessageCount: len(m.messages),
		TokenCount:   len(m.messages) * 50,
	}, nil
}

func (m *testFlushableMemory) PerformFlushWithStrategy(
	ctx context.Context,
	strategy memcore.FlushingStrategyType,
) (*memcore.FlushMemoryActivityOutput, error) {
	if m.performFlushWithStrategyFn != nil {
		return m.performFlushWithStrategyFn(ctx, strategy)
	}
	return &memcore.FlushMemoryActivityOutput{
		Success:      true,
		MessageCount: len(m.messages),
		TokenCount:   len(m.messages) * 50,
	}, nil
}

func (m *testFlushableMemory) GetConfiguredStrategy() memcore.FlushingStrategyType {
	if m.getConfiguredStrategyFn != nil {
		return m.getConfiguredStrategyFn()
	}
	return memcore.SimpleFIFOFlushing
}

func (m *testFlushableMemory) MarkFlushPending(_ context.Context, _ bool) error {
	return nil
}

// Simple flushable memory that doesn't implement dynamic interface
type simpleFlushableMemory struct {
	*testMemory
	performFlushFn    func(ctx context.Context) (*memcore.FlushMemoryActivityOutput, error)
	getMemoryHealthFn func(ctx context.Context) (*memcore.Health, error)
}

func (m *simpleFlushableMemory) PerformFlush(ctx context.Context) (*memcore.FlushMemoryActivityOutput, error) {
	if m.performFlushFn != nil {
		return m.performFlushFn(ctx)
	}
	return &memcore.FlushMemoryActivityOutput{
		Success:      true,
		MessageCount: len(m.messages),
		TokenCount:   len(m.messages) * 50,
	}, nil
}

func (m *simpleFlushableMemory) GetMemoryHealth(ctx context.Context) (*memcore.Health, error) {
	if m.getMemoryHealthFn != nil {
		return m.getMemoryHealthFn(ctx)
	}
	// Call parent implementation
	return m.testMemory.GetMemoryHealth(ctx)
}

func (m *simpleFlushableMemory) MarkFlushPending(_ context.Context, _ bool) error {
	return nil
}

// Test Flush operation
func TestMemoryService_Flush(t *testing.T) {
	ctx := context.Background()

	t.Run("Should use requested strategy when provided", func(t *testing.T) {
		memory := &testFlushableMemory{
			testMemory: &testMemory{
				messages: []llm.Message{
					{Role: llm.MessageRoleUser, Content: "Message 1"},
					{Role: llm.MessageRoleUser, Content: "Message 2"},
				},
			},
			performFlushWithStrategyFn: func(_ context.Context, strategy memcore.FlushingStrategyType) (*memcore.FlushMemoryActivityOutput, error) {
				assert.Equal(t, memcore.FlushingStrategyType("lru"), strategy)
				return &memcore.FlushMemoryActivityOutput{
					Success:          true,
					MessageCount:     2,
					TokenCount:       100,
					SummaryGenerated: false,
				}, nil
			},
			getConfiguredStrategyFn: func() memcore.FlushingStrategyType {
				return memcore.SimpleFIFOFlushing
			},
		}

		manager := &testMemoryManager{
			getInstance: func(_ context.Context, _ core.MemoryReference, _ map[string]any) (memcore.Memory, error) {
				return memory, nil
			},
		}

		service := NewMemoryOperationsService(manager, nil, nil)

		// Execute with requested strategy
		resp, err := service.Flush(ctx, &FlushRequest{
			BaseRequest: BaseRequest{
				MemoryRef: "test_memory",
				Key:       "test_key",
			},
			Config: &FlushConfig{
				Strategy: "lru",
			},
		})

		// Assert
		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Equal(t, "lru", resp.ActualStrategy)
		assert.Equal(t, 2, resp.MessageCount)
		assert.Equal(t, 100, resp.TokenCount)
	})

	t.Run("Should use configured strategy when no strategy requested", func(t *testing.T) {
		memory := &testFlushableMemory{
			testMemory: &testMemory{
				messages: []llm.Message{
					{Role: llm.MessageRoleUser, Content: "Message 1"},
				},
			},
			performFlushWithStrategyFn: func(_ context.Context, strategy memcore.FlushingStrategyType) (*memcore.FlushMemoryActivityOutput, error) {
				assert.Equal(t, memcore.FlushingStrategyType(""), strategy)
				return &memcore.FlushMemoryActivityOutput{
					Success:      true,
					MessageCount: 1,
					TokenCount:   50,
				}, nil
			},
			getConfiguredStrategyFn: func() memcore.FlushingStrategyType {
				return memcore.TokenAwareLRUFlushing
			},
		}

		manager := &testMemoryManager{
			getInstance: func(_ context.Context, _ core.MemoryReference, _ map[string]any) (memcore.Memory, error) {
				return memory, nil
			},
		}

		service := NewMemoryOperationsService(manager, nil, nil)

		// Execute without strategy
		resp, err := service.Flush(ctx, &FlushRequest{
			BaseRequest: BaseRequest{
				MemoryRef: "test_memory",
				Key:       "test_key",
			},
		})

		// Assert
		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Equal(t, "token_aware_lru", resp.ActualStrategy)
		assert.Equal(t, 1, resp.MessageCount)
		assert.Equal(t, 50, resp.TokenCount)
	})

	t.Run("Should fallback to standard flush for non-dynamic memory", func(t *testing.T) {
		// Create a simple flushable memory that doesn't support dynamic strategy
		memory := &simpleFlushableMemory{
			testMemory: &testMemory{
				messages: []llm.Message{
					{Role: llm.MessageRoleUser, Content: "Message 1"},
				},
			},
			performFlushFn: func(_ context.Context) (*memcore.FlushMemoryActivityOutput, error) {
				return &memcore.FlushMemoryActivityOutput{
					Success:      true,
					MessageCount: 1,
					TokenCount:   50,
				}, nil
			},
		}

		manager := &testMemoryManager{
			getInstance: func(_ context.Context, _ core.MemoryReference, _ map[string]any) (memcore.Memory, error) {
				return memory, nil
			},
		}

		service := NewMemoryOperationsService(manager, nil, nil)

		// Execute with strategy (should be ignored)
		resp, err := service.Flush(ctx, &FlushRequest{
			BaseRequest: BaseRequest{
				MemoryRef: "test_memory",
				Key:       "test_key",
			},
			Config: &FlushConfig{
				Strategy: "lru",
			},
		})

		// Assert
		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Equal(t, "fifo", resp.ActualStrategy) // Should get strategy from GetMemoryHealth
		assert.Equal(t, 1, resp.MessageCount)
		assert.Equal(t, 50, resp.TokenCount)
	})

	t.Run("Should handle dry run with requested strategy", func(t *testing.T) {
		memory := &testFlushableMemory{
			testMemory: &testMemory{
				messages: []llm.Message{
					{Role: llm.MessageRoleUser, Content: "Message 1"},
				},
			},
		}

		manager := &testMemoryManager{
			getInstance: func(_ context.Context, _ core.MemoryReference, _ map[string]any) (memcore.Memory, error) {
				return memory, nil
			},
		}

		service := NewMemoryOperationsService(manager, nil, nil)

		// Execute dry run with strategy
		resp, err := service.Flush(ctx, &FlushRequest{
			BaseRequest: BaseRequest{
				MemoryRef: "test_memory",
				Key:       "test_key",
			},
			Config: &FlushConfig{
				Strategy: "lru",
				DryRun:   true,
			},
		})

		// Assert
		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.True(t, resp.DryRun)
		assert.Equal(t, "lru", resp.ActualStrategy)
		assert.True(t, resp.WouldFlush)
	})

	t.Run("Should use 'unknown' when GetMemoryHealth fails for non-dynamic memory", func(t *testing.T) {
		// Create a simple flushable memory that returns error for GetMemoryHealth
		memory := &simpleFlushableMemory{
			testMemory: &testMemory{
				messages: []llm.Message{
					{Role: llm.MessageRoleUser, Content: "Message 1"},
				},
			},
			performFlushFn: func(_ context.Context) (*memcore.FlushMemoryActivityOutput, error) {
				return &memcore.FlushMemoryActivityOutput{
					Success:      true,
					MessageCount: 1,
					TokenCount:   50,
				}, nil
			},
			getMemoryHealthFn: func(_ context.Context) (*memcore.Health, error) {
				return nil, errors.New("health check failed")
			},
		}

		manager := &testMemoryManager{
			getInstance: func(_ context.Context, _ core.MemoryReference, _ map[string]any) (memcore.Memory, error) {
				return memory, nil
			},
		}

		service := NewMemoryOperationsService(manager, nil, nil)

		// Execute with strategy (should be ignored)
		resp, err := service.Flush(ctx, &FlushRequest{
			BaseRequest: BaseRequest{
				MemoryRef: "test_memory",
				Key:       "test_key",
			},
			Config: &FlushConfig{
				Strategy: "lru",
			},
		})

		// Assert
		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Equal(t, "unknown", resp.ActualStrategy) // Should fallback to unknown when health fails
		assert.Equal(t, 1, resp.MessageCount)
		assert.Equal(t, 50, resp.TokenCount)
	})
}
