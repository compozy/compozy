## markdown

## status: pending

<task_context>
<domain>engine/infra/server/router</domain>
<type>implementation</type>
<scope>configuration</scope>
<complexity>low</complexity>
<dependencies>http_server</dependencies>
</task_context>

# Task 12.0: Introduce Shared DTO Scaffolding & Naming

## Overview

Create common DTOs and naming conventions used by all typed top‑level resources.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Add `PageInfoDTO` in a shared location (e.g., `engine/infra/server/router/page_dto.go`).
- Document naming: `<Resource>DTO`, `<Resource>ListItem`, `<Resource>ListResponse`.
- Ensure DTOs/mappers are pure (no `gin` imports); keep envelope `router.Response`.
- Add minimal examples via struct tags where feasible.
</requirements>

## Subtasks

- [ ] 12.1 Add `PageInfoDTO { limit,total,next_cursor,prev_cursor }`.
- [ ] 12.2 Document naming & mapper purity in code comments.
- [ ] 12.3 Add skeleton DTO files to `engine/tool/router/dto.go` for the pilot (empty fields ok; filled in Task 13.0).

## Implementation Details

Place PageInfoDTO in a neutral package used by all routers; ensure no package cycles.

### Relevant Files

- `engine/infra/server/router/page_dto.go`
- `engine/tool/router/dto.go` (skeleton)

### Dependent Files

- `engine/*/router/*.go`

## Success Criteria

- `PageInfoDTO` available and imported by pilot.
- Naming documented and consistent.
