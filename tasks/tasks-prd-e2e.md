## Relevant Files

- `test/integration/worker/helpers/temporal.go` - Temporal test harness for in-memory workflow execution (created)
- `test/integration/worker/helpers/database.go` - Database setup with TestContainers/shared DB abstraction (created)
- `test/integration/worker/helpers/redis.go` - Miniredis setup with test isolation and TTL testing (created)
- `test/integration/worker/helpers/fixtures.go` - YAML fixture loading system with validation and assertions (created)
- `test/integration/worker/fixtures/basic/simple_success.yaml` - Basic task success scenario fixture (created)
- `test/integration/worker/fixtures/basic/with_error.yaml` - Basic task error handling fixture (created)
- `test/integration/worker/fixtures/basic/with_next_task.yaml` - Basic task with next task transitions fixture (created)
- `test/integration/worker/fixtures/basic/final_task.yaml` - Basic task with final flag fixture (created)
- `test/integration/worker/basic_test.go` - Integration tests for basic task type
- `test/integration/worker/collection_test.go` - Integration tests for collection task type
- `test/integration/worker/parallel_test.go` - Integration tests for parallel task type
- `test/integration/worker/composite_test.go` - Integration tests for composite task type
- `test/integration/worker/router_test.go` - Integration tests for router task type
- `test/integration/worker/aggregate_test.go` - Integration tests for aggregate task type

### Notes

- Unit tests should typically be placed alongside the code files they are testing (e.g., `MyComponent.tsx` and `MyComponent.test.tsx` in the same directory).
- Use `npx jest [optional/path/to/test/file]` to run tests. Running without a path executes all tests found by the Jest configuration.

## Tasks

- [x] 1.0 Set up test infrastructure

    - [x] 1.1 Create the test/integration/worker directory structure with fixtures subdirectories
    - [x] 1.2 Configure TestContainers for PostgreSQL with basic connection setup
    - [x] 1.3 Set up miniredis for Redis testing
    - [x] 1.4 Configure Temporal in-memory test suite
    - [x] 1.5 Create basic YAML fixture loading functionality

- [x] 2.0 Implement basic task integration tests

    - [x] 2.1 Create YAML fixtures for basic task happy path scenarios
    - [x] 2.2 Write test for successful basic task execution
    - [x] 2.3 Verify state transitions (pending → running → completed)
    - [x] 2.4 Assert input/output persistence in database
    - [x] 2.5 Validate next task reference in response

- [ ] 3.0 Implement collection task integration tests

    - [ ] 3.1 Create YAML fixtures for collection tasks (sequential and parallel modes)
    - [ ] 3.2 Test iteration over multiple items in both modes
    - [ ] 3.3 Verify child task creation for each collection item
    - [ ] 3.4 Validate parent state aggregation after children complete
    - [ ] 3.5 Assert correct item context passing to children
    - [ ] 3.6 Test empty collection handling

- [ ] 4.0 Implement parallel task integration tests

    - [ ] 4.1 Create YAML fixtures for parallel task scenarios
    - [ ] 4.2 Test concurrent execution of child tasks
    - [ ] 4.3 Verify children start simultaneously
    - [ ] 4.4 Test fail_fast strategy with child failures
    - [ ] 4.5 Test wait_all strategy with mixed success/failure
    - [ ] 4.6 Validate output aggregation from all children

- [ ] 5.0 Implement composite task integration tests

    - [ ] 5.1 Create YAML fixtures for composite task sequences
    - [ ] 5.2 Test sequential execution order of child tasks
    - [ ] 5.3 Verify state passing between sequential tasks
    - [ ] 5.4 Assert parent completes only after all children
    - [ ] 5.5 Test early termination on child failure

- [ ] 6.0 Implement router task integration tests

    - [ ] 6.1 Create YAML fixtures for various routing conditions
    - [ ] 6.2 Test simple condition evaluation and routing
    - [ ] 6.3 Test different condition types (equality, comparison, regex)
    - [ ] 6.4 Verify unmatched condition handling
    - [ ] 6.5 Test scenarios with multiple matching routes

- [ ] 7.0 Implement aggregate task integration tests

    - [ ] 7.1 Create YAML fixtures for aggregate task scenarios
    - [ ] 7.2 Test output transformation from previous tasks
    - [ ] 7.3 Verify no external action is executed
    - [ ] 7.4 Test complex data aggregation patterns
    - [ ] 7.5 Validate error handling for missing dependencies

- [ ] 8.0 Optimize test suite performance
    - [ ] 8.1 Enable parallel test execution in Go
    - [ ] 8.2 Implement unique workflow ID generation for test isolation
    - [ ] 8.3 Optimize database cleanup between tests
    - [ ] 8.4 Measure and ensure suite executes in <30 seconds
