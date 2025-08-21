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

// TestAgentMemoryResolver tests the MemoryResolver integration with agents
func TestAgentMemoryResolver(t *testing.T) {
	t.Run("Should resolve agent memories with templates", func(t *testing.T) {
		// Setup test environment with real Redis
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()

		// Create a test agent with memory configuration
		agentConfig := createTestAgentWithMemory(t, env)

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
		testMsg := llm.Message{Role: llm.MessageRoleUser, Content: "Hello from agent integration test"}
		err = memory.Append(ctx, testMsg)
		require.NoError(t, err)

		// Read back the message
		messages, err := memory.Read(ctx)
		require.NoError(t, err)
		assert.Len(t, messages, 1)
		assert.Equal(t, testMsg.Content, messages[0].Content)
		assert.Equal(t, testMsg.Role, messages[0].Role)
	})

	t.Run("Should handle multiple memory references with different templates", func(t *testing.T) {
		// Setup test environment
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()

		// Create agent with multiple memory configurations
		agentConfig := createTestAgentWithMultipleMemories(t, env)

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

	t.Run("Should handle memory template resolution errors by failing", func(t *testing.T) {
		// Setup test environment
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()

		// Create agent with invalid template
		agentConfig := createTestAgentWithInvalidTemplate(t, env)

		// Create MemoryResolver with missing template variables
		tplEngine := tplengine.NewEngine(tplengine.FormatText)
		workflowContext := map[string]any{
			"workflow": map[string]any{
				"input": map[string]any{
					// Missing user_id that template expects
				},
			},
		}

		memoryResolver := uc.NewMemoryResolver(env.GetMemoryManager(), tplEngine, workflowContext)

		// Should now fail due to invalid template resolution
		memories, err := memoryResolver.ResolveAgentMemories(ctx, agentConfig)
		assert.Error(t, err, "Should fail when template resolution produces invalid key")
		assert.Contains(t, err.Error(), "memory key validation failed", "Error should indicate validation failure")
		assert.Nil(t, memories, "Should not return memories when resolution fails")
	})

	t.Run("Should skip read-only memories during append operations", func(t *testing.T) {
		// Setup test environment
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()

		// Create agent with read-only memory
		agentConfig := createTestAgentWithReadOnlyMemory(t, env)

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

// TestMemoryResolverConcurrency tests concurrent access to MemoryResolver
func TestMemoryResolverConcurrency(t *testing.T) {
	t.Run("Should handle concurrent memory resolution safely", func(t *testing.T) {
		// Setup test environment
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()

		// Create agents with different memory configurations
		agentConfigs := []*agent.Config{
			createTestAgentWithMemory(t, env),
			createTestAgentWithMultipleMemories(t, env),
		}

		// Create multiple MemoryResolvers with different contexts
		tplEngine := tplengine.NewEngine(tplengine.FormatText)

		// Test concurrent resolution
		const numGoroutines = 5
		results := make(chan error, numGoroutines)

		for i := range numGoroutines {
			go func(goroutineID int) {
				workflowContext := map[string]any{
					"workflow": map[string]any{
						"input": map[string]any{
							"user_id":    "concurrent-user-" + string(rune(goroutineID+'0')),
							"session_id": "session-" + string(rune(goroutineID+'0')),
						},
					},
				}

				memoryResolver := uc.NewMemoryResolver(env.GetMemoryManager(), tplEngine, workflowContext)

				// Try resolving memories for different agents
				for _, agentConfig := range agentConfigs {
					memories, err := memoryResolver.ResolveAgentMemories(ctx, agentConfig)
					if err != nil {
						results <- err
						return
					}

					// Test memory operations
					for _, memory := range memories {
						testMsg := llm.Message{
							Role:    llm.MessageRoleUser,
							Content: "Concurrent message from goroutine " + string(rune(goroutineID+'0')),
						}
						if err := memory.Append(ctx, testMsg); err != nil {
							results <- err
							return
						}
					}
				}
				results <- nil
			}(i)
		}

		// Wait for all goroutines and check results
		for i := range numGoroutines {
			err := <-results
			require.NoError(t, err, "Concurrent operation %d should not fail", i)
		}
	})
}

// TestMemoryResolverAdapter tests the adapter implementation
func TestMemoryResolverAdapter(t *testing.T) {
	t.Run("Should properly adapt memory interface", func(t *testing.T) {
		// Setup test environment
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		ctx := context.Background()

		// Get a real memory instance
		memRef := core.MemoryReference{
			ID:          "customer-support",
			Key:         "adapter-test",
			ResolvedKey: "adapter-test",
			Mode:        "read-write",
		}

		memInstance, err := env.GetMemoryManager().GetInstance(ctx, memRef, map[string]any{})
		require.NoError(t, err)

		// Test through MemoryResolver to get adapter
		tplEngine := tplengine.NewEngine(tplengine.FormatText)
		workflowContext := map[string]any{
			"user_id": "adapter-test-user",
		}

		memoryResolver := uc.NewMemoryResolver(env.GetMemoryManager(), tplEngine, workflowContext)
		adaptedMemory, err := memoryResolver.GetMemory(ctx, "customer-support", "adapter-test")
		require.NoError(t, err)
		require.NotNil(t, adaptedMemory)

		// Test adapter methods
		assert.Equal(t, memInstance.GetID(), adaptedMemory.GetID())

		// Test append through adapter
		testMsg := llm.Message{Role: llm.MessageRoleUser, Content: "Adapter test message"}
		err = adaptedMemory.Append(ctx, testMsg)
		require.NoError(t, err)

		// Test read through adapter
		messages, err := adaptedMemory.Read(ctx)
		require.NoError(t, err)
		assert.Len(t, messages, 1)
		assert.Equal(t, testMsg.Content, messages[0].Content)
	})
}

// Helper functions to create test agent configurations

func createTestAgentWithMemory(t *testing.T, _ *TestEnvironment) *agent.Config {
	t.Helper()

	// Create CWD for agent
	cwd, err := core.CWDFromPath("/tmp/test-agent")
	require.NoError(t, err)

	agentConfig := &agent.Config{
		ID:           "test-agent-with-memory",
		Config:       core.ProviderConfig{Provider: core.ProviderMock, Model: "test-model"},
		Instructions: "Test agent with memory integration",
		Memory: []core.MemoryReference{
			{ID: "customer-support", Key: "user:{{.workflow.input.user_id}}", Mode: "read-write"},
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

func createTestAgentWithMultipleMemories(t *testing.T, _ *TestEnvironment) *agent.Config {
	t.Helper()

	// Create CWD for agent
	cwd, err := core.CWDFromPath("/tmp/test-agent-multi")
	require.NoError(t, err)

	agentConfig := &agent.Config{
		ID:           "test-agent-multi-memory",
		Config:       core.ProviderConfig{Provider: core.ProviderMock, Model: "test-model"},
		Instructions: "Test agent with multiple memories",
		Memory: []core.MemoryReference{
			{ID: "customer-support", Key: "user:{{.workflow.input.user_id}}", Mode: "read-write"},
			{ID: "shared-memory", Key: "shared:{{.workflow.input.session_id}}", Mode: "read-write"},
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

func createTestAgentWithInvalidTemplate(t *testing.T, _ *TestEnvironment) *agent.Config {
	t.Helper()

	// Create CWD for agent
	cwd, err := core.CWDFromPath("/tmp/test-agent-invalid")
	require.NoError(t, err)

	agentConfig := &agent.Config{
		ID:           "test-agent-invalid-template",
		Config:       core.ProviderConfig{Provider: core.ProviderMock, Model: "test-model"},
		Instructions: "Test agent with invalid template",
		Memory: []core.MemoryReference{
			{ID: "customer-support", Key: "invalid:{{.workflow.input.missing_variable}}", Mode: "read-write"},
		},
		CWD: cwd,
		Actions: []*agent.ActionConfig{
			{
				ID:     "error_test",
				Prompt: "Test error handling",
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

func createTestAgentWithReadOnlyMemory(t *testing.T, _ *TestEnvironment) *agent.Config {
	t.Helper()

	// Create CWD for agent
	cwd, err := core.CWDFromPath("/tmp/test-agent-readonly")
	require.NoError(t, err)

	agentConfig := &agent.Config{
		ID:           "test-agent-readonly",
		Config:       core.ProviderConfig{Provider: core.ProviderMock, Model: "test-model"},
		Instructions: "Test agent with read-only memory",
		Memory: []core.MemoryReference{
			{ID: "customer-support", Key: "readonly:{{.workflow.input.user_id}}", Mode: "read-only"},
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
