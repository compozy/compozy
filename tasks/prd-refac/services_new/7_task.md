---
status: pending
---

<task_context>
<domain>engine/task2/testing</domain>
<type>testing</type>
<scope>validation</scope>
<complexity>high</complexity>
<dependencies>all_new_components,testify_mock,benchmarking_tools,golden_master_framework</dependencies>
</task_context>

# Task 7.0: Comprehensive Testing Suite

## Overview

Create a comprehensive testing suite that validates all new components work correctly in isolation and integration. This includes unit tests, integration tests, and golden master tests to ensure behavioral parity with existing services.

## Subtasks

- [ ] 7.1 >70% test coverage for all new components
- [ ] 7.2 Unit tests for every public method and interface
- [ ] 7.3 Integration tests for component interactions
- [ ] 7.5 Golden master tests capturing current behavior
- [ ] 7.6 Error scenario coverage for all failure modes
- [ ] 7.7 Concurrent access and thread safety tests

## Implementation Details

### Test Files to Create/Enhance

**Unit Test Files:**

1. `engine/task2/shared/interfaces_test.go`
2. `engine/task2/shared/base_response_handler_test.go`
3. `engine/task2/shared/parent_status_manager_test.go`
4. `engine/task2/collection/expander_test.go`
5. `engine/task2/core/config_repo_test.go`
6. `engine/task2/*/response_handler_test.go` (8 files)
7. `engine/task2/factory_test.go`

**Integration Test Files:**

1. `engine/task2/integration/response_handlers_test.go`
2. `engine/task2/integration/collection_expander_test.go`
3. `engine/task2/integration/factory_integration_test.go`

**Golden Master Test Files:**

1. `engine/task2/golden/config_manager_golden_test.go`
2. `engine/task2/golden/task_responder_golden_test.go`

### Unit Testing Strategy

**BaseResponseHandler Tests:**

```go
func TestBaseResponseHandler_ProcessMainTaskResponse(t *testing.T) {
    t.Run("Should process successful task execution", func(t *testing.T) {
        // Arrange
        mockWorkflowRepo := new(MockWorkflowRepository)
        mockTaskRepo := new(MockTaskRepository)
        mockParentStatusManager := new(MockParentStatusManager)

        input := &ResponseInput{
            TaskConfig: &task.Config{Type: task.TaskTypeBasic},
            TaskState:  &task.State{Status: core.StatusRunning, TaskExecID: "test-id"},
            ExecutionError: nil,
        }

        mockTaskRepo.On("UpsertState", mock.Anything, mock.Anything).Return(nil)
        mockParentStatusManager.On("UpdateParentStatus", mock.Anything, mock.Anything, core.StatusSuccess).Return(nil)

        handler := NewBaseResponseHandler(mockTaskRepo, mockWorkflowRepo, mockParentStatusManager)

        // Act
        response, err := handler.ProcessMainTaskResponse(context.Background(), input)

        // Assert
        assert.NoError(t, err)
        assert.NotNil(t, response)
        assert.Equal(t, core.StatusSuccess, response.State.Status)
        mockTaskRepo.AssertExpectations(t)
        mockParentStatusManager.AssertExpectations(t)
    })

    t.Run("Should handle task execution failure", func(t *testing.T) {
        // Arrange
        mockTaskRepo := new(MockTaskRepository)
        mockParentStatusManager := new(MockParentStatusManager)

        input := &ResponseInput{
            TaskConfig: &task.Config{Type: task.TaskTypeBasic},
            TaskState:  &task.State{Status: core.StatusRunning, TaskExecID: "test-id"},
            ExecutionError: errors.New("execution failed"),
        }

        mockTaskRepo.On("UpsertState", mock.Anything, mock.Anything).Return(nil)
        mockParentStatusManager.On("UpdateParentStatus", mock.Anything, mock.Anything, core.StatusFailed).Return(nil)

        handler := NewBaseResponseHandler(mockTaskRepo, nil, mockParentStatusManager)

        // Act
        response, err := handler.ProcessMainTaskResponse(context.Background(), input)

        // Assert
        assert.NoError(t, err)
        assert.Equal(t, core.StatusFailed, response.State.Status)
        assert.Contains(t, response.State.Error, "execution failed")
        mockTaskRepo.AssertExpectations(t)
        mockParentStatusManager.AssertExpectations(t)
    })

    t.Run("Should handle context cancellation gracefully", func(t *testing.T) {
        // Arrange
        ctx, cancel := context.WithCancel(context.Background())
        cancel() // Cancel immediately

        mockTaskRepo := new(MockTaskRepository)
        mockTaskRepo.On("UpsertState", mock.Anything, mock.Anything).Return(context.Canceled)

        input := &ResponseInput{
            TaskState: &task.State{Status: core.StatusRunning, TaskExecID: "test-id"},
        }

        handler := NewBaseResponseHandler(mockTaskRepo, nil, nil)

        // Act
        response, err := handler.ProcessMainTaskResponse(ctx, input)

        // Assert
        assert.NoError(t, err) // Should not error on cancellation
        assert.Equal(t, input.TaskState, response.State) // Should return original state
        mockTaskRepo.AssertExpectations(t)
    })
}
```

