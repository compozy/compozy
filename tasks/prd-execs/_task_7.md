---
status: completed
parallelizable: true
blocked_by: ["3.0", "4.0", "5.0", "6.0"]
---

<task_context>
<domain>docs/api</domain>
<type>documentation</type>
<scope>api_docs</scope>
<complexity>medium</complexity>
<dependencies>OpenAPI,docs</dependencies>
<unblocks>10.0</unblocks>
</task_context>

# Task 7.0: Update OpenAPI and docs (examples, headers)

## Overview

Document the new endpoints in OpenAPI and surface them in the docs site. Include examples for 200/202/400/404/408/409.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Update `swagger.json` with routes, schemas, and headers (`X-Idempotency-Key`, `Location`).
- Regenerate docs via `docs/scripts/generate-openapi.ts`.
- Add cURL snippets for sync/async and status flows.
- Ensure consistent response envelope and problem codes.
</requirements>

## Subtasks

- [x] 7.1 Extend OpenAPI for new endpoints and components
- [x] 7.2 Generate docs and verify navigation
- [x] 7.3 Add examples with headers and sample payloads

## Sequencing

- Blocked by: 3.0, 4.0, 5.0, 6.0
- Unblocks: 10.0
- Parallelizable: Yes

## Implementation Details

Follow PRD endpoints and Tech Spec request/response shapes. Ensure `Location` header appears on all async 202 responses.

### Relevant Files

- `swagger.json`
- `docs/scripts/generate-openapi.ts`
- `docs/content/docs/api/*`

### Dependent Files

- `engine/*/router/*.go` (for source of truth)

## Success Criteria

- Docs compile with new endpoints and examples
- Lints/tests pass
