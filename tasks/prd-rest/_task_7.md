---
status: completed
parallelizable: true
blocked_by: ["6.0"]
---

<task_context>
<domain>engine/{project|memoryconfig|model|schema}</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server|database</dependencies>
<unblocks>"9.0","10.0"</unblocks>
</task_context>

# Task 7.0: Promote Project/Schemas/Models/Memories CRUD Endpoints

## Overview

Create top‑level routers and UC packages for project, memory configs (base `/memories`), models, and schemas with identical semantics to workflows (ETag, Problem Details, pagination, `fields`/`expand` as applicable).

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Add packages: `engine/project/router|uc`, `engine/memoryconfig/router|uc`, `engine/model/router|uc`, `engine/schema/router|uc`.
- Implement list/get/put/delete with keyset cursor pagination (`after`/`before` tokens) and strong `If-Match`.
- Add delete referential integrity checks returning `409 Conflict` with references.
</requirements>

## Subtasks

- [ ] 7.1 Scaffold routers and UC packages for each resource.
- [ ] 7.2 Implement handlers and wire route registration.
- [ ] 7.3 Add Swagger annotations and examples.

## Sequencing

- Blocked by: 6.0
- Unblocks: 9.0, 10.0
- Parallelizable: Yes (per resource)

## Implementation Details

Replicate workflow patterns; reuse shared router helpers.

### Relevant Files

- `engine/infra/server/router/*`
- `engine/{project,memoryconfig,model,schema}/router/*`
- `engine/{project,memoryconfig,model,schema}/uc/*`

### Dependent Files

- `docs/swagger.yaml`

## Success Criteria

- All Phase‑2 resources expose CRUD with correct headers and error handling.
