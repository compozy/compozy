package memory

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task/uc"
	wf "github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMemoryOperationConsistency verifies that memory operations are consistent across multiple executions
func TestMemoryOperationConsistency(t *testing.T) {
	// Setup test environment
	env := NewTestEnvironment(t)
	defer env.Cleanup()
	ctx := env.ctx

	// Create test agent with memory configuration
	testAgent := &agent.Config{
		ID:           "memory_test_agent",
		Model:        agent.Model{Config: core.ProviderConfig{Provider: "openai", Model: "gpt-4"}},
		Instructions: "Test agent with memory",
		LLMProperties: agent.LLMProperties{
			Memory: []core.MemoryReference{
				{
					ID:   "customer-support", // Use existing registered memory config
					Key:  "user:{{.workflow.input.user_id}}",
					Mode: "read-write",
				},
			},
		},
		Actions: []*agent.ActionConfig{
			{
				ID:     "test_action",
				Prompt: "Test prompt",
			},
		},
	}

	t.Run("Should consistently resolve memory references across multiple executions", func(t *testing.T) {
		const numExecutions = 10
		results := make([]string, numExecutions)
		errors := make([]error, numExecutions)

		// Run the same execution multiple times
		for i := range numExecutions {
			t.Run(fmt.Sprintf("Execution_%d", i+1), func(t *testing.T) {
				// Create workflow state with consistent input
				execID, _ := core.NewID()
				workflowState := &wf.State{
					WorkflowID:     "test-workflow",
					WorkflowExecID: execID,
				}

				// Build workflow context
				workflowContext := map[string]any{
					"workflow": map[string]any{
						"id":      workflowState.WorkflowID,
						"exec_id": workflowState.WorkflowExecID.String(),
						"input":   map[string]any{"user_id": "test_user_123"},
					},
				}

				// Create memory resolver
				memoryResolver := uc.NewMemoryResolver(
					env.memoryManager,
					env.tplEngine,
					workflowContext,
				)

				// Resolve agent memories
				memories, err := memoryResolver.ResolveAgentMemories(ctx, testAgent)
				if err != nil {
					errors[i] = err
					return
				}

				// Verify memory was resolved
				require.NotNil(t, memories, "Memories should not be nil")
				require.Len(t, memories, 1, "Should have exactly one memory")

				// Get the memory instance
				memory, exists := memories["customer-support"]
				require.True(t, exists, "Memory with ID 'customer-support' should exist")
				require.NotNil(t, memory, "Memory instance should not be nil")

				// Store the memory ID for comparison
				results[i] = memory.GetID()
			})
		}

		// Verify all executions produced the same result
		for i, err := range errors {
			assert.NoError(t, err, "Execution %d should not have errors", i+1)
		}

		// All memory IDs should be the same
		expectedID := results[0]
		for i := 1; i < numExecutions; i++ {
			assert.Equal(t, expectedID, results[i],
				"Memory ID should be consistent across executions (execution %d vs 1)", i+1)
		}
	})

	t.Run("Should handle concurrent memory operations safely", func(t *testing.T) {
		const numGoroutines = 20
		var wg sync.WaitGroup
		errors := make(chan error, numGoroutines)
		results := make(chan string, numGoroutines)

		// Run concurrent memory operations
		for i := range numGoroutines {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				// Create unique workflow execution ID for each goroutine
				execID, _ := core.NewID()
				workflowState := &wf.State{
					WorkflowID:     "concurrent-workflow",
					WorkflowExecID: execID,
				}

				// Build workflow context
				workflowContext := map[string]any{
					"workflow": map[string]any{
						"id":      workflowState.WorkflowID,
						"exec_id": workflowState.WorkflowExecID.String(),
						"input":   map[string]any{"user_id": fmt.Sprintf("concurrent_user_%d", idx)},
					},
				}

				// Create memory resolver
				memoryResolver := uc.NewMemoryResolver(
					env.memoryManager,
					env.tplEngine,
					workflowContext,
				)

				// Resolve agent memories
				memories, err := memoryResolver.ResolveAgentMemories(ctx, testAgent)
				if err != nil {
					errors <- err
					return
				}

				// Verify memory was resolved
				if len(memories) != 1 {
					errors <- fmt.Errorf("expected 1 memory, got %d", len(memories))
					return
				}

				// Get the memory instance
				memory, exists := memories["customer-support"]
				if !exists || memory == nil {
					errors <- fmt.Errorf("memory not found or nil")
					return
				}

				results <- memory.GetID()
			}(i)
		}

		// Wait for all goroutines to complete
		wg.Wait()
		close(errors)
		close(results)

		// Check for errors
		for err := range errors {
			assert.NoError(t, err, "Concurrent execution should not have errors")
		}

		// Verify all results are unique (different user IDs should produce different memory keys)
		uniqueResults := make(map[string]bool)
		for result := range results {
			uniqueResults[result] = true
		}
		assert.Equal(t, numGoroutines, len(uniqueResults),
			"Each concurrent execution with different user_id should produce a unique memory key")
	})

	t.Run("Should handle empty memory configuration gracefully", func(t *testing.T) {
		// Create agent without memory configuration
		agentNoMemory := &agent.Config{
			ID:           "agent_without_memory",
			Model:        agent.Model{Config: core.ProviderConfig{Provider: "openai", Model: "gpt-4"}},
			Instructions: "Test agent without memory",
			LLMProperties: agent.LLMProperties{
				Memory: []core.MemoryReference{}, // Empty memory configuration
			},
		}

		execID, _ := core.NewID()
		workflowContext := map[string]any{
			"workflow": map[string]any{
				"id":      "test-workflow",
				"exec_id": execID.String(),
				"input":   map[string]any{"user_id": "test_user"},
			},
		}

		memoryResolver := uc.NewMemoryResolver(
			env.memoryManager,
			env.tplEngine,
			workflowContext,
		)

		// Should not error with empty memory configuration
		memories, err := memoryResolver.ResolveAgentMemories(ctx, agentNoMemory)
		assert.NoError(t, err, "Should handle empty memory configuration without error")
		assert.Empty(t, memories, "Should return empty map for agent with no memory")
	})
}

