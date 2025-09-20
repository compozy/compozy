package memory

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/pkg/tplengine"
)

// TestNewMemoryFormatIntegration tests the new simplified memory format
func TestNewMemoryFormatIntegration(t *testing.T) {
	t.Run("Should resolve single memory reference", func(t *testing.T) {
		// Setup test environment with real Redis
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()

		// Create a test agent with new memory format
		agentConfig := createTestAgentWithNewMemoryFormat(t, env)

		// Create MemoryResolver
		tplEngine := tplengine.NewEngine(tplengine.FormatText)
		workflowContext := map[string]any{
			"workflow": map[string]any{
				"input": map[string]any{
					"user_id": "test-user-123",
				},
			},
			"project": map[string]any{
				"id": "test-project",
			},
		}

		memoryResolver := uc.NewMemoryResolver(env.GetMemoryManager(), tplEngine, workflowContext)

		// Resolve agent memories
		memories, err := memoryResolver.ResolveAgentMemories(ctx, agentConfig)
		require.NoError(t, err)
		require.NotNil(t, memories)

		// Should have one memory instance resolved
		assert.Len(t, memories, 1)
		memory, exists := memories["customer-support"]
		require.True(t, exists, "Memory should be resolved by ID")
		assert.NotNil(t, memory)

		// Test memory operations
		testMsg := llm.Message{Role: llm.MessageRoleUser, Content: "Hello from new format test"}
		err = memory.Append(ctx, testMsg)
		require.NoError(t, err)

		// Read back the message
		messages, err := memory.Read(ctx)
		require.NoError(t, err)
		assert.Len(t, messages, 1)
		assert.Equal(t, testMsg.Content, messages[0].Content)
		assert.Equal(t, testMsg.Role, messages[0].Role)
	})

	t.Run("Should resolve multiple memory references", func(t *testing.T) {
		// Setup test environment
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()

		// Create agent with multiple memory references
		agentConfig := createTestAgentWithMultipleNewMemoryFormat(t, env)

		// Create MemoryResolver with different template variables
		tplEngine := tplengine.NewEngine(tplengine.FormatText)
		workflowContext := map[string]any{
			"workflow": map[string]any{
				"input": map[string]any{
					"user_id":    "multi-user-456",
					"session_id": "session-789",
				},
			},
		}

		memoryResolver := uc.NewMemoryResolver(env.GetMemoryManager(), tplEngine, workflowContext)

		// Resolve agent memories
		memories, err := memoryResolver.ResolveAgentMemories(ctx, agentConfig)
		require.NoError(t, err)
		require.NotNil(t, memories)

		// Should have multiple memory instances
		assert.Len(t, memories, 2)

		userMemory, exists := memories["customer-support"]
		require.True(t, exists)
		assert.NotNil(t, userMemory)

		sharedMemory, exists := memories["shared-memory"]
		require.True(t, exists)
		assert.NotNil(t, sharedMemory)

		// Test that memories are isolated
		userMsg := llm.Message{Role: llm.MessageRoleUser, Content: "User-specific message"}
		err = userMemory.Append(ctx, userMsg)
		require.NoError(t, err)

		sharedMsg := llm.Message{Role: llm.MessageRoleAssistant, Content: "Shared knowledge"}
		err = sharedMemory.Append(ctx, sharedMsg)
		require.NoError(t, err)

		// Verify isolation
		userMessages, err := userMemory.Read(ctx)
		require.NoError(t, err)
		assert.Len(t, userMessages, 1)
		assert.Equal(t, "User-specific message", userMessages[0].Content)

		sharedMessages, err := sharedMemory.Read(ctx)
		require.NoError(t, err)
		assert.Len(t, sharedMessages, 1)
		assert.Equal(t, "Shared knowledge", sharedMessages[0].Content)
	})

	t.Run("Should handle read-only memory references", func(t *testing.T) {
		// Setup test environment
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()

		// Create agent with read-only memory
		agentConfig := createTestAgentWithReadOnlyNewMemoryFormat(t, env)

		// Create MemoryResolver
		tplEngine := tplengine.NewEngine(tplengine.FormatText)
		workflowContext := map[string]any{
			"workflow": map[string]any{
				"input": map[string]any{
					"user_id": "readonly-user",
				},
			},
		}

		memoryResolver := uc.NewMemoryResolver(env.GetMemoryManager(), tplEngine, workflowContext)

		// Resolve agent memories - should skip read-only for now
		memories, err := memoryResolver.ResolveAgentMemories(ctx, agentConfig)
		require.NoError(t, err)

		// Should be empty map since read-only memories are skipped
		assert.NotNil(t, memories)
		assert.Empty(t, memories)
	})
}

