## markdown

## status: completed # Options: pending, in-progress, completed, excluded

<task_context>
<domain>test/integration/standalone</domain>
<type>testing</type>
<scope>integration_validation</scope>
<complexity>medium</complexity>
<dependencies>cache|memory_store|resource_store|streaming|persistence</dependencies>
</task_context>

# Task 9.0: End-to-End Workflow Tests [Size: M - 1-2 days]

## Overview

Create comprehensive end-to-end integration tests that validate complete workflow execution in standalone mode. These tests verify that all components (cache, memory store, resource store, streaming, persistence) work together correctly using miniredis as the cache backend.

<critical>
- **ALWAYS READ** @.cursor/rules/test-standards.mdc before start
- **ALWAYS READ** the technical docs from this PRD before start
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha, and support old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility
- **MUST** use `t.Context()` in all tests - NEVER `context.Background()`
- **MUST** follow `t.Run("Should ...")` naming convention
- **MUST** use testify assertions (require/assert)
</critical>

<research>
# When you need information about a library or external API:
- use perplexity and context7 to find out how to properly fix/resolve this
- when using perplexity mcp, you can pass a prompt to the query param with more description about what you want to know, you don't need to pass a query-style search phrase, the same for the topic param of context7
- for context7 to use the mcp is two steps, one you will find out the library id and them you will check what you want
</research>

<requirements>
- Complete workflow execution in standalone mode must work identically to distributed mode
- Multi-agent workflows must execute correctly
- Workflows with memory and tools must function properly
- Concurrent workflow execution must be supported
- Workflow state persistence must work across snapshots
- All tests must be deterministic and parallelizable where safe
- Test coverage >80% for integration test scenarios
</requirements>

## Subtasks

- [x] 9.1 Create test environment helper for standalone mode
- [x] 9.2 Implement end-to-end workflow execution tests
- [x] 9.3 Implement multi-agent workflow tests
- [x] 9.4 Implement workflows with memory and tools tests
- [x] 9.5 Implement concurrent workflow execution tests
- [x] 9.6 Implement workflow state persistence tests
- [x] 9.7 Add performance benchmarks for workflow execution

## Implementation Details

### Test Structure

Create integration tests under `test/integration/standalone/` that validate complete workflow execution scenarios. Use real miniredis (no mocks) with test fixtures and helper functions.

### Relevant Files

**New Files:**
- `test/integration/standalone/workflow_test.go` - End-to-end workflow execution tests
- `test/integration/standalone/helpers.go` - Test environment setup helpers
- `test/fixtures/standalone/workflows/test-workflow.yaml` - Sample workflow fixture
- `test/fixtures/standalone/workflows/stateful-workflow.yaml` - Workflow with memory fixture

### Dependent Files

- `engine/infra/cache/miniredis_standalone.go` - Miniredis wrapper (Task 2.0)
- `engine/infra/cache/snapshot_manager.go` - Snapshot manager (Task 7.0)
- `engine/memory/store/redis.go` - Memory store (Task 4.0)
- `engine/resources/redis_store.go` - Resource store (Task 5.0)
- `engine/infra/server/dependencies.go` - Server dependencies setup

## Deliverables

- Test environment helper (`SetupStandaloneTestEnv`) for integration tests
- End-to-end workflow execution tests with complete lifecycle
- Multi-agent workflow tests with agent coordination
- Memory and tools integration tests with state management
- Concurrent workflow execution tests (10+ workflows)
- State persistence tests with snapshot/restore cycles
- Performance benchmarks comparing standalone vs distributed mode
- Test fixtures and sample workflows
- All tests passing with `make test`

## Tests

Unit tests mapped from `_tests.md` for this feature:

### End-to-End Workflow Tests (`test/integration/standalone/workflow_test.go`)

- [x] Should execute complete workflow with agent, tasks, and tools in standalone mode
- [ ] Should persist conversation history across workflow steps
- [ ] Should handle workflow state correctly during execution
- [x] Should execute multiple workflows concurrently (10+ workflows)
- [ ] Should handle workflow errors and retries gracefully
- [ ] Should maintain workflow isolation (no cross-workflow interference)
- [ ] Should cleanup resources after workflow completion

### Multi-Agent Workflows

- [ ] Should coordinate multiple agents in single workflow
- [ ] Should maintain separate conversation histories per agent
- [ ] Should share resources between agents correctly
- [ ] Should handle agent failures without affecting other agents

### Workflows with Memory and Tools

- [ ] Should persist agent memory across workflow steps
- [ ] Should execute tool calls correctly
- [ ] Should maintain tool state across invocations
- [ ] Should handle tool errors gracefully

### Concurrent Execution

- [ ] Should execute 10+ workflows concurrently without interference
- [ ] Should maintain correct state for each workflow
- [ ] Should handle concurrent cache operations correctly
- [ ] Should not exceed memory limits under load

### State Persistence

- [ ] Should persist workflow state to snapshots
- [ ] Should restore workflow state after restart
- [ ] Should handle snapshot failures gracefully
- [ ] Should continue execution after restore

### Performance Benchmarks

- [ ] Should complete workflow within 1.5x of Redis time
- [ ] Should handle 100+ cache operations per second
- [ ] Should use <512MB memory for typical workload
- [ ] Should complete snapshots within 5 seconds

### Edge Cases

- [ ] Should handle empty workflows
- [ ] Should handle workflows with no memory or tools
- [ ] Should handle long-running workflows (>1 hour)
- [ ] Should recover from miniredis errors

## Success Criteria

- All workflow execution tests pass (`go test -v -race ./test/integration/standalone/`)
- Workflows execute correctly in standalone mode with miniredis
- Multi-agent workflows coordinate properly
- Memory and tools work as expected
- Concurrent workflows execute without interference (10+ concurrent)
- State persistence works across restarts
- Performance benchmarks meet NFRs (<1.5x Redis time, 100+ ops/sec)
- Test coverage >80% for integration scenarios
- No flaky tests in the test suite
- All tests follow project test standards (naming, context, assertions)
