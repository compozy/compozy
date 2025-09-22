---
status: completed
parallelizable: true
blocked_by: ["2.0", "3.0", "4.0"]
---

<task_context>
<domain>docs/swagger</domain>
<type>documentation</type>
<scope>api_docs</scope>
<complexity>low</complexity>
<dependencies>http_server</dependencies>
<unblocks>"6.0","9.0"</unblocks>
</task_context>

# Task 5.0: Swagger Updates for Workflows (Headers, Examples, RFC7807)

## Overview

Annotate new and updated workflow endpoints and regenerate Swagger. Include header docs (`If-Match`, `ETag`, `Location`, `Link`, `RateLimit-*`) and Problem Details examples for error codes.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Add tags `workflows`, `executions` and annotate all request/response shapes and headers.
- Include examples for 201/200/204 and 4xx/5xx Problem Details.
- Regenerate `swagger.yaml/json` and commit.
</requirements>

## Subtasks

- [x] 5.1 Add/adjust annotations in handlers.
- [x] 5.2 Regenerate Swagger and verify output.
- [x] 5.3 Ensure pagination docs show `Link`, `page.next_cursor`, and `page.prev_cursor` with keyset cursor examples.

## Sequencing

- Blocked by: 2.0, 3.0, 4.0
- Unblocks: 6.0, 9.0
- Parallelizable: Yes

## Implementation Details

Run `make swagger` (or project‑specific target) to regenerate docs.

### Relevant Files

- `engine/workflow/router/workflows.go`
- `docs/swagger.yaml`
- `docs/swagger.json`

### Dependent Files

- `docs/docs.go`

## Success Criteria

- Swagger includes all headers and Problem Details examples for workflow routes.
