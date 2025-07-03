---
status: pending
---

<task_context>
<domain>engine/task2/validation</domain>
<type>testing</type>
<scope>validation</scope>
<complexity>high</complexity>
<dependencies>comprehensive_testing_suite,legacy_services</dependencies>
</task_context>

# Task 8.0: Behavior Validation & Golden Master Tests

## Overview

Validate that the new modular components produce identical behavior to the existing TaskResponder and ConfigManager services. This critical validation phase ensures zero regressions before replacing the legacy services in production code.

## Subtasks

- [ ] 8.1 Golden master test suite captures all current behaviors
- [ ] 8.2 New components produce identical outputs for all test scenarios
- [ ] 8.3 Edge cases and error conditions validated
- [ ] 8.5 Memory usage patterns analyzed and approved
- [ ] 8.6 Concurrency behavior validated
- [ ] 8.7 Template processing separation maintained
- [ ] 8.8 100% behavior parity achieved with legacy services

## Implementation Details

### Template Processing Separation Validation

**Ensure Proper Separation of Concerns:**

```go
func TestTemplateBoundaries_ConfigVsOutput(t *testing.T) {
    t.Run("Should maintain template processing separation for basic tasks", func(t *testing.T) {
        // Arrange
        config := &task.Config{Type: task.TaskTypeBasic}
        handler := setupResponseHandler(task.TaskTypeBasic)

        // Act & Assert
        shouldDefer := handler.shouldDeferOutputTransformation(config)

        // Basic tasks should process outputs immediately
        assert.False(t, shouldDefer, "Basic tasks must process outputs immediately")
    })

    t.Run("Should defer output transformation for collection tasks", func(t *testing.T) {
        // Arrange
        config := &task.Config{Type: task.TaskTypeCollection}
        handler := setupResponseHandler(task.TaskTypeCollection)

        // Act & Assert
        shouldDefer := handler.shouldDeferOutputTransformation(config)

        // Collection tasks should defer output processing
        assert.True(t, shouldDefer, "Collection tasks must defer output transformation")
    })

    t.Run("Should not process outputs during config creation", func(t *testing.T) {
        // Arrange
        expander := setupCollectionExpander()
        config := &task.Config{
            Type: task.TaskTypeCollection,
            CollectionConfig: task.CollectionConfig{
                Items: []interface{}{"a", "b"},
            },
        }

        // Act
        result, err := expander.ExpandItems(context.Background(), config, &workflow.State{}, &workflow.Config{})

        // Assert
        require.NoError(t, err)

        // CRITICAL: Child configs should have templates, not processed outputs
        for i, childConfig := range result.ChildConfigs {
            assert.Nil(t, childConfig.Output, "CollectionExpander must not process outputs at index %d", i)
            assert.NotNil(t, childConfig.OutputTemplate, "Child should have output template at index %d", i)
        }
    })

    t.Run("Should defer parallel task output transformation", func(t *testing.T) {
        // Arrange
        config := &task.Config{Type: task.TaskTypeParallel}
        handler := setupResponseHandler(task.TaskTypeParallel)

        // Act & Assert
        shouldDefer := handler.shouldDeferOutputTransformation(config)

        // Parallel tasks should defer output processing
        assert.True(t, shouldDefer, "Parallel tasks must defer output transformation")
    })

    t.Run("Should process composite task outputs immediately", func(t *testing.T) {
        // Arrange
        config := &task.Config{Type: task.TaskTypeComposite}
        handler := setupResponseHandler(task.TaskTypeComposite)

        // Act & Assert
        shouldDefer := handler.shouldDeferOutputTransformation(config)

        // Composite tasks should process outputs immediately
        assert.False(t, shouldDefer, "Composite tasks must process outputs immediately")
    })
}
```

### Test Data Creation

**Comprehensive Scenario Capture:**

1. `testdata/task_responder_scenarios.json` - All TaskResponder test cases
2. `testdata/config_manager_scenarios.json` - All ConfigManager test cases
3. `testdata/edge_cases.json` - Edge cases and boundary conditions
4. `testdata/error_scenarios.json` - Error conditions and failure modes

### Golden Master Test Implementation

**TaskResponder Behavior Validation:**