**CollectionExpander Tests:**

```go
func TestCollectionExpander_ExpandItems(t *testing.T) {
    t.Run("Should expand basic collection items", func(t *testing.T) {
        // Arrange
        mockNormalizer := new(MockCollectionNormalizer)
        mockContextBuilder := new(MockContextBuilder)
        mockConfigBuilder := new(MockConfigBuilder)

        config := &task.Config{
            Type: task.TaskTypeCollection,
            CollectionConfig: task.CollectionConfig{
                Items: []interface{}{"a", "b", "c"},
            },
        }
        workflowState := &workflow.State{}
        workflowConfig := &workflow.Config{}

        expectedItems := []interface{}{"a", "b", "c"}
        mockNormalizer.On("ExpandCollectionItems", mock.Anything, config, mock.Anything).Return(expectedItems, nil)
        mockNormalizer.On("FilterCollectionItems", expectedItems, config).Return(expectedItems, 0, nil)
        mockConfigBuilder.On("BuildFromTemplate", mock.Anything, mock.Anything).Return(&task.Config{}, nil).Times(3)

        expander := NewExpander(mockNormalizer, mockContextBuilder, mockConfigBuilder)

        // Act
        result, err := expander.ExpandItems(context.Background(), config, workflowState, workflowConfig)

        // Assert
        assert.NoError(t, err)
        assert.NotNil(t, result)
        assert.Equal(t, 3, result.ItemCount)
        assert.Equal(t, 0, result.SkippedCount)
        assert.Len(t, result.ChildConfigs, 3)
        mockNormalizer.AssertExpectations(t)
        mockConfigBuilder.AssertExpectations(t)
    })

    t.Run("Should handle empty collection gracefully", func(t *testing.T) {
        // Arrange
        mockNormalizer := new(MockCollectionNormalizer)
        mockContextBuilder := new(MockContextBuilder)

        config := &task.Config{
            Type: task.TaskTypeCollection,
            CollectionConfig: task.CollectionConfig{
                Items: []interface{}{},
            },
        }

        mockNormalizer.On("ExpandCollectionItems", mock.Anything, config, mock.Anything).Return([]interface{}{}, nil)
        mockNormalizer.On("FilterCollectionItems", mock.Anything, config).Return([]interface{}{}, 0, nil)

        expander := NewExpander(mockNormalizer, mockContextBuilder, nil)

        // Act
        result, err := expander.ExpandItems(context.Background(), config, nil, nil)

        // Assert
        assert.NoError(t, err)
        assert.Equal(t, 0, result.ItemCount)
        assert.Equal(t, 0, result.SkippedCount)
        assert.Empty(t, result.ChildConfigs)
        mockNormalizer.AssertExpectations(t)
    })

    t.Run("Should inject collection context variables", func(t *testing.T) {
        // Arrange
        mockNormalizer := new(MockCollectionNormalizer)
        mockContextBuilder := new(MockContextBuilder)
        mockConfigBuilder := new(MockConfigBuilder)

        config := &task.Config{
            Type: task.TaskTypeCollection,
            CollectionConfig: task.CollectionConfig{
                Items: []interface{}{"item1"},
                ItemVar: "customItem",
                IndexVar: "customIndex",
            },
        }

        childConfig := &task.Config{With: &core.Input{}}
        expectedItems := []interface{}{"item1"}

        mockNormalizer.On("ExpandCollectionItems", mock.Anything, config, mock.Anything).Return(expectedItems, nil)
        mockNormalizer.On("FilterCollectionItems", expectedItems, config).Return(expectedItems, 0, nil)
        mockConfigBuilder.On("BuildFromTemplate", mock.Anything, mock.Anything).Return(childConfig, nil)

        expander := NewExpander(mockNormalizer, mockContextBuilder, mockConfigBuilder)

        // Act
        result, err := expander.ExpandItems(context.Background(), config, nil, nil)

        // Assert
        assert.NoError(t, err)
        require.Len(t, result.ChildConfigs, 1)

        childWith := result.ChildConfigs[0].With
        assert.Equal(t, "item1", (*childWith)["_collection_item"])
        assert.Equal(t, 0, (*childWith)["_collection_index"])
        assert.Equal(t, "customItem", (*childWith)["_collection_item_var"])
        assert.Equal(t, "customIndex", (*childWith)["_collection_index_var"])

        mockNormalizer.AssertExpectations(t)
        mockConfigBuilder.AssertExpectations(t)
    })
}
```

### Integration Testing Strategy

**Full Workflow Integration:**

```go
func TestResponseHandler_Integration_WithRealDependencies(t *testing.T) {
    // Setup real dependencies (not mocked)
    factory := setupRealFactory(t)
    handler, err := factory.CreateResponseHandler(task.TaskTypeCollection)
    require.NoError(t, err)

    // Test with real workflow and task states
    input := createRealResponseInput(t)
    response, err := handler.HandleResponse(ctx, input)

    // Validate response matches expected behavior
    assert.NoError(t, err)
    assert.Equal(t, expectedResponse, response)
}
```

