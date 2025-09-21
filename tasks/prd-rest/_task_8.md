---
status: completed
parallelizable: true
blocked_by: ["6.0"]
---

<task_context>
<domain>engine/{agent|tool|task}/router</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server|database</dependencies>
<unblocks>"9.0","10.0"</unblocks>
</task_context>

# Task 8.0: Top‑Level Agents/Tools/Tasks CRUD + Referential Integrity

## Overview

Add top‑level list/get/put/delete endpoints for agents, tools, and tasks (independent resources). Keep nested read views under workflows. Enforce referential integrity on delete.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Create routers `engine/agent/router`, `engine/tool/router`, `engine/task/router` for top‑level CRUD.
- Implement UC for each and reuse shared validators; add missing Task validator.
- DELETE returns `409 Conflict` when referenced by any workflow (Problem Details lists references).
</requirements>

## Subtasks

- [ ] 8.1 Implement list/get for each resource using UC/store.
- [ ] 8.2 Implement upsert (PUT) with strong `If-Match` and `ETag`/`Location`.
- [ ] 8.3 Implement delete with reference checks.

## Sequencing

- Blocked by: 6.0
- Unblocks: 9.0, 10.0
- Parallelizable: Yes (per resource)

## Implementation Details

Follow workflow patterns; preserve existing nested read endpoints.

### Relevant Files

- `engine/{agent,tool,task}/router/*`
- `engine/{agent,tool,task}/uc/*`

### Dependent Files

- `engine/workflow/router/workflows.go`

## Success Criteria

- Top‑level endpoints available and documented; deletes enforce referential integrity.