```go
func TestGoldenMaster_TaskResponder_Complete(t *testing.T) {
    t.Run("Should match legacy behavior for basic task handling", func(t *testing.T) {
        // Arrange
        scenarios := loadTaskResponderScenarios("testdata/basic_task_scenarios.json")
        legacyResponder := setupLegacyTaskResponder(t)
        factory := setupFactory(t)

        for _, scenario := range scenarios {
            // Run legacy implementation
            legacyResult, legacyErr := runLegacyTaskResponder(legacyResponder, scenario)

            // Run new implementation
            handler, err := factory.CreateResponseHandler(task.TaskTypeBasic)
            require.NoError(t, err)
            newResult, newErr := runNewResponseHandler(handler, scenario)

            // Validate identical behavior
            validateIdenticalBehavior(t, scenario.Name, legacyResult, legacyErr, newResult, newErr)
        }
    })

    t.Run("Should match legacy behavior for collection task handling", func(t *testing.T) {
        // Arrange
        scenarios := loadTaskResponderScenarios("testdata/collection_task_scenarios.json")
        legacyResponder := setupLegacyTaskResponder(t)
        factory := setupFactory(t)

        for _, scenario := range scenarios {
            // Run legacy implementation
            legacyResult, legacyErr := runLegacyTaskResponder(legacyResponder, scenario)

            // Run new implementation
            handler, err := factory.CreateResponseHandler(task.TaskTypeCollection)
            require.NoError(t, err)
            newResult, newErr := runNewResponseHandler(handler, scenario)

            // Validate identical behavior including deferred transformation
            validateIdenticalBehavior(t, scenario.Name, legacyResult, legacyErr, newResult, newErr)

            // Validate collection-specific behavior
            if legacyErr == nil && newErr == nil {
                validateCollectionSpecificBehavior(t, legacyResult, newResult, scenario.Name)
            }
        }
    })

    t.Run("Should match legacy behavior for parallel task handling", func(t *testing.T) {
        // Arrange
        scenarios := loadTaskResponderScenarios("testdata/parallel_task_scenarios.json")
        legacyResponder := setupLegacyTaskResponder(t)
        factory := setupFactory(t)

        for _, scenario := range scenarios {
            // Run legacy implementation
            legacyResult, legacyErr := runLegacyTaskResponder(legacyResponder, scenario)

            // Run new implementation
            handler, err := factory.CreateResponseHandler(task.TaskTypeParallel)
            require.NoError(t, err)
            newResult, newErr := runNewResponseHandler(handler, scenario)

            // Validate identical behavior including strategy handling
            validateIdenticalBehavior(t, scenario.Name, legacyResult, legacyErr, newResult, newErr)

            // Validate parallel-specific behavior
            if legacyErr == nil && newErr == nil {
                validateParallelSpecificBehavior(t, legacyResult, newResult, scenario.Name)
            }
        }
    })
}
```

**ConfigManager Behavior Validation:**

```go
func TestGoldenMaster_ConfigManager_Complete(t *testing.T) {
    scenarios := loadConfigManagerScenarios("testdata/config_manager_scenarios.json")

    // Setup legacy ConfigManager
    legacyManager := setupLegacyConfigManager(t)

    // Setup new components
    factory := setupFactory(t)
    expander := factory.CreateCollectionExpander()
    repository := factory.CreateTaskConfigRepository(configStore)

    for _, scenario := range scenarios {
        t.Run(scenario.Name, func(t *testing.T) {
            switch scenario.Operation {
            case "PrepareCollectionConfigs":
                validateCollectionExpansion(t, legacyManager, expander, scenario)
            case "PrepareParallelConfigs":
                validateParallelStorage(t, legacyManager, repository, scenario)
            case "PrepareCompositeConfigs":
                validateCompositeStorage(t, legacyManager, repository, scenario)
            default:
                t.Errorf("Unknown operation: %s", scenario.Operation)
            }
        })
    }
}
```

### Scenario Generation Strategy

**Golden Master Test Targets:**

- **Minimum 50 scenarios per task type** (8 types × 50 = 400 scenarios minimum)
- **25 edge case scenarios** per component
- **10 concurrency scenarios** per handler
- **15 error condition scenarios** per task type

