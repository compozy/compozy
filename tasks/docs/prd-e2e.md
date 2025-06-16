# PRD: Worker Integration Tests

## Introduction/Overview

This document outlines the requirements for creating a comprehensive integration test suite for Compozy's worker component. The worker is the core execution engine that orchestrates workflows using Temporal.io, manages state in PostgreSQL, and caches configurations in Redis. The test suite will ensure reliability and maintainability as new features are added, providing confidence that changes don't break existing functionality.

## Goals

1. Create a fast, reliable integration test suite that validates all core task types and their execution flows
2. Ensure database state synchronization is correctly maintained throughout workflow execution phases
3. Validate the integration between worker, database, and cache components
4. Establish a foundation for continuous testing that runs quickly in CI/CD pipelines
5. Use YAML fixtures exclusively for test data to maintain consistency with production usage

## User Stories

1. As a developer, I want to run integration tests locally in under 30 seconds so that I can get rapid feedback during development
2. As a developer, I want tests to use YAML fixtures so that test scenarios mirror real-world workflow definitions
3. As a QA engineer, I want to verify that each task type correctly updates its state in the database at each execution phase
4. As a team lead, I want comprehensive coverage of happy paths for all task types to ensure system reliability
5. As a developer, I want test infrastructure that can easily switch between TestContainers and docker-compose for flexibility

## Functional Requirements

### Test Coverage

1. **Basic Task Tests**

    - Test successful execution with simple action
    - Verify state transitions: pending → running → completed
    - Validate input/output persistence in database
    - Assert task response contains correct next task reference

2. **Collection Task Tests**

    - Test iteration over multiple items with both sequential and parallel modes
    - Verify child task creation for each collection item
    - Validate parent state aggregation after all children complete
    - Assert correct item context passing to each child
    - Test empty collection handling

3. **Parallel Task Tests**

    - Test concurrent execution of multiple child tasks
    - Verify all children start simultaneously
    - Validate parent status updates with different strategies (fail_fast, wait_all)
    - Assert final response aggregates all child outputs
    - Test partial failure scenarios

4. **Composite Task Tests**

    - Test sequential execution of child tasks
    - Verify tasks execute in defined order
    - Validate state passing between sequential tasks
    - Assert parent completes only after all children
    - Test early termination on child failure

5. **Router Task Tests**

    - Test condition evaluation and route selection
    - Verify only selected route executes
    - Validate different condition types (simple, complex expressions)
    - Assert unmatched condition handling
    - Test multiple matching routes

6. **Aggregate Task Tests**
    - Test output transformation from previous tasks
    - Verify no external action execution
    - Validate complex data aggregation patterns
    - Assert proper error handling for missing dependencies

### State Synchronization Requirements

7. **Database State Verification**

    - Every test must assert database state after each major operation
    - Verify task states include: status, started_at, completed_at, inputs, outputs, error
    - Validate parent-child relationships are correctly established
    - Assert workflow state tracks overall execution progress

8. **Cache Integration**
    - Verify task configurations are stored in Redis with correct keys
    - Test configuration retrieval during execution
    - Validate TTL settings on cached data
    - Assert graceful handling of cache misses

### Test Infrastructure

9. **YAML Fixture System**

    - All tests must use YAML files for workflow/task definitions
    - Fixtures should be organized by task type
    - Support for parameterized test data within YAML
    - Validation of YAML structure before test execution

10. **Test Helpers**

    - Database state assertion helpers
    - Temporal workflow execution helpers
    - Redis state inspection utilities
    - YAML fixture loading and validation
    - Test data cleanup utilities

11. **Performance Requirements**
    - Individual test execution < 500ms
    - Full suite execution < 30s
    - Parallel test execution support
    - Minimal database setup/teardown overhead

## Non-Goals (Out of Scope)

1. Testing signal workflows and cross-workflow communication
2. Complex failure scenarios (network partitions, cascading failures)
3. Edge cases like workflow replay, version compatibility
4. Load testing or performance benchmarking
5. Testing with real external services or APIs
6. Backward compatibility testing for fixtures
7. Testing manual intervention scenarios

## Design Considerations

### Test Organization

```
test/integration/worker/
├── fixtures/
│   ├── basic/
│   ├── collection/
│   ├── parallel/
│   ├── composite/
│   ├── router/
│   └── aggregate/
├── helpers/
│   ├── temporal.go      # Temporal test harness
│   ├── database.go      # DB assertions and setup
│   ├── redis.go         # Cache inspection
│   └── fixtures.go      # YAML loading
├── basic_test.go
├── collection_test.go
├── parallel_test.go
├── composite_test.go
├── router_test.go
└── aggregate_test.go
```

### YAML Fixture Format

```yaml
name: test-basic-happy-path
workflow:
    name: basic-workflow
    project: test-project
    tasks:
        - name: simple-task
          type: basic
          action:
              type: script
              config:
                  script: "return { result: 'success' }"

expected:
    task_states:
        - name: simple-task
          status: completed
          outputs:
              result: success
    workflow_state:
        status: completed
        total_tasks: 1
        completed_tasks: 1
```

## Technical Considerations

1. **Temporal Test Environment**

    - Use Temporal's in-memory test suite for speed
    - Mock time advancement for timeout testing
    - Capture workflow histories for debugging

2. **Database Isolation**

    - Use TestContainers with PostgreSQL for true integration
    - Implement transaction-based cleanup for speed
    - Consider connection pooling for parallel tests

3. **Redis Testing**

    - Use TestContainers Redis or miniredis
    - Implement key namespace isolation per test
    - Clear cache between test runs

4. **Parallelization Strategy**
    - Run tests in parallel with isolated databases
    - Use unique workflow IDs to prevent conflicts
    - Implement resource pools for container reuse

## Success Metrics

1. All core task types have >90% code coverage
2. Test suite executes in <30 seconds locally
3. Zero flaky tests after 100 consecutive runs
4. New features can add tests using existing helpers without framework changes
5. Test failures provide clear, actionable error messages
6. CI pipeline runs tests on every PR in <2 minutes

## Open Questions

1. Should we implement snapshot testing for complex state assertions?
2. Do we need a separate test mode for debugging with verbose logging?
3. Should test fixtures support environment variable substitution?
4. How should we handle test data generation for large collections?
5. Should we create a test report dashboard for tracking test stability over time?
