---
status: completed # Options: pending, in-progress, completed, excluded
---

<task_context>
<domain>engine/task/activities</domain>
<type>testing</type>
<scope>integration</scope>
<complexity>medium</complexity>
<dependencies>task_1.0,task_2.0</dependencies>
</task_context>

# Task 3.0: Memory Task Integration Testing

## Overview

Create comprehensive integration tests to verify memory task execution works end-to-end, including all memory operations (read, write, append, delete, flush, health, clear, stats) and error handling scenarios.

<critical>
**MANDATORY REQUIREMENTS:**
- **ALWAYS** check dependent files APIs before write tests to avoid write wrong code
- **ALWAYS** verify against PRD and tech specs - NEVER make assumptions
- **NEVER** use workarounds, especially in tests - implement proper solutions
- **MUST** follow all established project standards:
    - Architecture patterns: `.cursor/rules/architecture.mdc`
    - Go coding standards: `.cursor/rules/go-coding-standards.mdc`
    - Testing requirements: `.cursor/rules/testing-standards.mdc`
    - API standards: `.cursor/rules/api-standards.mdc`
    - Security & quality: `.cursor/rules/quality-security.mdc`
- **MUST** run `make lint` and `make test` before completing ANY subtask
- **MUST** follow `.cursor/rules/task-review.mdc` workflow for parent tasks
**Enforcement:** Violating these standards results in immediate task rejection.
</critical>

## Subtasks

- [ ] 3.1 Create ExecuteMemory activity integration test for basic operations
- [ ] 3.2 Implement tests for all 8 memory operations (read, write, append, delete, flush, health, clear, stats)
- [ ] 3.3 Add error handling tests for missing configuration and invalid operations
- [ ] 3.4 Create workflow-level memory task integration tests
- [ ] 3.5 Verify memory persistence and state management across operations

## Implementation Details

### Activity Integration Tests

**File: `engine/task/activities/exec_memory_test.go`**

Create comprehensive tests for the ExecuteMemory activity:

```go
package activities

import (
    "context"
    "testing"

    "github.com/compozy/compozy/engine/core"
    "github.com/compozy/compozy/engine/task"
    "github.com/compozy/compozy/engine/task2"
    "github.com/compozy/compozy/engine/memory"
    "github.com/compozy/compozy/pkg/tplengine"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestExecuteMemory_BasicOperations(t *testing.T) {
    t.Run("Should execute memory write operation", func(t *testing.T) {
        // Arrange
        ctx := context.Background()
        activity := createTestMemoryActivity(t)

        input := &ExecuteMemoryInput{
            WorkflowID:     "test-workflow",
            WorkflowExecID: core.MustNewID(),
            TaskConfig: &task.Config{
                Type:        task.TaskTypeMemory,
                Operation:   task.MemoryOpWrite,
                MemoryRef:   "test_memory",
                KeyTemplate: "test:{{.workflow.id}}",
                Payload: map[string]any{
                    "message": "Hello, Memory!",
                    "timestamp": "2024-01-01T00:00:00Z",
                },
            },
            MergedInput: &core.Input{
                "user_id": "test-user",
            },
        }

        // Act
        response, err := activity.Run(ctx, input)

        // Assert
        assert.NoError(t, err)
        assert.NotNil(t, response)
        assert.Equal(t, task.StatusCompleted, response.Status)
        assert.NotNil(t, response.Output)
        assert.Equal(t, true, response.Output["success"])
    })

    t.Run("Should execute memory read operation", func(t *testing.T) {
        // First write some data
        writeInput := createWriteInput()
        activity := createTestMemoryActivity(t)
        _, err := activity.Run(context.Background(), writeInput)
        require.NoError(t, err)

        // Now read it back
        readInput := &ExecuteMemoryInput{
            WorkflowID:     writeInput.WorkflowID,
            WorkflowExecID: writeInput.WorkflowExecID,
            TaskConfig: &task.Config{
                Type:        task.TaskTypeMemory,
                Operation:   task.MemoryOpRead,
                MemoryRef:   "test_memory",
                KeyTemplate: "test:{{.workflow.id}}",
            },
        }

        // Act
        response, err := activity.Run(context.Background(), readInput)

        // Assert
        assert.NoError(t, err)
        assert.NotNil(t, response)
        assert.NotNil(t, response.Output["messages"])
        messages := response.Output["messages"].([]any)
        assert.Len(t, messages, 1)
    })
}

func TestExecuteMemory_AllOperations(t *testing.T) {
    operations := []struct {
        name      string
        operation task.MemoryOpType
        config    map[string]any
    }{
        {
            name:      "append",
            operation: task.MemoryOpAppend,
            config: map[string]any{
                "payload": map[string]any{"message": "appended message"},
            },
        },
        {
            name:      "delete",
            operation: task.MemoryOpDelete,
        },
        {
            name:      "flush",
            operation: task.MemoryOpFlush,
            config: map[string]any{
                "flush_config": &task.FlushConfig{
                    DryRun: true,
                    Force:  false,
                },
            },
        },
        {
            name:      "health",
            operation: task.MemoryOpHealth,
            config: map[string]any{
                "health_config": &task.HealthConfig{
                    IncludeStats: true,
                },
            },
        },
        {
            name:      "clear",
            operation: task.MemoryOpClear,
            config: map[string]any{
                "clear_config": &task.ClearConfig{
                    Confirm: true,
                    Backup:  false,
                },
            },
        },
        {
            name:      "stats",
            operation: task.MemoryOpStats,
            config: map[string]any{
                "stats_config": &task.StatsConfig{
                    IncludeContent: true,
                },
            },
        },
    }

    for _, tc := range operations {
        t.Run("Should execute "+tc.name+" operation", func(t *testing.T) {
            // Arrange
            activity := createTestMemoryActivity(t)
            config := &task.Config{
                Type:        task.TaskTypeMemory,
                Operation:   tc.operation,
                MemoryRef:   "test_memory",
                KeyTemplate: "test:{{.workflow.id}}",
            }

            // Add operation-specific config
            for k, v := range tc.config {
                switch k {
                case "payload":
                    config.Payload = v
                case "flush_config":
                    config.FlushConfig = v.(*task.FlushConfig)
                case "health_config":
                    config.HealthConfig = v.(*task.HealthConfig)
                case "clear_config":
                    config.ClearConfig = v.(*task.ClearConfig)
                case "stats_config":
                    config.StatsConfig = v.(*task.StatsConfig)
                }
            }

            input := &ExecuteMemoryInput{
                WorkflowID:     "test-workflow",
                WorkflowExecID: core.MustNewID(),
                TaskConfig:     config,
            }

            // Act
            response, err := activity.Run(context.Background(), input)

            // Assert
            assert.NoError(t, err)
            assert.NotNil(t, response)
            assert.Equal(t, task.StatusCompleted, response.Status)
        })
    }
}

func TestExecuteMemory_ErrorHandling(t *testing.T) {
    t.Run("Should fail with missing memory_ref", func(t *testing.T) {
        // Arrange
        activity := createTestMemoryActivity(t)
        input := &ExecuteMemoryInput{
            TaskConfig: &task.Config{
                Type:        task.TaskTypeMemory,
                Operation:   task.MemoryOpWrite,
                // memory_ref missing
                KeyTemplate: "test:key",
            },
        }

        // Act
        response, err := activity.Run(context.Background(), input)

        // Assert
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "memory_ref")
    })

    t.Run("Should fail with missing key_template", func(t *testing.T) {
        // Arrange
        activity := createTestMemoryActivity(t)
        input := &ExecuteMemoryInput{
            TaskConfig: &task.Config{
                Type:      task.TaskTypeMemory,
                Operation: task.MemoryOpWrite,
                MemoryRef: "test_memory",
                // key_template missing
            },
        }

        // Act
        response, err := activity.Run(context.Background(), input)

        // Assert
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "key_template")
    })

    t.Run("Should fail with invalid operation", func(t *testing.T) {
        // Arrange
        activity := createTestMemoryActivity(t)
        input := &ExecuteMemoryInput{
            TaskConfig: &task.Config{
                Type:        task.TaskTypeMemory,
                Operation:   "invalid_operation",
                MemoryRef:   "test_memory",
                KeyTemplate: "test:key",
            },
        }

        // Act
        response, err := activity.Run(context.Background(), input)

        // Assert
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "unsupported memory operation")
    })
}
```