**Automated Scenario Capture:**

```go
func GenerateGoldenMasterScenarios(t *testing.T) {
    // Generate comprehensive test scenarios covering:

    // 1. All task types with normal operations (50+ scenarios each)
    taskTypes := []task.Type{
        task.TaskTypeBasic, task.TaskTypeParallel, task.TaskTypeCollection,
        task.TaskTypeComposite, task.TaskTypeRouter, task.TaskTypeWait,
        task.TaskTypeSignal, task.TaskTypeAggregate,
    }

    // 2. All success/failure combinations
    statusCombinations := []core.Status{
        core.StatusSuccess, core.StatusFailed, core.StatusRunning,
    }

    // 3. Various execution error scenarios (15+ per type)
    errorScenarios := []string{
        "nil", "timeout", "validation_error", "execution_failure",
    }

    // 4. Collection-specific scenarios
    collectionScenarios := []int{0, 1, 10, 100, 1000} // Item counts

    // 5. Parallel-specific scenarios
    parallelStrategies := []task.ParallelStrategy{
        task.StrategyWaitAll, task.StrategyFailFast, task.StrategyBestEffort,
    }

    // Generate all combinations and capture current behavior
    scenarios := generateAllCombinations(taskTypes, statusCombinations, errorScenarios, collectionScenarios, parallelStrategies)
    saveScenarios("testdata/comprehensive_scenarios.json", scenarios)
}
```

### Behavior Validation Functions

**Identical Output Validation:**

```go
func validateIdenticalBehavior(t *testing.T, scenarioName string, legacyResult, legacyErr interface{}, newResult, newErr interface{}) {
    // Error status must match
    assert.Equal(t, legacyErr != nil, newErr != nil,
        "Error status mismatch in scenario: %s", scenarioName)

    if legacyErr != nil && newErr != nil {
        // Error messages should be identical or equivalent
        validateErrorEquivalence(t, legacyErr, newErr, scenarioName)
        return
    }

    if legacyErr == nil && newErr == nil {
        // Success results must be identical
        validateResultEquivalence(t, legacyResult, newResult, scenarioName)
    }
}

func validateErrorEquivalence(t *testing.T, legacyErr, newErr error, scenario string) {
    // Extract error messages and compare
    legacyMsg := extractErrorMessage(legacyErr)
    newMsg := extractErrorMessage(newErr)

    assert.Equal(t, legacyMsg, newMsg,
        "Error message mismatch in scenario: %s", scenario)
}

func validateResultEquivalence(t *testing.T, legacyResult, newResult interface{}, scenario string) {
    // Deep comparison of result structures
    switch legacy := legacyResult.(type) {
    case *task.MainTaskResponse:
        new := newResult.(*task.MainTaskResponse)
        validateMainTaskResponse(t, legacy, new, scenario)
    case *task.CollectionResponse:
        new := newResult.(*task.CollectionResponse)
        validateCollectionResponse(t, legacy, new, scenario)
    case *services.CollectionMetadata:
        new := newResult.(*services.CollectionMetadata)
        validateCollectionMetadata(t, legacy, new, scenario)
    default:
        assert.Equal(t, legacyResult, newResult,
            "Result mismatch in scenario: %s", scenario)
    }
}
```

**Memory Usage Analysis:**

```go
func TestMemoryUsage_NewComponents_vs_Legacy(t *testing.T) {
    scenarios := loadMemoryTestScenarios()

    for _, scenario := range scenarios {
        t.Run(scenario.Name, func(t *testing.T) {
            // Measure legacy memory usage
            legacyMemory := measureMemoryUsage(func() {
                runLegacyImplementation(scenario)
            })

            // Measure new memory usage
            newMemory := measureMemoryUsage(func() {
                runNewImplementation(scenario)
            })

            // Validate no significant increase
            memoryIncrease := float64(newMemory-legacyMemory) / float64(legacyMemory) * 100
            assert.LessOrEqual(t, memoryIncrease, 10.0,
                "Memory usage increase in scenario %s: %.2f%%", scenario.Name, memoryIncrease)
        })
    }
}
```

### Edge Case Validation

