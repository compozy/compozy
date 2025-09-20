package uc

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/testutil"
	"github.com/compozy/compozy/pkg/tplengine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testTimeout = 5 * time.Second

// Helper function to create real memory manager for testing
func setupTestMemoryManager(t *testing.T) (*testutil.TestRedisSetup, memcore.ManagerInterface) {
	t.Helper()
	setup := testutil.SetupTestRedis(t)
	return setup, setup.Manager
}
func TestNewMemoryResolver(t *testing.T) {
	t.Run("Should create memory resolver", func(t *testing.T) {
		setup, memMgr := setupTestMemoryManager(t)
		defer setup.Cleanup()
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
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	t.Run("Should return nil when memory manager is nil", func(t *testing.T) {
		resolver := &MemoryResolver{
			memoryManager: nil,
		}
		memory, err := resolver.GetMemory(ctx, "test-memory", "key-template")
		assert.Nil(t, memory)
		assert.NoError(t, err)
	})
	t.Run("Should resolve memory with template using real Redis", func(t *testing.T) {
		// Setup real Redis environment
		setup, memMgr := setupTestMemoryManager(t)
		defer setup.Cleanup()
		// Pre-create a memory instance that the resolver can find
		_ = setup.CreateTestMemoryInstance(t, "test-memory")
		tplEngine := tplengine.NewEngine(tplengine.FormatText)
		resolver := &MemoryResolver{
			memoryManager:   memMgr,
			templateEngine:  tplEngine,
			workflowContext: map[string]any{"user": "test-user"},
		}
		// Create and test real memory instance using a predefined memory ID
		memory, err := resolver.GetMemory(ctx, "test-memory", "user-{{ .user }}")
		assert.NoError(t, err)
		assert.NotNil(t, memory)
		// The instance ID will be a hash-based key generated from the resolved key
		assert.NotEmpty(t, memory.GetID())
		// Test actual memory operations with real Redis
		testMsg := llm.Message{Role: llm.MessageRoleUser, Content: "Hello from real Redis!"}
		err = memory.Append(ctx, testMsg)
		assert.NoError(t, err)
		// Read back the message
		messages, err := memory.Read(ctx)
		assert.NoError(t, err)
		assert.Len(t, messages, 1)
		assert.Equal(t, testMsg.Content, messages[0].Content)
		assert.Equal(t, testMsg.Role, messages[0].Role)
	})
	t.Run("Should handle template resolution error", func(t *testing.T) {
		setup, memMgr := setupTestMemoryManager(t)
		defer setup.Cleanup()
		// Pre-create a memory instance so config exists
		_ = setup.CreateTestMemoryInstance(t, "test-memory")
		tplEngine := tplengine.NewEngine(tplengine.FormatText)
		resolver := &MemoryResolver{
			memoryManager:   memMgr,
			templateEngine:  tplEngine,
			workflowContext: map[string]any{},
		}
		// This template will now fail fast due to invalid characters after failed resolution
		memory, err := resolver.GetMemory(ctx, "test-memory", "invalid-{{ .missing }}")
		assert.Error(t, err)
		assert.Nil(t, memory)
		assert.ErrorContains(t, err, "failed to execute key template")
	})
	t.Run("Should handle memory instance error", func(t *testing.T) {
		setup, memMgr := setupTestMemoryManager(t)
		defer setup.Cleanup()
		tplEngine := tplengine.NewEngine(tplengine.FormatText)
		resolver := &MemoryResolver{
			memoryManager:   memMgr,
			templateEngine:  tplEngine,
			workflowContext: map[string]any{},
		}
		// Try to get a memory instance that doesn't exist
		memory, err := resolver.GetMemory(ctx, "non-existent-memory", "key")
		assert.Error(t, err)
		assert.ErrorContains(t, err, "failed to get memory instance")
		assert.Nil(t, memory)
	})
}
func TestMemoryResolver_ResolveAgentMemories(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	t.Run("Should return nil for agent without memory", func(t *testing.T) {
		setup, memMgr := setupTestMemoryManager(t)
		defer setup.Cleanup()
		resolver := &MemoryResolver{
			memoryManager: memMgr,
		}
		agentCfg := &agent.Config{
			ID: "test-agent",
		}
		memories, err := resolver.ResolveAgentMemories(ctx, agentCfg)
		assert.NoError(t, err)
		assert.Nil(t, memories)
	})
	t.Run("Should demonstrate template resolution and memory operations", func(t *testing.T) {
		// Setup real Redis environment
		setup, memMgr := setupTestMemoryManager(t)
		defer setup.Cleanup()
		// Pre-create memory instances that the resolvers can find
		_ = setup.CreateTestMemoryInstance(t, "session-memory")
		tplEngine := tplengine.NewEngine(tplengine.FormatText)
		resolver := &MemoryResolver{
			memoryManager:   memMgr,
			templateEngine:  tplEngine,
			workflowContext: map[string]any{"session": "123"},
		}
		// Test the GetMemory method directly with template resolution
		memory, err := resolver.GetMemory(ctx, "session-memory", "session-{{ .session }}")
		require.NoError(t, err)
		require.NotNil(t, memory)
		assert.NotEmpty(t, memory.GetID())
		// Test memory operations
		testMsg := llm.Message{Role: llm.MessageRoleUser, Content: "Session-specific message"}
		err = memory.Append(ctx, testMsg)
		assert.NoError(t, err)
		messages, err := memory.Read(ctx)
		assert.NoError(t, err)
		assert.Len(t, messages, 1)
		assert.Equal(t, "Session-specific message", messages[0].Content)
	})
	t.Run("Should demonstrate concurrent memory isolation with real Redis", func(t *testing.T) {
		// Setup real Redis environment
		setup, memMgr := setupTestMemoryManager(t)
		defer setup.Cleanup()
		// Pre-create memory instances that the resolvers can find
		_ = setup.CreateTestMemoryInstance(t, "session-memory")
		tplEngine := tplengine.NewEngine(tplengine.FormatText)
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
		// Get separate memory instances using real Redis
		mem1, err1 := resolver1.GetMemory(ctx, "session-memory", "session-{{ .session }}")
		mem2, err2 := resolver2.GetMemory(ctx, "session-memory", "session-{{ .session }}")
		// Verify both succeeded
		assert.NoError(t, err1)
		assert.NoError(t, err2)
		assert.NotNil(t, mem1)
		assert.NotNil(t, mem2)
		// Verify they have different IDs (isolated memories)
		assert.NotEmpty(t, mem1.GetID())
		assert.NotEmpty(t, mem2.GetID())
		assert.NotEqual(t, mem1.GetID(), mem2.GetID(), "Memory instances should have different IDs")
		// Test that memories are isolated - writing to one doesn't affect the other
		msg1 := llm.Message{Role: llm.MessageRoleUser, Content: "Message for session 123"}
		msg2 := llm.Message{Role: llm.MessageRoleUser, Content: "Message for session 456"}
		err := mem1.Append(ctx, msg1)
		assert.NoError(t, err)
		err = mem2.Append(ctx, msg2)
		assert.NoError(t, err)
		// Read from both memories and verify isolation
		messages1, err := mem1.Read(ctx)
		assert.NoError(t, err)
		assert.Len(t, messages1, 1)
		assert.Equal(t, "Message for session 123", messages1[0].Content)
		messages2, err := mem2.Read(ctx)
		assert.NoError(t, err)
		assert.Len(t, messages2, 1)
		assert.Equal(t, "Message for session 456", messages2[0].Content)
	})
}
func TestMemoryResolverAdapter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	t.Run("Should adapt real memory interface correctly", func(t *testing.T) {
		// Setup real Redis environment
		setup := testutil.SetupTestRedis(t)
		defer setup.Cleanup()
		realMem := setup.CreateTestMemoryInstance(t, "adapter-test-memory")
		adapter := &memoryResolverAdapter{memory: realMem}
		// Test Append with real Redis
		msg := llm.Message{Role: llm.MessageRoleUser, Content: "test adapter"}
		err := adapter.Append(ctx, msg)
		assert.NoError(t, err)
		// Test Read with real Redis
		result, err := adapter.Read(ctx)
		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "test adapter", result[0].Content)
		assert.Equal(t, llm.MessageRoleUser, result[0].Role)
		// Test GetID - the adapter should return the hash-based ID from the real memory
		id := adapter.GetID()
		assert.NotEmpty(t, id)
	})
	t.Run("Should handle concurrent access correctly", func(t *testing.T) {
		// Setup real Redis environment
		setup := testutil.SetupTestRedis(t)
		defer setup.Cleanup()
		realMem := setup.CreateTestMemoryInstance(t, "concurrent-test-memory")
		adapter := &memoryResolverAdapter{memory: realMem}
		// Test concurrent append operations
		done := make(chan bool, 2)
		errors := make(chan error, 2)
		// First goroutine
		go func() {
			defer func() { done <- true }()
			msg := llm.Message{Role: llm.MessageRoleUser, Content: "Message from goroutine 1"}
			if err := adapter.Append(ctx, msg); err != nil {
				errors <- err
			}
		}()
		// Second goroutine
		go func() {
			defer func() { done <- true }()
			msg := llm.Message{Role: llm.MessageRoleAssistant, Content: "Message from goroutine 2"}
			if err := adapter.Append(ctx, msg); err != nil {
				errors <- err
			}
		}()
		// Wait for both operations to complete
		<-done
		<-done
		// Check for errors
		select {
		case err := <-errors:
			t.Fatalf("Concurrent operation failed: %v", err)
		default:
			// No errors
		}
		// Verify both messages were stored
		messages, err := adapter.Read(ctx)
		require.NoError(t, err)
		assert.Len(t, messages, 2)
		// Verify message content (order may vary due to concurrency)
		contents := make([]string, len(messages))
		for i, msg := range messages {
			contents[i] = msg.Content
		}
		assert.Contains(t, contents, "Message from goroutine 1")
		assert.Contains(t, contents, "Message from goroutine 2")
	})
}

