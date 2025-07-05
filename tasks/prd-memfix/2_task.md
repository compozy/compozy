---
status: completed # Options: pending, in-progress, completed, excluded
---

<task_context>
<domain>engine/task2</domain>
<type>testing</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>task_1.0</dependencies>
</task_context>

# Task 2.0: Factory Unit Testing

## Overview

Create comprehensive unit tests for the memory task factory methods to ensure they properly return normalizer and response handler instances and handle edge cases correctly.

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

- [x] 2.1 Create unit test for CreateNormalizer with memory task type ✅
- [x] 2.2 Create unit test for CreateResponseHandler with memory task type ✅
- [x] 2.3 Verify memory task type is included in factory validation tests ✅
- [x] 2.4 Ensure all factory tests pass with new memory task support ✅
- [x] 2.5 Follow project testing standards with proper test naming and structure ✅

## Implementation Details

### Test Structure

Following the project testing standards, all tests must use the `t.Run("Should...")` pattern and testify assertions.

**File: `engine/task2/factory_test.go`**

Add the following test cases:

```go
func TestCreateNormalizer_Memory(t *testing.T) {
    t.Run("Should create normalizer for memory task type", func(t *testing.T) {
        // Arrange
        factory := createTestFactory(t)

        // Act
        normalizer, err := factory.CreateNormalizer(task.TaskTypeMemory)

        // Assert
        assert.NoError(t, err)
        assert.NotNil(t, normalizer)
        assert.IsType(t, &basic.Normalizer{}, normalizer)
    })

    t.Run("Should normalize memory task configuration", func(t *testing.T) {
        // Arrange
        factory := createTestFactory(t)
        normalizer, _ := factory.CreateNormalizer(task.TaskTypeMemory)

        config := &task.Config{
            Type:        task.TaskTypeMemory,
            Operation:   task.MemoryOpWrite,
            MemoryRef:   "test_memory",
            KeyTemplate: "test:{{.id}}",
        }
        ctx := createTestNormalizationContext()

        // Act
        err := normalizer.Normalize(config, ctx)

        // Assert
        assert.NoError(t, err)
    })
}

func TestCreateResponseHandler_Memory(t *testing.T) {
    t.Run("Should create response handler for memory task type", func(t *testing.T) {
        // Arrange
        factory := createTestFactory(t)

        // Act
        handler, err := factory.CreateResponseHandler(task.TaskTypeMemory)

        // Assert
        assert.NoError(t, err)
        assert.NotNil(t, handler)
        assert.Equal(t, task.TaskTypeMemory, handler.Type())
    })

    t.Run("Should handle memory task response", func(t *testing.T) {
        // Arrange
        factory := createTestFactory(t)
        handler, _ := factory.CreateResponseHandler(task.TaskTypeMemory)

        input := &shared.ResponseInput{
            TaskConfig: &task.Config{
                Type: task.TaskTypeMemory,
            },
            TaskState: &task.State{
                Status: task.StatusCompleted,
                Output: &core.Output{
                    "success": true,
                    "key":     "test:123",
                },
            },
        }

        // Act
        result, err := handler.HandleResponse(context.Background(), input)

        // Assert
        assert.NoError(t, err)
        assert.NotNil(t, result)
        assert.NotNil(t, result.Response)
    })
}
```

### Integration with Existing Tests

Update existing factory validation tests to include memory task type:

```go
func TestFactory_SupportedTaskTypes(t *testing.T) {
    t.Run("Should support all defined task types", func(t *testing.T) {
        supportedTypes := []task.Type{
            task.TaskTypeBasic,
            task.TaskTypeParallel,
            task.TaskTypeCollection,
            task.TaskTypeRouter,
            task.TaskTypeWait,
            task.TaskTypeAggregate,
            task.TaskTypeComposite,
            task.TaskTypeSignal,
            task.TaskTypeMemory, // Add this
        }

        factory := createTestFactory(t)

        for _, taskType := range supportedTypes {
            t.Run(string(taskType), func(t *testing.T) {
                normalizer, err := factory.CreateNormalizer(taskType)
                assert.NoError(t, err)
                assert.NotNil(t, normalizer)

                handler, err := factory.CreateResponseHandler(taskType)
                assert.NoError(t, err)
                assert.NotNil(t, handler)
            })
        }
    })
}
```

### Relevant Files

> Files that this task will modify:

- `engine/task2/factory_test.go` - Main test file to update

### Dependent Files

> Files that must be checked for test compatibility:

- `engine/task2/factory.go` - Implementation being tested
- `engine/task2/shared/handler_factory_concurrent_test.go` - Concurrent tests that include memory type
- `engine/task/config.go` - Task type definitions

## Success Criteria

- [x] Unit tests cover both CreateNormalizer and CreateResponseHandler for memory tasks ✅
- [x] Tests follow project standards with `t.Run("Should...")` pattern ✅
- [x] All new tests pass successfully ✅
- [x] No regression in existing factory tests ✅
- [x] Tests verify error handling and edge cases ✅
- [x] Code coverage maintained or improved ✅
- [x] Tests pass `make test` without failures ✅