**Boundary Conditions:**

```go
func TestEdgeCases_ComprehensiveCoverage(t *testing.T) {
    edgeCases := []struct {
        name        string
        description string
        setup       func() (legacy interface{}, new interface{})
        validate    func(t *testing.T, legacy, new interface{})
    }{
        {
            name:        "empty_collection_expansion",
            description: "Collection with zero items after filtering",
            setup:       setupEmptyCollectionScenario,
            validate:    validateEmptyCollectionBehavior,
        },
        {
            name:        "context_cancellation_during_save",
            description: "Context cancelled during state save operation",
            setup:       setupCancellationScenario,
            validate:    validateCancellationBehavior,
        },
        {
            name:        "large_collection_processing",
            description: "Collection with 10,000 items",
            setup:       setupLargeCollectionScenario,
            validate:    validateLargeCollectionBehavior,
        },
        // ... more edge cases
    }

    for _, tc := range edgeCases {
        t.Run(tc.name, func(t *testing.T) {
            legacy, new := tc.setup()
            tc.validate(t, legacy, new)
        })
    }
}
```

### Concurrency Validation

**Concurrent Access Patterns:**

```go
func TestConcurrency_BehaviorConsistency(t *testing.T) {
    scenarios := []struct {
        name         string
        concurrency  int
        operation    string
    }{
        {"parallel_response_handling", 10, "response_handling"},
        {"concurrent_collection_expansion", 5, "collection_expansion"},
        {"mixed_operations", 20, "mixed"},
    }

    for _, scenario := range scenarios {
        t.Run(scenario.name, func(t *testing.T) {
            // Run legacy implementation concurrently
            legacyResults := runConcurrentLegacy(scenario)

            // Run new implementation concurrently
            newResults := runConcurrentNew(scenario)

            // Validate results are equivalent
            validateConcurrentResults(t, legacyResults, newResults, scenario.name)
        })
    }
}
```

## Test Data Management

**Scenario File Structure:**

```json
{
  "scenarios": [
    {
      "name": "basic_task_success",
      "task_type": "basic",
      "input": {
        "workflow_config": {...},
        "task_state": {...},
        "task_config": {...},
        "execution_error": null
      },
      "expected_behavior": "success",
      "description": "Basic task with successful execution"
    }
  ]
}
```

## Dependencies

- Task 7: Comprehensive testing suite infrastructure
- All implemented components (Tasks 1-6)
- Legacy TaskResponder and ConfigManager for comparison
- Memory profiling utilities

## Validation Criteria

**Behavior Parity Requirements:**

- 100% identical outputs for all success scenarios
- 100% identical error behaviors for all failure scenarios
- Memory usage increase <10%
- No regressions in edge cases or concurrent scenarios

**Quality Gates:**

- All golden master tests must pass
- Memory usage analysis must be approved
- Edge case coverage must be complete
- Concurrency behavior must be validated

## Implementation Notes

- Generate comprehensive test scenarios automatically where possible
- Use deterministic inputs to ensure reproducible results
- Include timing-sensitive scenarios for race condition detection
- Validate exact JSON serialization compatibility
- Test context cancellation at various points

## Rollback Plan

If behavior parity cannot be achieved:

1. Document specific discrepancies
2. Determine if discrepancies are acceptable
3. If not acceptable, modify new implementation to match legacy
4. Re-run validation until 100% parity achieved

## Success Criteria

- All golden master tests pass with 100% behavior parity
- Memory usage validated and approved
- Edge cases and concurrency scenarios validated
- Documentation of any intentional behavior changes
- Code review approved
- Ready for production integration
- Zero regressions identified in validation testing

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

- [ ] Golden master scenarios generated (minimum 50 per task type)
- [ ] All TaskResponder behaviors captured in test data
- [ ] All ConfigManager behaviors captured in test data
- [ ] Edge case scenarios documented (25+ per component)
- [ ] Error scenarios comprehensive (15+ per task type)
- [ ] Behavior validation functions compare outputs exactly
- [ ] Memory usage analysis completed
- [ ] Concurrency scenarios validated (10+ per handler)
- [ ] 100% behavior parity achieved
- [ ] All golden master tests passing
