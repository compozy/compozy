## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine/llm</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>database</dependencies>
</task_context>

# Task 3.0: Usage Collector & Orchestrator Integration

## Overview

Introduce the `llmusage` package, aggregate usage during orchestrator runs, propagate execution metadata (including direct tasks), and persist totals via the new repository. Document the architectural decision with an ADR.

<critical>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</critical>

<requirements>
- Create `engine/llm/usage` with collector interfaces, aggregation logic, and status-aware finalization that calls the repository.
- Wire collector into orchestrator loop and ensure direct task executions pass metadata (task/workflow IDs, component type).
- Handle retries/timeouts by upserting usage rows when executions reach any terminal status.
- Log ingestion failures and propagate metrics hooks for later observability task.
- Record ADR `docs/adr/20251015-llm-usage-reporting.md` summarizing architecture and trade-offs.
</requirements>

## Subtasks

- [ ] 3.1 Implement `engine/llm/usage` collector (record, finalize, status-aware upsert)
- [ ] 3.2 Hook collector into orchestrator (`loop.go`, context propagation, error paths)
- [ ] 3.3 Update direct executor / agent execution paths to supply metadata to collector
- [ ] 3.4 Add unit tests (`collector_test.go`) and integration tests (`test/integration/usage/*`)
- [ ] 3.5 Publish ADR documenting the usage aggregation decision and repository interface

## Implementation Details

- Follow “Component Overview” and “Development Sequencing” sections in `_techspec.md`.
- Use context-based access for logger/config; no singletons.
- Integration tests should cover workflow, task, and missing metadata scenarios per `_tests.md`.

### Relevant Files

- `engine/llm/usage/*`
- `engine/llm/orchestrator/loop.go`
- `engine/task/directexec/direct_executor.go`
- `engine/agent/exec/runner.go`
- `docs/adr/20251015-llm-usage-reporting.md`
- `test/integration/usage/*`

### Dependent Files

- `engine/infra/postgres/usage_repo.go`
- `engine/infra/server/router/*.go`
- `infra/monitoring/*`

## Deliverables

- New `llmusage` package with interfaces and implementation
- Orchestrator and direct executor wired to collector with status-aware upserts
- Passing unit and integration tests covering workflow/task/missing metadata cases
- ADR committed and referenced from tech spec/PRD

## Tests

- Unit tests mapped from `_tests.md` for this feature:
  - [ ] `engine/llm/usage/collector_test.go` – aggregation and finalize behavior
- Integration tests mapped from `_tests.md`:
  - [ ] `test/integration/usage/workflow_usage_test.go`
  - [ ] `test/integration/usage/task_usage_test.go`
  - [ ] `test/integration/usage/missing_metadata_test.go`

## Success Criteria

- Collector reliably persists usage for all terminal statuses, including retries/timeouts
- Direct task executions produce stored usage rows identical to workflow runs
- ADR documents rationale and is linked from `_techspec.md` and `_prd.md`
- All tests pass locally and in CI without flakiness