### Workflow Integration Tests

**File: `test/integration/memory/memory_workflow_test.go`**

```go
package memory

import (
    "testing"
    "time"

    "github.com/compozy/compozy/engine/worker"
    "github.com/compozy/compozy/test/helpers"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestMemoryWorkflow_EndToEnd(t *testing.T) {
    t.Run("Should execute complete memory workflow", func(t *testing.T) {
        // Arrange
        testEnv := helpers.NewTestEnvironment(t)
        defer testEnv.Cleanup()

        workflow := &workflow.Config{
            Name: "test-memory-workflow",
            Tasks: []task.Config{
                {
                    ID:          "write_memory",
                    Type:        task.TaskTypeMemory,
                    Operation:   task.MemoryOpWrite,
                    MemoryRef:   "conversation",
                    KeyTemplate: "conv:{{.workflow.id}}",
                    Payload: map[string]any{
                        "role":    "user",
                        "content": "Hello AI",
                    },
                },
                {
                    ID:          "read_memory",
                    Type:        task.TaskTypeMemory,
                    Operation:   task.MemoryOpRead,
                    MemoryRef:   "conversation",
                    KeyTemplate: "conv:{{.workflow.id}}",
                },
            },
        }

        // Act
        result, err := testEnv.ExecuteWorkflow(workflow)

        // Assert
        require.NoError(t, err)
        assert.Equal(t, workflow.StatusCompleted, result.Status)

        // Verify write task output
        writeOutput := result.Tasks["write_memory"].Output
        assert.Equal(t, true, writeOutput["success"])

        // Verify read task output
        readOutput := result.Tasks["read_memory"].Output
        assert.NotNil(t, readOutput["messages"])
        messages := readOutput["messages"].([]any)
        assert.Len(t, messages, 1)
    })
}
```

### Relevant Files

> Files that this task will create/modify:

- `engine/task/activities/exec_memory_test.go` - Activity-level tests
- `test/integration/memory/memory_workflow_test.go` - Workflow-level tests

### Dependent Files

> Files that must be checked for test dependencies:

- `engine/task/activities/exec_memory.go` - Implementation being tested
- `engine/task/uc/exec_memory_operation.go` - Use case being tested
- `engine/memory/service/operations.go` - Memory service operations
- `test/helpers/` - Test helper utilities

## Success Criteria

- [ ] All 8 memory operations have integration tests
- [ ] Error handling scenarios are thoroughly tested
- [ ] Tests verify end-to-end execution from workflow to memory storage
- [ ] Memory persistence is validated across operations
- [ ] Tests follow project standards with proper naming and structure
- [ ] All tests pass with `make test`
- [ ] No race conditions in concurrent test execution
- [ ] Code coverage for memory execution paths > 80%
