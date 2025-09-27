---
status: pending
parallelizable: true
blocked_by: ["7.0", "9.0"]
---

<task_context>
<domain>docs/examples</domain>
<type>documentation</type>
<scope>developer_experience</scope>
<complexity>low</complexity>
<dependencies>docs,OpenAPI</dependencies>
<unblocks></unblocks>
</task_context>

# Task 10.0: Developer guides and cURL examples

## Overview

Provide concise developer documentation and cURL snippets covering sync workflow, direct agent/task (sync/async), and status polling, matching the final OpenAPI.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Add runnable cURL examples for 200, 202 (`Location`), 408, and typical 4xx errors.
- Ensure examples include `X-Idempotency-Key` and show dedupe behavior.
- Cross-link to API reference pages.
</requirements>

## Subtasks

- [ ] 10.1 Add cURL snippets for each endpoint
- [ ] 10.2 Add troubleshooting notes (timeouts, duplicates, payload caps)
- [ ] 10.3 Final pass for consistency with PRD and Tech Spec

## Sequencing

- Blocked by: 7.0, 9.0
- Parallelizable: Yes

## Implementation Details

Place examples under docs content adjacent to API pages; keep them minimal and copy-paste friendly.

### Relevant Files

- `docs/content/docs/api/*`

### Dependent Files

- `swagger.json`

## Success Criteria

- Clear guides exist; examples work against local dev server
- Lints/tests pass
