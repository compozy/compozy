## markdown

## status: completed # Options: pending, in-progress, completed, excluded

<task_context>
<domain>test</domain>
<type>testing</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>config|cache|temporal</dependencies>
</task_context>

# Task 4.0: Rename Test Functions, Files & Update Test Cases

## Overview

This task renames test helper functions, test files, and potentially the test package directory from "standalone" to "embedded" terminology. It also adds new test cases for MCPProxy mode validation and updates existing test assertions.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the _techspec.md and the _prd.md docs from this PRD before start
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha, and support old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility
</critical>

<research>
# When you need information about a library or external API:
- use perplexity and context7 to find out how to properly fix/resolve this
- when using perplexity mcp, you can pass a prompt to the query param with more description about what you want to know, you don't need to pass a query-style search phrase, the same for the topic param of context7
- for context7 to use the mcp is two steps, one you will find out the library id and them you will check what you want
</research>

<requirements>
- Rename test helper functions (`startStandaloneServer` → `startEmbeddedServer`, etc.)
- Rename test files (`standalone_test.go` → `embedded_test.go`)
- Consider renaming test package directory (`standalone/` → `embedded/`)
- Add MCPProxy mode validation tests
- Update test fixtures paths if directory renamed
- Update existing test assertions to use new function names
- Update test function names (`TestStandalone*` → `TestEmbedded*`)
</requirements>

## Subtasks

- [x] 4.1 Rename `startStandaloneServer` → `startEmbeddedServer` in `test/integration/temporal/standalone_test.go`
- [x] 4.2 Rename test functions: `TestStandaloneMemoryMode` → `TestEmbeddedMemoryMode`
- [x] 4.3 Rename test functions: `TestStandaloneFileMode` → `TestEmbeddedFileMode`
- [x] 4.4 Rename test functions: `TestStandaloneCustomPorts` → `TestEmbeddedCustomPorts`
- [x] 4.5 Rename test functions: `TestStandaloneWorkflowExecution` → `TestEmbeddedWorkflowExecution`
- [x] 4.6 Rename file: `test/integration/temporal/standalone_test.go` → `embedded_test.go`
- [x] 4.7 Update references in `test/integration/temporal/startup_lifecycle_test.go`
- [x] 4.8 Update references in `test/integration/temporal/persistence_test.go`
- [x] 4.9 Update references in `test/integration/temporal/errors_test.go`
- [x] 4.10 Update references in `test/integration/temporal/mode_switching_test.go`
- [x] 4.11 Update `toEmbeddedConfig` function to use `EmbeddedTemporalConfig` type
- [x] 4.12 Consider renaming directory: `test/integration/standalone/` → `test/integration/embedded/`
- [x] 4.13 Update package declaration in standalone test package (if directory renamed)
- [x] 4.14 Rename helper functions: `SetupStandaloneStreaming` → `SetupEmbeddedStreaming`
- [x] 4.15 Rename helper functions: `SetupStandaloneResourceStore` → `SetupEmbeddedResourceStore`
- [x] 4.16 Rename helper functions: `SetupStandaloneWithPersistence` → `SetupEmbeddedWithPersistence`
- [x] 4.17 Update all test files in `test/integration/standalone/` directory
- [x] 4.18 Add MCPProxy mode validation tests to `pkg/config/loader_test.go`
- [x] 4.19 Update test fixtures paths if directory renamed
- [x] 4.20 Update existing test assertions

## Implementation Details

See Phase 3.1, 3.2, and Phase 4.1, 4.2, 4.3 in the techspec for detailed implementation steps.

Key changes:
- Rename all test helper functions to use "embedded"
- Rename test files from `standalone_test.go` to `embedded_test.go`
- Consider renaming test package directory
- Add comprehensive MCPProxy validation tests
- Update all test references and assertions

### Relevant Files

- `test/integration/temporal/standalone_test.go` (rename to `embedded_test.go`)
- `test/integration/temporal/startup_lifecycle_test.go`
- `test/integration/temporal/persistence_test.go`
- `test/integration/temporal/errors_test.go`
- `test/integration/temporal/mode_switching_test.go`
- `test/integration/standalone/` directory (consider renaming)
- `pkg/config/loader_test.go`
- `test/fixtures/standalone/` directory (update paths if renamed)

### Dependent Files

- `engine/worker/embedded/server.go` - Used by test helpers
- `pkg/config/config.go` - Config types used in tests
- `pkg/config/loader.go` - Validation logic being tested

## Deliverables

- All test helper functions renamed to use "embedded" terminology
- Test files renamed (`standalone_test.go` → `embedded_test.go`)
- Test package directory optionally renamed (`standalone/` → `embedded/`)
- MCPProxy mode validation tests added
- Test fixtures paths updated
- All test assertions updated
- All tests passing
- Test coverage maintained or improved

## Tests

- Unit tests for MCPProxy validation:
  - [x] Test MCPProxy mode validation rejects "standalone" mode with helpful error
  - [x] Test MCPProxy mode validation accepts empty string (inheritance)
  - [x] Test MCPProxy mode validation accepts "memory" mode
  - [x] Test MCPProxy mode validation accepts "persistent" mode
  - [x] Test MCPProxy mode validation accepts "distributed" mode
  - [x] Test MCPProxy mode validation rejects invalid mode values
  - [x] Test MCPProxy port validation still works correctly

- Integration tests:
  - [x] Verify renamed test functions work correctly
  - [x] Verify embedded Temporal tests work with renamed helpers
  - [x] Verify cache tests work with renamed setup functions
  - [x] Verify test fixtures load correctly after path updates

- Regression tests:
  - [x] Run all existing integration tests
  - [x] Run all existing unit tests
  - [x] Verify no test failures introduced

## Success Criteria

- All test helper functions use "embedded" terminology
- All test files renamed appropriately
- Test package directory renamed (if decided)
- MCPProxy validation tests comprehensive and passing
- All test references updated
- Test fixtures paths updated correctly
- All tests pass (`make test`)
- Test coverage maintained or improved
- No broken test references
- Linter passes (`make lint`)
