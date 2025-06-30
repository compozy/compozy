# TaskResponder & ConfigManager Refactor Implementation Task Summary

## Relevant Files

### Core Implementation Files

- `engine/task/services/task_responder.go` - Legacy TaskResponder service (732 LOC)
- `engine/task/services/config_manager.go` - Legacy ConfigManager service (493 LOC)
- `engine/task2/shared/interfaces.go` - New shared interfaces and contracts
- `engine/task2/shared/response_handler.go` - Common response handling logic
- `engine/task2/collection/expander.go` - Domain service for collection expansion
- `engine/task2/core/config_repo.go` - Infrastructure service for config storage
- `engine/task2/factory.go` - Extended factory for component creation

### Task-Specific Response Handlers

- `engine/task2/basic/response_handler.go` - Basic task response handling
- `engine/task2/parallel/response_handler.go` - Parallel task response handling
- `engine/task2/collection/response_handler.go` - Collection task response handling
- `engine/task2/composite/response_handler.go` - Composite task response handling
- `engine/task2/router/response_handler.go` - Router task response handling
- `engine/task2/wait/response_handler.go` - Wait task response handling
- `engine/task2/signal/response_handler.go` - Signal task response handling
- `engine/task2/aggregate/response_handler.go` - Aggregate task response handling

### Integration Points

- `engine/worker/activities.go` - Main integration point for new components
- `engine/task/activities/*.go` - Individual activity implementations

### Testing Files

- `engine/task2/*/response_handler_test.go` - Unit tests for response handlers
- `engine/task2/integration/*_test.go` - Integration tests
- `engine/task2/golden/*_golden_test.go` - Golden master tests

### Documentation Files

- `tasks/prd-refac/services_new/_prd.md` - Product requirements document
- `tasks/prd-refac/services_new/_techspec.md` - Technical specification

### Notes

- Unit tests should be placed alongside the implementation files (e.g., `response_handler.go` and `response_handler_test.go` in the same directory)
- Use `go test ./...` to run all tests or `go test -v ./path/to/package` for specific package tests
- Always run `make fmt && make lint && make test` before committing changes
- Follow project testing standards with `t.Run("Should...")` pattern
- Progressive refactoring: build new components first, test thoroughly, validate behavior, then replace

## Tasks

- [ ] 1.0 Shared Interfaces & Components
- [ ] 2.0 CollectionExpander Domain Service
- [ ] 3.0 TaskConfigRepository Infrastructure Service
- [ ] 4.0 BaseResponseHandler Implementation
- [ ] 5.0 Task-Specific Response Handlers
- [ ] 6.0 Factory Integration
- [ ] 7.0 Comprehensive Testing Suite
- [ ] 8.0 Behavior Validation & Golden Master Tests
- [ ] 9.0 Activities.go Integration
- [ ] 10.0 Legacy Service Removal
