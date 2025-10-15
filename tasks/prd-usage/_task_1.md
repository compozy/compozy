## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine/infra/postgres</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>database</dependencies>
</task_context>

# Task 1.0: Database Migration & Usage Repository

## Overview

Create the `execution_llm_usage` persistence layer so execution token totals can be stored reliably with referential integrity. This includes the goose migration, repository implementation, wiring, and initial unit tests.

<critical>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</critical>

<requirements>
- Add goose migration `20251015090000_create_execution_llm_usage.sql` with columns, indexes, and foreign keys exactly as defined in the tech spec.
- Implement `UsageRepository` with idempotent upsert semantics and FK validation.
- Ensure repository is registered for dependency injection so collectors can call it.
- Update change-control references (e.g., add migration entry to changelog stub if required).
</requirements>

## Subtasks

- [ ] 1.1 Create migration with table, indexes, and FK constraints
- [ ] 1.2 Implement `engine/infra/postgres/usage_repo.go` and constructor wiring
- [ ] 1.3 Add repository unit tests (`usage_repo_test.go`) using pgxmock
- [ ] 1.4 Update integration fixtures/setup (if needed) to include new table

## Implementation Details

- See “Data Models” in `_techspec.md` for column list, indexes, and naming conventions.
- Migration should default `created_at`/`updated_at` to `now()` and include `(component, created_at)` composite index for reporting.
- Repository must expose `Upsert`, `GetByTaskExecID`, and `GetByWorkflowExecID` operations and handle nullable references.

### Relevant Files

- `engine/infra/postgres/migrations/20251015090000_create_execution_llm_usage.sql`
- `engine/infra/postgres/usage_repo.go`
- `engine/infra/postgres/usage_repo_test.go`

### Dependent Files

- `engine/llm/usage/collector.go`
- `engine/llm/orchestrator/loop.go`

## Deliverables

- Migration file applied locally via goose and ready for deployment
- Repository implementation compiled and wired into application composition root
- Passing unit tests covering upsert, FK validation, and lookup paths
- Updated migration fixtures for integration tests (if applicable)

## Tests

- Unit tests mapped from `_tests.md` for this feature:
  - [ ] `engine/infra/postgres/usage_repo_test.go` – upsert + unique constraint coverage
  - [ ] Integration DB fixture ensuring migration loads before tests

## Success Criteria

- Migration runs cleanly on local Postgres instance (apply + rollback)
- Repository methods return expected rows and error on orphan references
- CI pipeline includes new migration and tests without regressions
