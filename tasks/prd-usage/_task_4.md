## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine/infra/server</domain>
<type>integration</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>database</dependencies>
</task_context>

# Task 4.0: API & CLI Usage Exposure

## Overview

Expose execution usage data through workflow/task/agent APIs and CLI commands using the new `UsageSummary` DTO, ensuring backward compatibility and null-safe behavior.

<critical>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</critical>

<requirements>
- Create `UsageSummary` struct and embed it in agent/task/workflow execution DTOs.
- Update routers to fetch usage rows from repository (workflow, task, agent endpoints).
- Ensure failed executions still include whatever usage data is stored (can be null if unavailable).
- Update CLI outputs (`compozy executions ... get`) to render usage tokens.
- Generate example response matching tech spec sample for documentation reuse.
</requirements>

## Subtasks

- [ ] 4.1 Add `UsageSummary` model and repository wiring in routers
- [ ] 4.2 Update workflow/task/agent execution endpoints to include `usage` field
- [ ] 4.3 Update CLI formatting and golden files to surface usage tokens
- [ ] 4.4 Add/refresh API contract tests and CLI goldens

## Implementation Details

- Follow “API Exposure & Client Compatibility” section in `_techspec.md`.
- JSON schema should treat usage fields as nullable (documented in docs task).
- Maintain existing response shapes; `usage` is additive and optional.

### Relevant Files

- `engine/agent/router/exec.go`
- `engine/task/router/exec.go`
- `engine/workflow/router/execs.go`
- `engine/infra/server/router/dto.go`
- `cli/cmd/executions/*.go`
- CLI golden files (if present)

### Dependent Files

- `engine/llm/usage/collector.go`
- `engine/infra/postgres/usage_repo.go`
- `schemas/execution-usage.json`

## Deliverables

- API responses for workflows/tasks/agents include `usage` object reflecting latest status
- CLI commands display usage totals in human-readable format
- Updated tests cover API contract and CLI golden outputs with usage data

## Tests

- Unit/integration tests mapped from `_tests.md` for this feature:
  - [ ] API contract assertions for `usage` presence/nullability
  - [ ] CLI golden updates verifying new fields

## Success Criteria

- API consumers receive usage data without breaking existing clients
- CLI outputs align with documentation examples
- Tests pass and demonstrate both populated and null usage cases
