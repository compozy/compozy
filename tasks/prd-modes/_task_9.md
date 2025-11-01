## status: completed

<task_context>
<domain>test/helpers</domain>
<type>testing</type>
<scope>test_infrastructure</scope>
<complexity>low</complexity>
<dependencies>database|sqlite</dependencies>
</task_context>

# Task 9.0: Update Test Helpers

## Overview

Update test helper utilities to default to SQLite memory mode instead of PostgreSQL testcontainers, enabling 50-80% faster test execution with zero Docker dependencies.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the technical docs from this PRD before start (tasks/prd-modes/_techspec.md)
- **YOU SHOULD ALWAYS** have in mind that this is a greenfield approach - no backwards compatibility required
</critical>

<research>
When you need information about SQLite best practices:
- Use perplexity to find SQLite testing patterns and in-memory database optimization
- Use context7 for Go testing framework documentation
</research>

<requirements>
- Default `SetupTestDatabase` to SQLite memory mode
- Remove testcontainers as default dependency
- Add explicit `SetupPostgresContainer` for tests requiring PostgreSQL
- Ensure all helper functions use `t.Context()` instead of `context.Background()`
- Maintain backwards compatibility for tests explicitly requiring PostgreSQL
</requirements>

## Subtasks

- [x] 9.1 Update `SetupTestDatabase` to default to SQLite :memory:
- [x] 9.2 Add explicit `SetupPostgresContainer` helper for PostgreSQL tests
- [x] 9.3 Update `GetSharedPostgresDB` documentation to recommend SQLite
- [x] 9.4 Verify context inheritance patterns (t.Context() usage)
- [x] 9.5 Run test suite to measure performance improvement

## Implementation Details

### Objective
Change default test database from PostgreSQL testcontainers to SQLite memory mode for dramatic speed improvements while maintaining PostgreSQL test capability for specialized tests.

### Key Changes

**File:** `test/helpers/database.go`

1. **Update `SetupTestDatabase` signature and implementation:**
   - Change default driver from "postgres" to "sqlite"
   - Use `:memory:` as default SQLite path
   - Remove testcontainers startup from default path

2. **Add new helper:**
   - Create `SetupPostgresContainer(t *testing.T)` for tests explicitly requiring PostgreSQL
   - Move testcontainers logic from `SetupTestDatabase` to this new function

3. **Update documentation:**
   - Document when to use SQLite vs PostgreSQL in tests
   - Add migration guide comments for existing tests

### Relevant Files

- `test/helpers/database.go` - Primary implementation
- `test/helpers/standalone.go` - May reference database helpers

### Dependent Files

- All integration tests using `SetupTestDatabase`
- Tests using `GetSharedPostgresDB`

## Deliverables

- Updated `test/helpers/database.go` with SQLite default
- New `SetupPostgresContainer` helper for PostgreSQL tests
- Documentation comments explaining when to use each helper
- Performance benchmarks showing speedup

## Tests

Since this task updates test infrastructure itself, validation is through:

- [x] Run `make test` and verify all tests pass
- [x] Measure test suite execution time (should be 50-80% faster)
- [x] Verify no testcontainers startup in default test runs
- [x] Confirm PostgreSQL tests still work with explicit `SetupPostgresContainer`
- [x] Check that all helpers use `t.Context()` for proper context inheritance

## Success Criteria

- All tests pass with SQLite as default
- Test suite runs 50-80% faster than before
- Zero Docker dependencies for default test runs
- PostgreSQL tests still functional with explicit opt-in
- No `context.Background()` usage in test helpers