// Helper functions to create test agent configurations with new memory format

func createTestAgentWithNewMemoryFormat(t *testing.T, _ *TestEnvironment) *agent.Config {
	t.Helper()

	// Create CWD for agent
	cwd, err := core.CWDFromPath("/tmp/test-agent")
	require.NoError(t, err)

	agentConfig := &agent.Config{
		ID:           "test-agent-new-format",
		Model:        agent.Model{Config: core.ProviderConfig{Provider: core.ProviderMock, Model: "test-model"}},
		Instructions: "Test agent with new memory format",
		LLMProperties: agent.LLMProperties{
			Memory: []core.MemoryReference{
				{ID: "customer-support", Key: "user:{{.workflow.input.user_id}}", Mode: "read-write"},
			},
		},
		CWD: cwd,
		Actions: []*agent.ActionConfig{
			{
				ID:     "chat",
				Prompt: "Chat with memory",
			},
		},
	}

	// Set CWD for actions
	err = agentConfig.SetCWD("/tmp/test-agent")
	require.NoError(t, err)

	// Validate the configuration to process memory settings
	err = agentConfig.Validate()
	require.NoError(t, err, "Agent config should be valid")

	return agentConfig
}

func createTestAgentWithMultipleNewMemoryFormat(t *testing.T, _ *TestEnvironment) *agent.Config {
	t.Helper()

	// Create CWD for agent
	cwd, err := core.CWDFromPath("/tmp/test-agent-multi")
	require.NoError(t, err)

	agentConfig := &agent.Config{
		ID:           "test-agent-multi-new-format",
		Model:        agent.Model{Config: core.ProviderConfig{Provider: core.ProviderMock, Model: "test-model"}},
		Instructions: "Test agent with multiple memories in new format",
		LLMProperties: agent.LLMProperties{
			Memory: []core.MemoryReference{
				{ID: "customer-support", Key: "user:{{.workflow.input.user_id}}", Mode: "read-write"},
				{ID: "shared-memory", Key: "shared:{{.workflow.input.session_id}}", Mode: "read-write"},
			},
		},
		CWD: cwd,
		Actions: []*agent.ActionConfig{
			{
				ID:     "process",
				Prompt: "Process with multiple memories",
			},
		},
	}

	// Set CWD for actions
	err = agentConfig.SetCWD("/tmp/test-agent")
	require.NoError(t, err)

	// Validate the configuration to process memory settings
	err = agentConfig.Validate()
	require.NoError(t, err, "Agent config should be valid")

	return agentConfig
}

func createTestAgentWithReadOnlyNewMemoryFormat(t *testing.T, _ *TestEnvironment) *agent.Config {
	t.Helper()

	// Create CWD for agent
	cwd, err := core.CWDFromPath("/tmp/test-agent-readonly")
	require.NoError(t, err)

	agentConfig := &agent.Config{
		ID:           "test-agent-readonly-new-format",
		Model:        agent.Model{Config: core.ProviderConfig{Provider: core.ProviderMock, Model: "test-model"}},
		Instructions: "Test agent with read-only memory in new format",
		LLMProperties: agent.LLMProperties{
			Memory: []core.MemoryReference{
				{ID: "customer-support", Key: "readonly:{{.workflow.input.user_id}}", Mode: "read-only"},
			},
		},
		CWD: cwd,
		Actions: []*agent.ActionConfig{
			{
				ID:     "readonly_test",
				Prompt: "Test read-only memory",
			},
		},
	}

	// Set CWD for actions
	err = agentConfig.SetCWD("/tmp/test-agent")
	require.NoError(t, err)

	// Validate the configuration to process memory settings
	err = agentConfig.Validate()
	require.NoError(t, err, "Agent config should be valid")

	return agentConfig
}
