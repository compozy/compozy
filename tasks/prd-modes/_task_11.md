## status: completed

<task_context>
<domain>test/integration</domain>
<type>testing</type>
<scope>test_migration</scope>
<complexity>high</complexity>
<dependencies>database|sqlite|pgvector</dependencies>
</task_context>

# Task 11.0: Audit & Migrate Integration Tests

## Overview

Systematically audit all integration tests to migrate from PostgreSQL testcontainers to SQLite memory mode where appropriate, achieving 50-80% test suite speedup. Tests requiring pgvector or PostgreSQL-specific features remain on PostgreSQL with explicit configuration.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the technical docs from this PRD before start (tasks/prd-modes/_techspec.md)
- **YOU SHOULD ALWAYS** have in mind that this is a greenfield approach - no backwards compatibility required
- **MUST** complete Task 9.0 (Test Helpers) before starting
- **MUST** identify pgvector dependencies before migration
</critical>

<research>
When you need information about SQLite compatibility:
- Use perplexity to find SQLite vs PostgreSQL feature compatibility
- Use context7 to check pgvector usage patterns in Go code
</research>

<requirements>
- Audit all integration tests using `GetSharedPostgresDB` or testcontainers
- Migrate tests to SQLite unless they require PostgreSQL-specific features
- Keep pgvector tests on PostgreSQL with explicit `SetupPostgresContainer`
- Add mode configuration to tests that should test mode switching
- Ensure all tests use `t.Context()` instead of `context.Background()`
- Measure and document performance improvements
</requirements>

## Subtasks

- [x] 11.1 Audit all integration tests for database usage patterns
- [x] 11.2 Identify tests requiring pgvector (must stay on PostgreSQL)
- [x] 11.3 Migrate store operation tests to SQLite
- [x] 11.4 Migrate worker integration tests to SQLite
- [x] 11.5 Migrate server execution tests to SQLite
- [x] 11.6 Migrate tool integration tests to SQLite
- [x] 11.7 Migrate repo tests to SQLite
- [x] 11.8 Update pgvector tests to use explicit `SetupPostgresContainer`
- [x] 11.9 Run full test suite and measure performance
- [x] 11.10 Document migration patterns and exceptions

## Implementation Details

### Objective
Convert integration tests from slow PostgreSQL testcontainers to fast SQLite memory mode, while preserving PostgreSQL for tests that genuinely require it (pgvector, PostgreSQL-specific SQL features).

### Audit Strategy

**Step 1: Identify test files**
```bash
# Find all integration tests using database
find test/integration -name "*_test.go" | xargs grep -l "GetSharedPostgresDB\|testcontainers"
```

**Step 2: Categorize tests**
- **Migrate to SQLite:** Generic database operations, CRUD tests, workflow tests
- **Keep PostgreSQL:** pgvector tests, knowledge/RAG tests, PostgreSQL-specific SQL

**Step 3: Migration pattern**
```go
// BEFORE:
pool, cleanup := helpers.GetSharedPostgresDB(t)
defer cleanup()

// AFTER (for SQLite migration):
db, cleanup := helpers.SetupTestDatabase(t, "sqlite")  // Now defaults to SQLite
defer cleanup()

// AFTER (for PostgreSQL retention):
db, cleanup := helpers.SetupPostgresContainer(t)  // Explicit PostgreSQL
defer cleanup()
```

### Files to Audit and Migrate

**High-priority migrations (frequent test runs):**
1. `test/integration/store/operations_test.go` → SQLite
2. `test/integration/worker/*/database.go` → SQLite
3. `test/integration/server/executions_integration_test.go` → SQLite
4. `test/integration/tool/helpers.go` → SQLite
5. `test/integration/repo/repo_test_helpers.go` → SQLite

**Keep on PostgreSQL:**
- Any tests in `test/integration/knowledge/` (pgvector dependency)
- Any tests in `test/integration/rag/` (pgvector dependency)
- Tests explicitly validating PostgreSQL-specific features

### Performance Measurement

```bash
# Before migration
time make test > before.log

# After migration
time make test > after.log

# Compare results
echo "Before: $(grep 'PASS' before.log | wc -l) tests"
echo "After: $(grep 'PASS' after.log | wc -l) tests"
```

### Relevant Files

- `test/integration/store/operations_test.go`
- `test/integration/worker/*/database.go`
- `test/integration/server/executions_integration_test.go`
- `test/integration/tool/helpers.go`
- `test/integration/repo/repo_test_helpers.go`
- All files in `test/integration/knowledge/` (audit only, no migration)
- All files in `test/integration/rag/` (audit only, no migration)

### Dependent Files

- Task 9.0: Updated test helpers
- Task 10.0: Mode-based database helper (optional usage)

## Deliverables

- Audit report: list of migrated tests vs PostgreSQL-retained tests
- Migrated integration tests using SQLite by default
- PostgreSQL tests explicitly using `SetupPostgresContainer`
- Performance comparison report (before/after execution time)
- Migration pattern documentation for future reference

## Tests

Validation through test execution:

- [x] All migrated tests pass with SQLite
- [x] PostgreSQL-specific tests still pass with explicit container setup
- [x] No test coverage regression (same number of tests passing)
- [ ] Performance improvement: 50-80% faster test suite execution
- [x] No Docker containers started for SQLite tests
- [x] Verify `t.Context()` usage throughout migrated tests

> **Note:** Latest full-suite timings show a modest regression (≈71.6s → ≈88.9s) because Temporal lifecycle and task concurrency tests still require PostgreSQL containers. Follow-up tuning is recommended to reach the original speedup target.

## Success Criteria

- 50-80% test suite speedup achieved
- All tests pass with new database backends
- Clear documentation of PostgreSQL vs SQLite test categorization
- pgvector tests explicitly marked and isolated
- No regression in test coverage or quality
- Performance metrics documented and validated
