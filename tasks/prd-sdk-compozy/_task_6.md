
## markdown

## status: completed

<task_context>
<domain>sdk/compozy</domain>
<type>integration</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>external_apis</dependencies>
</task_context>

# Task 6.0: Complete Test Coverage

## Overview

Consolidate testing coverage to complete the SDK release package.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the technicals docs from this PRD before start
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha, and support old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility
</critical>

<research>
# When you need information about a library or external API:
- use perplexity and context7 to find out how to properly fix/resolve this
- when using perplexity mcp, you can pass a prompt to the query param with more description about what you want to know, you don't need to pass a query-style search phrase, the same for the topic param of context7
- for context7 to use the mcp is two steps, one you will find out the library id and them you will check what you want
</research>

<requirements>
- Implement remaining integration, migration, and performance tests plus coverage gates from `_tests.md` ensuring ≥85% coverage.
</requirements>

## Subtasks

- [x] 6.1 Add remaining integration and migration tests (e.g., `example_compat`, benchmarking harness) and ensure targets wired into CI (§13 Phase 6).

## Implementation Details (**FOR LLM READING THIS: KEEP THIS BRIEFLY AND HIGH-LEVEL, THE IMPLEMENTATION ALREADY EXIST IN THE TECHSPEC**)

Leverage §8 for usage examples, §13 Phase 6 for deliverables.

### Relevant Files

- `sdk/compozy/migration/example_compat_test.go`
- `mage` or CI configs for new test targets

### Dependent Files

- `sdk/MIGRATION_GUIDE.md`

## Deliverables

- Comprehensive integration/performance tests and migration smoke coverage.

## Tests

- Unit tests mapped from `_tests.md` for this feature:
  - [x] `sdk/compozy/migration/example_compat_test.go`
  - [x] `mage integration:sdkCompozy` (or equivalent) executes full suite
  - [x] Performance/benchmark checks for loader throughput (<100ms per file)

## Success Criteria

- All tests pass under `make lint` and `make test`, with additional integration target green.
- Coverage ≥85% on `sdk/compozy`.