// TestMemoryResolverTemplateResolution tests comprehensive template resolution scenarios
func TestMemoryResolverTemplateResolution(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	t.Run("Should resolve simple template variables", func(t *testing.T) {
		tplEngine := tplengine.NewEngine(tplengine.FormatText)
		workflowContext := map[string]any{
			"user_id": "user123",
			"session": "session456",
		}
		resolver := &MemoryResolver{
			memoryManager:   nil,
			templateEngine:  tplEngine,
			workflowContext: workflowContext,
		}
		// Test simple variable replacement
		resolved, err := resolver.resolveKey(ctx, "user:{{.user_id}}")
		assert.NoError(t, err)
		assert.Equal(t, "user:user123", resolved)
		// Test multiple variables
		resolved, err = resolver.resolveKey(ctx, "{{.user_id}}:{{.session}}")
		assert.NoError(t, err)
		assert.Equal(t, "user123:session456", resolved)
	})
	t.Run("Should resolve nested template variables", func(t *testing.T) {
		tplEngine := tplengine.NewEngine(tplengine.FormatText)
		workflowContext := map[string]any{
			"workflow": map[string]any{
				"input": map[string]any{
					"user_id": "nested123",
				},
				"exec_id": "exec456",
			},
			"project": map[string]any{
				"id": "project789",
			},
		}
		resolver := &MemoryResolver{
			memoryManager:   nil,
			templateEngine:  tplEngine,
			workflowContext: workflowContext,
		}
		// Test nested object access
		resolved, err := resolver.resolveKey(ctx, "user:{{.workflow.input.user_id}}")
		assert.NoError(t, err)
		assert.Equal(t, "user:nested123", resolved)
		// Test complex nested template
		resolved, err = resolver.resolveKey(ctx, "{{.project.id}}:{{.workflow.exec_id}}:{{.workflow.input.user_id}}")
		assert.NoError(t, err)
		assert.Equal(t, "project789:exec456:nested123", resolved)
	})
	t.Run("Should handle template with missing variables", func(t *testing.T) {
		tplEngine := tplengine.NewEngine(tplengine.FormatText)
		workflowContext := map[string]any{
			"user_id": "user123",
		}
		resolver := &MemoryResolver{
			memoryManager:   nil,
			templateEngine:  tplEngine,
			workflowContext: workflowContext,
		}
		// Template with missing variable should fail at MemoryResolver level
		key, err := resolver.resolveKey(ctx, "user:{{.missing_variable}}")
		assert.Error(t, err)
		assert.ErrorContains(t, err, "failed to execute key template")
		assert.Empty(t, key)
	})
	t.Run("Should handle empty template engine", func(t *testing.T) {
		resolver := &MemoryResolver{
			memoryManager:   nil,
			templateEngine:  nil, // No template engine
			workflowContext: map[string]any{},
		}
		// Should return error when template engine is nil and template syntax is detected
		resolved, err := resolver.resolveKey(ctx, "static-key-{{.user_id}}")
		assert.Error(t, err)
		assert.ErrorContains(t, err, "template engine is required")
		assert.Empty(t, resolved)
	})
	t.Run("Should handle complex workflow context structures", func(t *testing.T) {
		tplEngine := tplengine.NewEngine(tplengine.FormatText)
		workflowContext := map[string]any{
			"workflow": map[string]any{
				"id":      "wf-123",
				"exec_id": "exec-456",
				"input": map[string]any{
					"user_id":    "user789",
					"session_id": "sess101",
					"metadata": map[string]any{
						"channel": "web",
						"version": "v2.1",
					},
				},
				"output": map[string]any{
					"status": "success",
				},
			},
			"project": map[string]any{
				"id":      "proj-abc",
				"name":    "test-project",
				"version": "1.0.0",
			},
			"task": map[string]any{
				"id":   "task-def",
				"type": "basic",
			},
		}
		resolver := &MemoryResolver{
			memoryManager:   nil,
			templateEngine:  tplEngine,
			workflowContext: workflowContext,
		}
		testCases := []struct {
			template string
			expected string
		}{
			{
				template: "{{.workflow.id}}:{{.project.id}}:{{.workflow.input.user_id}}",
				expected: "wf-123:proj-abc:user789",
			},
			{
				template: "{{.workflow.input.metadata.channel}}-{{.workflow.input.metadata.version}}",
				expected: "web-v2.1",
			},
			{
				template: "session:{{.workflow.input.session_id}}:{{.task.type}}",
				expected: "session:sess101:basic",
			},
		}
		for _, tc := range testCases {
			resolved, err := resolver.resolveKey(ctx, tc.template)
			assert.NoError(t, err, "Template: %s", tc.template)
			assert.Equal(t, tc.expected, resolved, "Template: %s", tc.template)
		}
	})
	t.Run("Should handle edge cases in template resolution", func(t *testing.T) {
		setup, memMgr := setupTestMemoryManager(t)
		defer setup.Cleanup()
		tplEngine := tplengine.NewEngine(tplengine.FormatText)
		workflowContext := map[string]any{
			"empty_string": "",
			"zero_value":   0,
			"null_value":   nil,
			"bool_value":   true,
		}
		resolver := &MemoryResolver{
			memoryManager:   memMgr,
			templateEngine:  tplEngine,
			workflowContext: workflowContext,
		}
		testCases := []struct {
			template string
			expected string
		}{
			{
				template: "empty:{{.empty_string}}",
				expected: "empty:",
			},
			{
				template: "zero:{{.zero_value}}",
				expected: "zero:0",
			},
			{
				template: "bool:{{.bool_value}}",
				expected: "bool:true",
			},
		}
		for _, tc := range testCases {
			resolved, err := resolver.resolveKey(ctx, tc.template)
			assert.NoError(t, err, "Template: %s", tc.template)
			assert.Equal(t, tc.expected, resolved, "Template: %s", tc.template)
		}
		// Test null value template - template engine handles nil gracefully
		resolved, err := resolver.resolveKey(ctx, "null:{{.null_value}}")
		assert.NoError(t, err)
		assert.Equal(t, "null:<no value>", resolved)
	})
	t.Run("Should handle special characters in templates", func(t *testing.T) {
		setup, memMgr := setupTestMemoryManager(t)
		defer setup.Cleanup()
		tplEngine := tplengine.NewEngine(tplengine.FormatText)
		workflowContext := map[string]any{
			"user_id":    "user@example.com",
			"session_id": "sess-123-456",
			"project":    "my-project_v2",
		}
		resolver := &MemoryResolver{
			memoryManager:   memMgr,
			templateEngine:  tplEngine,
			workflowContext: workflowContext,
		}
		testCases := []struct {
			template string
			expected string
		}{
			{
				template: "user:{{.user_id}}",
				expected: "user:user@example.com",
			},
			{
				template: "session:{{.session_id}}",
				expected: "session:sess-123-456",
			},
			{
				template: "{{.project}}_cache",
				expected: "my-project_v2_cache",
			},
		}
		for _, tc := range testCases {
			resolved, err := resolver.resolveKey(ctx, tc.template)
			assert.NoError(t, err, "Template: %s", tc.template)
			assert.Equal(t, tc.expected, resolved, "Template: %s", tc.template)
		}
	})
}

