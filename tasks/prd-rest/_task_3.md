---
status: completed
parallelizable: true
blocked_by: ["1.0"]
---

<task_context>
<domain>engine/workflow/router</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server|database</dependencies>
<unblocks>"5.0","6.0","9.0"</unblocks>
</task_context>

# Task 3.0: Workflow GET Rewiring + Cursor Pagination

## Overview

Replace in‑memory reads with UC/store backed reads for workflows. Implement keyset cursor pagination (`limit`, `cursor`) with `Link` headers and include `page.next_cursor` when applicable. Cursor tokens must encode direction (`after`/`before`) plus the workflow ID to keep pagination stable as items change.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Use `engine/workflow/uc.Get` and `engine/workflow/uc.List` for reads.
- Support `q` prefix filters; support `fields=` and `expand=`.
- Add `Link` headers for next/prev and include `page.limit`, `page.next_cursor`, and `page.prev_cursor` in the body.
- Return `ETag` on GET of a single item.
</requirements>

## Subtasks

- [x] 3.1 Implement `listWorkflows` with cursor and `Link` headers.
- [x] 3.2 Implement `getWorkflowByID` returning body with `_etag` and header `ETag`.
- [x] 3.3 Implement response shaping for `fields`/`expand`.

## Sequencing

- Blocked by: 1.0
- Unblocks: 5.0, 6.0, 9.0
- Parallelizable: Yes

## Implementation Details

See `engine/workflow/router/workflows.go` and `engine/workflow/uc/*`.

### Relevant Files

- `engine/workflow/router/workflows.go`
- `engine/workflow/uc/list.go`
- `engine/workflow/uc/get.go`
- `engine/infra/server/router/pagination.go`

### Dependent Files

- `engine/infra/server/router/problem.go`
- `engine/infra/server/router/project.go`

## Success Criteria

- GET returns expected shape with `page` object and correct `Link` headers.
- Single GET returns `ETag` header and `_etag` field in body.
