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
<dependencies>http_server</dependencies>
<unblocks>"3.0","5.0","6.0"</unblocks>
</task_context>

# Task 2.0: Workflow PUT/DELETE Endpoints (Idempotent Upsert + Strong ETag)

## Overview

Add `PUT /workflows/{workflow_id}` and `DELETE /workflows/{workflow_id}` with strong `If-Match` semantics and provenance metadata writes.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Use `router.GetResourceStore(c)` and per‑resource UC: `engine/workflow/uc.Upsert` and `engine/workflow/uc.Delete`.
- Parse `If-Match` via strong comparator; weak validators rejected.
- On create via PUT, return `201 Created` and set `Location` to `/api/v0/workflows/{id}`; on update return `200 OK`.
- Always set `ETag` on success; respond via envelope helpers.
</requirements>

## Subtasks

- [x] 2.1 Implement handler `upsertWorkflow` with ETag handling and Location.
- [x] 2.2 Implement handler `deleteWorkflow` with 204 semantics.
- [x] 2.3 Wire routes in `register.go`.

## Sequencing

- Blocked by: 1.0
- Unblocks: 3.0, 5.0, 6.0
- Parallelizable: Yes

## Implementation Details

See implemented files under `engine/workflow/router` and `engine/workflow/uc`.

### Relevant Files

- `engine/workflow/router/register.go`
- `engine/workflow/router/workflows.go`
- `engine/workflow/uc/upsert.go`
- `engine/workflow/uc/delete.go`
- `engine/infra/server/routes/routes.go`

### Dependent Files

- `engine/infra/server/router/etag.go`
- `engine/infra/server/router/problem.go`

## Success Criteria

- Handlers exist and return correct status codes, `ETag`, and `Location`.
- UC writes provenance meta and respects strong `If-Match`.