// TestMemoryResolverWorkflowIntegration tests integration with workflow execution patterns
func TestMemoryResolverWorkflowIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	t.Run("Should handle real workflow execution context", func(t *testing.T) {
		setup, memMgr := setupTestMemoryManager(t)
		defer setup.Cleanup()
		_ = setup.CreateTestMemoryInstance(t, "customer-support")
		tplEngine := tplengine.NewEngine(tplengine.FormatText)
		// Simulate real workflow context from buildWorkflowContext function
		workflowContext := map[string]any{
			"workflow": map[string]any{
				"id":      "memory-api",
				"exec_id": "exec-12345",
				"status":  "running",
				"input": map[string]any{
					"user_id": "api_test_user",
					"message": "Hello, what do you remember about me?",
				},
			},
			"config": map[string]any{
				"id":          "memory-api",
				"version":     "0.1.0",
				"description": "Simple agent with memory for testing REST API endpoints",
			},
			"project": map[string]any{
				"id":          "test-project",
				"name":        "test-project",
				"version":     "1.0.0",
				"description": "Test project for memory integration",
			},
			"task": map[string]any{
				"id":   "chat",
				"type": "basic",
			},
		}
		resolver := NewMemoryResolver(memMgr, tplEngine, workflowContext)
		// Test memory resolution with workflow template
		memory, err := resolver.GetMemory(ctx, "customer-support", "user:{{.workflow.input.user_id}}")
		require.NoError(t, err)
		require.NotNil(t, memory)
		// Verify memory operations work correctly
		testMsg := llm.Message{
			Role:    llm.MessageRoleUser,
			Content: "Integration test message from workflow context",
		}
		err = memory.Append(ctx, testMsg)
		require.NoError(t, err)
		messages, err := memory.Read(ctx)
		require.NoError(t, err)
		assert.Len(t, messages, 1)
		assert.Equal(t, testMsg.Content, messages[0].Content)
	})
	t.Run("Should handle agent configuration memory references", func(t *testing.T) {
		setup, memMgr := setupTestMemoryManager(t)
		defer setup.Cleanup()
		_ = setup.CreateTestMemoryInstance(t, "customer-support")
		tplEngine := tplengine.NewEngine(tplengine.FormatText)
		workflowContext := map[string]any{
			"workflow": map[string]any{
				"input": map[string]any{
					"user_id": "agent_test_user",
				},
			},
		}
		resolver := NewMemoryResolver(memMgr, tplEngine, workflowContext)
		// Create agent config with memory (simulating validated agent)
		agentConfig := &agent.Config{
			ID:           "test-agent",
			Instructions: "Test agent for memory resolver",
			Model:        agent.Model{Config: core.ProviderConfig{Provider: core.ProviderMock, Model: "test-model"}},
		}
		// Manually set resolved memory references (simulating validation)
		// In real code, this is done during agent.Validate() processing
		agentConfig.Memory = []core.MemoryReference{
			{ID: "customer-support", Key: "user:{{.workflow.input.user_id}}", Mode: "read-write"},
		}
		// Create CWD for agent validation
		cwd, err := core.CWDFromPath(t.TempDir())
		require.NoError(t, err)
		agentConfig.CWD = cwd
		// Validate to properly set up memory references
		err = agentConfig.Validate()
		require.NoError(t, err)
		// Test resolving agent memories
		memories, err := resolver.ResolveAgentMemories(ctx, agentConfig)
		require.NoError(t, err)
		require.NotNil(t, memories)
		assert.Len(t, memories, 1)
		memory, exists := memories["customer-support"]
		require.True(t, exists)
		assert.NotNil(t, memory)
		// Test memory functionality
		testMsg := llm.Message{
			Role:    llm.MessageRoleAssistant,
			Content: "Agent memory integration test",
		}
		err = memory.Append(ctx, testMsg)
		require.NoError(t, err)
		messages, err := memory.Read(ctx)
		require.NoError(t, err)
		assert.Len(t, messages, 1)
		assert.Equal(t, testMsg.Content, messages[0].Content)
	})
}

// Note: The helper method for setting resolved memory references is not needed
// as we now use proper agent validation which sets up memory references correctly