**Factory Integration Tests:**

```go
func TestFactory_CreatesWorkingComponents(t *testing.T) {
    factory := task2.NewExtendedFactory(config)

    // Test each component creation and basic functionality
    for _, taskType := range allTaskTypes {
        handler, err := factory.CreateResponseHandler(taskType)
        require.NoError(t, err)
        assert.Equal(t, taskType, handler.Type())

        // Test basic functionality
        testBasicHandlerOperation(t, handler)
    }
}
```

### Golden Master Testing

**Behavior Capture and Comparison:**

```go
func TestGoldenMaster_ConfigManager_vs_CollectionExpander(t *testing.T) {
    testCases := loadTestCases("testdata/collection_scenarios.json")

    for _, tc := range testCases {
        t.Run(tc.Name, func(t *testing.T) {
            // Run old implementation
            oldResult, oldErr := runConfigManager(tc.Input)

            // Run new implementation
            newResult, newErr := runCollectionExpander(tc.Input)

            // Compare results
            assert.Equal(t, oldErr != nil, newErr != nil, "Error status should match")
            if oldErr == nil && newErr == nil {
                assert.Equal(t, oldResult, newResult, "Results should be identical")
            }
        })
    }
}
```

### Error Scenario Testing

**Comprehensive Error Coverage:**

```go
func TestErrorScenarios_AllComponents(t *testing.T) {
    errorScenarios := []struct {
        name        string
        component   string
        setupError  func() error
        expectError string
    }{
        {
            name:        "collection expansion with invalid template",
            component:   "CollectionExpander",
            setupError:  func() error { return errors.New("template error") },
            expectError: "failed to expand collection items",
        },
        // ... more error scenarios
    }

    for _, scenario := range errorScenarios {
        t.Run(scenario.name, func(t *testing.T) {
            // Test error handling
        })
    }
}
```

### Concurrent Access Testing

**Thread Safety Validation:**

```go
func TestConcurrentAccess_Factory(t *testing.T) {
    factory := task2.NewExtendedFactory(config)

    // Run concurrent handler creation
    var wg sync.WaitGroup
    errors := make(chan error, 100)

    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(taskType task.Type) {
            defer wg.Done()
            handler, err := factory.CreateResponseHandler(taskType)
            if err != nil {
                errors <- err
                return
            }
            // Test basic handler operation
            if err := testHandlerOperation(handler); err != nil {
                errors <- err
            }
        }(allTaskTypes[i%len(allTaskTypes)])
    }

    wg.Wait()
    close(errors)

    for err := range errors {
        t.Errorf("Concurrent access error: %v", err)
    }
}
```

## Dependencies

- Tasks 1-6: All implemented components
- Test data and fixtures
- Mocking framework (testify/mock)
- Benchmarking utilities

## Coverage Requirements

**Minimum Coverage Targets:**

- Unit tests: >70% line coverage
- Branch coverage: >85%
- Error path coverage: >80%
- Integration test coverage: >70%

**Coverage Validation:**

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
go tool cover -func=coverage.out | grep total
```

## Implementation Notes

- Use table-driven tests for comprehensive scenario coverage
- Mock external dependencies appropriately
- Include edge cases and boundary conditions
- Test context cancellation scenarios
- Validate memory leaks and resource cleanup

## Success Criteria

- All test files created and passing
- Coverage targets met for all components
- Golden master tests validate behavior parity
- Concurrent access tests pass
- Code review approved
- Ready for behavior validation phase
- Comprehensive test infrastructure provides confidence for deployment

<critical>
**MANDATORY REQUIREMENTS:**

- **ALWAYS** verify against PRD and tech specs - NEVER make assumptions
- **NEVER** use workarounds, especially in tests - implement proper solutions
- **MUST** follow all established project standards:
    - Architecture patterns: `.cursor/rules/architecture.mdc`
    - Go coding standards: `.cursor/rules/go-coding-standards.mdc`
    - Testing requirements: `.cursor/rules/testing-standards.mdc`
    - API standards: `.cursor/rules/api-standards.mdc`
    - Security & quality: `.cursor/rules/quality-security.mdc`
- **MUST** run `make lint` and `make test` before completing parent tasks
- **MUST** follow `.cursor/rules/task-review.mdc` workflow for parent tasks

**Enforcement:** Violating these standards results in immediate task rejection.
</critical>

## Validation Checklist

Before marking this task complete, verify:

- [ ] Unit test coverage >70% for all new components
- [ ] All public methods have corresponding tests
- [ ] Tests follow mandatory `t.Run("Should...")` pattern
- [ ] Integration tests verify component interactions
- [ ] Golden master test framework established
- [ ] Error scenarios comprehensively tested
- [ ] Concurrent access tests implemented
- [ ] Mock implementations use testify/mock
- [ ] No test suite patterns used (direct assertions only)
- [ ] All tests pass with `make test`
