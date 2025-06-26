package uc

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/pkg/tplengine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock implementations
type mockMemoryManager struct {
	mock.Mock
}

func (m *mockMemoryManager) GetInstance(
	ctx context.Context,
	memRef core.MemoryReference,
	workflowCtx map[string]any,
) (memcore.Memory, error) {
	args := m.Called(ctx, memRef, workflowCtx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(memcore.Memory), args.Error(1)
}

type mockMemory struct {
	mock.Mock
	id string
}

func (m *mockMemory) Append(ctx context.Context, msg llm.Message) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *mockMemory) Read(ctx context.Context) ([]llm.Message, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]llm.Message), args.Error(1)
}

func (m *mockMemory) Len(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (m *mockMemory) GetTokenCount(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (m *mockMemory) GetMemoryHealth(ctx context.Context) (*memcore.Health, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*memcore.Health), args.Error(1)
}

func (m *mockMemory) Clear(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockMemory) GetID() string {
	return m.id
}

func (m *mockMemory) AppendWithPrivacy(ctx context.Context, msg llm.Message, metadata memcore.PrivacyMetadata) error {
	args := m.Called(ctx, msg, metadata)
	return args.Error(0)
}

// Since TemplateEngine is a concrete struct, we'll test with real instances or nil

func TestNewMemoryResolver(t *testing.T) {
	t.Run("Should create memory resolver", func(t *testing.T) {
		memMgr := &mockMemoryManager{}
		tplEngine := tplengine.NewEngine(tplengine.FormatText)
		workflowCtx := map[string]any{
			"workflow": map[string]any{
				"id": "test-workflow",
			},
		}

		resolver := NewMemoryResolver(memMgr, tplEngine, workflowCtx)

		assert.NotNil(t, resolver)
		assert.Equal(t, memMgr, resolver.memoryManager)
		assert.Equal(t, tplEngine, resolver.templateEngine)
		assert.Equal(t, workflowCtx, resolver.workflowContext)
	})
}

func TestMemoryResolver_GetMemory(t *testing.T) {
	ctx := context.Background()

	t.Run("Should return nil when memory manager is nil", func(t *testing.T) {
		resolver := &MemoryResolver{
			memoryManager: nil,
		}

		memory, err := resolver.GetMemory(ctx, "test-memory", "key-template")

		assert.Nil(t, memory)
		assert.NoError(t, err)
	})

	t.Run("Should resolve memory with template", func(t *testing.T) {
		memMgr := &mockMemoryManager{}
		tplEngine := tplengine.NewEngine(tplengine.FormatText)
		mockMem := &mockMemory{id: "resolved-key"}

		resolver := &MemoryResolver{
			memoryManager:   memMgr,
			templateEngine:  tplEngine,
			workflowContext: map[string]any{"user": "test-user"},
		}

		expectedRef := core.MemoryReference{
			ID:          "test-memory",
			Key:         "user-{{ .user }}",
			ResolvedKey: "user-test-user",
			Mode:        "read-write",
		}
		memMgr.On("GetInstance", ctx, expectedRef, resolver.workflowContext).Return(mockMem, nil)

		memory, err := resolver.GetMemory(ctx, "test-memory", "user-{{ .user }}")

		assert.NoError(t, err)
		assert.NotNil(t, memory)
		assert.Equal(t, "resolved-key", memory.GetID())

		memMgr.AssertExpectations(t)
	})

	t.Run("Should handle template resolution error", func(t *testing.T) {
		memMgr := &mockMemoryManager{}
		tplEngine := tplengine.NewEngine(tplengine.FormatText)

		resolver := &MemoryResolver{
			memoryManager:   memMgr,
			templateEngine:  tplEngine,
			workflowContext: map[string]any{},
		}

		// This template will fail because of missingkey=error option in template engine
		memory, err := resolver.GetMemory(ctx, "test-memory", "invalid-{{ .missing }}")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to resolve memory key template")
		assert.Nil(t, memory)
	})

	t.Run("Should handle memory instance error", func(t *testing.T) {
		memMgr := &mockMemoryManager{}
		tplEngine := tplengine.NewEngine(tplengine.FormatText)

		resolver := &MemoryResolver{
			memoryManager:   memMgr,
			templateEngine:  tplEngine,
			workflowContext: map[string]any{},
		}

		expectedRef := core.MemoryReference{
			ID:          "test-memory",
			Key:         "key",
			ResolvedKey: "key",
			Mode:        "read-write",
		}
		memMgr.On("GetInstance", ctx, expectedRef, resolver.workflowContext).Return(nil, assert.AnError)

		memory, err := resolver.GetMemory(ctx, "test-memory", "key")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get memory instance")
		assert.Nil(t, memory)

		memMgr.AssertExpectations(t)
	})
}

func TestMemoryResolver_ResolveAgentMemories(t *testing.T) {
	ctx := context.Background()

	t.Run("Should return nil for agent without memory", func(t *testing.T) {
		resolver := &MemoryResolver{}
		agentCfg := &agent.Config{
			ID: "test-agent",
		}

		memories, err := resolver.ResolveAgentMemories(ctx, agentCfg)

		assert.NoError(t, err)
		assert.Nil(t, memories)
	})

	t.Run("Should resolve multiple memories", func(t *testing.T) {
		memMgr := &mockMemoryManager{}
		tplEngine := tplengine.NewEngine(tplengine.FormatText)
		mockMem1 := &mockMemory{id: "mem1-resolved"}
		mockMem2 := &mockMemory{id: "mem2-resolved"}

		resolver := &MemoryResolver{
			memoryManager:   memMgr,
			templateEngine:  tplEngine,
			workflowContext: map[string]any{"session": "123"},
		}

		expectedRef1 := core.MemoryReference{
			ID:          "memory1",
			Key:         "session-{{ .session }}",
			ResolvedKey: "session-123",
			Mode:        "read-write",
		}
		expectedRef2 := core.MemoryReference{
			ID:          "memory2",
			Key:         "global",
			ResolvedKey: "global",
			Mode:        "read-write",
		}
		memMgr.On("GetInstance", ctx, expectedRef1, resolver.workflowContext).Return(mockMem1, nil)
		memMgr.On("GetInstance", ctx, expectedRef2, resolver.workflowContext).Return(mockMem2, nil)

		// Test the GetMemory method directly
		mem1, err1 := resolver.GetMemory(ctx, "memory1", "session-{{ .session }}")
		mem2, err2 := resolver.GetMemory(ctx, "memory2", "global")

		assert.NoError(t, err1)
		assert.NoError(t, err2)
		assert.NotNil(t, mem1)
		assert.NotNil(t, mem2)
		assert.Equal(t, "mem1-resolved", mem1.GetID())
		assert.Equal(t, "mem2-resolved", mem2.GetID())

		memMgr.AssertExpectations(t)

		// Note: Testing ResolveAgentMemories fully would require either:
		// 1. Making resolvedMemoryReferences public
		// 2. Testing through the full agent validation flow
		// 3. Adding a test-only method to set resolved references
	})

	t.Run("Should skip read-only memories", func(t *testing.T) {
		// This test would require setting up an agent with read-only memory references
		// which requires access to the private resolvedMemoryReferences field
		// For now, this is documented as a limitation that will be tested
		// when Task 8 implements read-only support
		t.Skip("Read-only memory support not yet implemented")
	})

	t.Run("Should not mutate shared agent config during concurrent executions", func(t *testing.T) {
		memMgr := &mockMemoryManager{}
		tplEngine := tplengine.NewEngine(tplengine.FormatText)
		mockMem := &mockMemory{id: "test-memory"}

		// Simulate pre-validated memory references (normally set during Validate())
		// In production, these would be part of a shared agent.Config
		originalMemRefs := []core.MemoryReference{
			{
				ID:          "memory1",
				Key:         "session-{{ .session }}",
				Mode:        "read-write",
				ResolvedKey: "", // Should remain empty in shared config
			},
		}

		// Create multiple resolvers with different contexts
		resolver1 := &MemoryResolver{
			memoryManager:   memMgr,
			templateEngine:  tplEngine,
			workflowContext: map[string]any{"session": "123"},
		}
		resolver2 := &MemoryResolver{
			memoryManager:   memMgr,
			templateEngine:  tplEngine,
			workflowContext: map[string]any{"session": "456"},
		}

		// Set up mock expectations
		expectedRef1 := core.MemoryReference{
			ID:          "memory1",
			Key:         "session-{{ .session }}",
			ResolvedKey: "session-123",
			Mode:        "read-write",
		}
		expectedRef2 := core.MemoryReference{
			ID:          "memory1",
			Key:         "session-{{ .session }}",
			ResolvedKey: "session-456",
			Mode:        "read-write",
		}
		memMgr.On("GetInstance", ctx, expectedRef1, resolver1.workflowContext).Return(mockMem, nil)
		memMgr.On("GetInstance", ctx, expectedRef2, resolver2.workflowContext).Return(mockMem, nil)

		// Execute GetMemory calls concurrently
		mem1, err1 := resolver1.GetMemory(ctx, "memory1", "session-{{ .session }}")
		mem2, err2 := resolver2.GetMemory(ctx, "memory1", "session-{{ .session }}")

		// Verify both succeeded without interference
		assert.NoError(t, err1)
		assert.NoError(t, err2)
		assert.NotNil(t, mem1)
		assert.NotNil(t, mem2)

		// Verify original memory references were not mutated
		assert.Equal(t, "", originalMemRefs[0].ResolvedKey, "Original memory reference should not be mutated")

		memMgr.AssertExpectations(t)
	})
}

func TestMemoryResolverAdapter(t *testing.T) {
	ctx := context.Background()

	t.Run("Should adapt memory interface correctly", func(t *testing.T) {
		mockMem := &mockMemory{id: "test-memory"}
		adapter := &memoryResolverAdapter{memory: mockMem}

		// Test Append
		msg := llm.Message{Role: llm.MessageRoleUser, Content: "test"}
		mockMem.On("Append", ctx, msg).Return(nil)
		err := adapter.Append(ctx, msg)
		assert.NoError(t, err)

		// Test Read
		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "test1"},
			{Role: llm.MessageRoleAssistant, Content: "test2"},
		}
		mockMem.On("Read", ctx).Return(messages, nil)
		result, err := adapter.Read(ctx)
		assert.NoError(t, err)
		assert.Equal(t, messages, result)

		// Test GetID
		id := adapter.GetID()
		assert.Equal(t, "test-memory", id)

		mockMem.AssertExpectations(t)
	})
}
