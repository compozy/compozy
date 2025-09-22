---
status: pending
parallelizable: true
blocked_by: ["5.0", "7.0", "8.0"]
---

<task_context>
<domain>testing</domain>
<type>testing</type>
<scope>unit|integration|contract</scope>
<complexity>medium</complexity>
<dependencies>http_server</dependencies>
<unblocks>"10.0"</unblocks>
</task_context>

# Task 9.0: Tests — Unit, Integration, OpenAPI Validation

## Overview

Add/adjust tests to cover new endpoints, headers, and error responses. Add CI gate for OpenAPI schema validation.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Unit: handler logic (DTO binding, ETag parsing, Problem Details, If‑Match mismatch → 412).
- Integration: workflow + other resources CRUD; verify `Location`, `ETag`, `RateLimit-*`, `Link` headers, and read‑after‑write.
- Contract: validate generated OpenAPI against schema; ensure examples present for error cases.
</requirements>

## Subtasks

- [ ] 9.1 Replace `test/integration/resources/*` with new integration suites.
- [ ] 9.2 Add unit tests for helpers and handlers.
- [ ] 9.3 Add OpenAPI validation in CI.

## Sequencing

- Blocked by: 5.0, 7.0, 8.0
- Unblocks: 10.0
- Parallelizable: Yes

## Implementation Details

Follow project testing standards and naming. Prefer testify.

### Relevant Files

- `engine/workflow/router/workflows.go`
- `engine/infra/server/router/*`
- `docs/swagger.yaml`

### Dependent Files

- `test/integration/**`

## Success Criteria

- Tests cover headers and error mapping; CI gate validates OpenAPI spec.
