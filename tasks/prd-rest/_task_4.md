---
status: completed
parallelizable: true
blocked_by: ["1.0"]
---

<task_context>
<domain>engine/infra/server/router</domain>
<type>implementation</type>
<scope>middleware|helpers</scope>
<complexity>low</complexity>
<dependencies>http_server</dependencies>
<unblocks>"2.0","3.0","5.0"</unblocks>
</task_context>

# Task 4.0: Router Helpers (ETag, Pagination, Problem, Project)

## Overview

Provide shared helpers for strong ETag parsing, cursor encode/decode + `Link` headers, Problem Details responses, and project resolution.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- `ParseStrongETag` rejects weak validators and invalid formats.
- Cursor is opaque (base64) with next/prev helpers and `LimitOrDefault`.
- `RespondProblem` returns `application/problem+json` with required fields.
- `ProjectFromQueryOrDefault` resolves via `?project=` or app state default.
</requirements>

## Subtasks

- [x] 4.1 Add `etag.go`, `pagination.go`, `problem.go`, `project.go`.
- [x] 4.2 Unit‑exercise via calling handlers in integration tests.

## Sequencing

- Blocked by: 1.0
- Unblocks: 2.0, 3.0, 5.0
- Parallelizable: Yes

## Implementation Details

Helpers live under `engine/infra/server/router` and are imported by workflow handlers.

### Relevant Files

- `engine/infra/server/router/etag.go`
- `engine/infra/server/router/pagination.go`
- `engine/infra/server/router/problem.go`
- `engine/infra/server/router/project.go`

### Dependent Files

- `engine/workflow/router/workflows.go`

## Success Criteria

- Helpers compile and are exercised by handlers; headers produced as expected.