// TestMemoryKeyTemplateResolution verifies that memory key templates are properly resolved
func TestMemoryKeyTemplateResolution(t *testing.T) {
	// Setup test environment
	env := NewTestEnvironment(t)
	defer env.Cleanup()
	ctx := env.ctx

	testCases := []struct {
		name          string
		keyTemplate   string
		workflowInput map[string]any
		expectedKey   string
		shouldError   bool
	}{
		{
			name:        "Should resolve simple template",
			keyTemplate: "user:{{.workflow.input.user_id}}",
			workflowInput: map[string]any{
				"user_id": "john_doe",
			},
			expectedKey: "user:john_doe",
			shouldError: false,
		},
		{
			name:        "Should resolve complex template",
			keyTemplate: "session:{{.workflow.input.app_id}}:{{.workflow.input.session_id}}",
			workflowInput: map[string]any{
				"app_id":     "myapp",
				"session_id": "12345",
			},
			expectedKey: "session:myapp:12345",
			shouldError: false,
		},
		{
			name:        "Should handle missing template variable",
			keyTemplate: "user:{{.workflow.input.missing_field}}",
			workflowInput: map[string]any{
				"user_id": "john_doe",
			},
			expectedKey: "",
			shouldError: true,
		},
		{
			name:        "Should handle literal key without template",
			keyTemplate: "static_key_123",
			workflowInput: map[string]any{
				"user_id": "john_doe",
			},
			expectedKey: "static_key_123",
			shouldError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Build workflow context
			execID, _ := core.NewID()
			workflowContext := map[string]any{
				"workflow": map[string]any{
					"id":      "test-workflow",
					"exec_id": execID.String(),
					"input":   tc.workflowInput,
				},
			}

			// Create memory reference
			memRef := core.MemoryReference{
				ID:   "customer-support", // Use existing registered memory config
				Key:  tc.keyTemplate,
				Mode: "read-write",
			}

			// Get memory instance
			instance, err := env.memoryManager.GetInstance(ctx, memRef, workflowContext)

			if tc.shouldError {
				assert.Error(t, err, "Should error for template: %s", tc.keyTemplate)
			} else {
				assert.NoError(t, err, "Should not error for template: %s", tc.keyTemplate)
				assert.NotNil(t, instance, "Instance should not be nil")

				// Verify the resolved key
				if instance != nil {
					assert.Equal(t, tc.expectedKey, instance.GetID(),
						"Memory key should be resolved correctly")
				}
			}
		})
	}
}

// TestMemoryRetryLogic verifies that the retry logic works for transient failures
func TestMemoryRetryLogic(t *testing.T) {
	// Setup test environment
	env := NewTestEnvironment(t)
	defer env.Cleanup()
	ctx := env.ctx

	t.Run("Should retry on empty key template", func(t *testing.T) {
		// Create memory reference with empty key (simulating the error condition)
		memRef := core.MemoryReference{
			ID:          "test_memory",
			Key:         "", // Empty key to trigger retry logic
			ResolvedKey: "",
		}

		execID, _ := core.NewID()
		workflowContext := map[string]any{
			"workflow": map[string]any{
				"id":      "test-workflow",
				"exec_id": execID.String(),
				"input":   map[string]any{"user_id": "test_user"},
			},
		}

		// Start timer to verify retry attempts
		start := time.Now()

		// Should fail after retries
		_, err := env.memoryManager.GetInstance(ctx, memRef, workflowContext)

		// Verify error occurred
		assert.Error(t, err, "Should error with empty key template")
		assert.Contains(t, err.Error(), "memory key template is empty")

		// Verify retries happened (should take at least 300ms due to backoff)
		duration := time.Since(start)
		assert.GreaterOrEqual(t, duration, 300*time.Millisecond,
			"Should have retry delays totaling at least 300ms")
	})
}
