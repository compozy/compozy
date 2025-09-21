---
status: pending
parallelizable: true
blocked_by: []
---

<task_context>
<domain>engine/infra/docs</domain>
<type>validation</type>
<scope>pipeline|documentation</scope>
<complexity>low</complexity>
<dependencies>http_server</dependencies>
<unblocks>"2.0","3.0","4.0","5.0"</unblocks>
</task_context>

# Task 1.0: Baseline & Swagger Pipeline Check

## Overview

Establish a clean baseline and validate the Swagger/OpenAPI generation pipeline before wider changes.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Run `make lint` and `make test`; capture baseline.
- Verify Swagger generation from annotations builds without warnings; confirm tags and grouping are correct.
- Identify any blockers in codegen/templates for documenting headers: `If-Match`, `ETag`, `Location`, `Link`, `RateLimit-*`.
</requirements>

## Subtasks

- [ ] 1.1 Run baseline `make lint && make test` and record versions.
- [ ] 1.2 Run Swagger generation and confirm updated docs are produced.
- [ ] 1.3 Ensure Problem Details schemas are present in OpenAPI.

## Sequencing

- Blocked by: —
- Unblocks: 2.0, 3.0, 4.0, 5.0
- Parallelizable: Yes

## Implementation Details

Use existing Makefile targets. Validate header documentation capability in Swagger annotations.

### Relevant Files

- `docs/swagger.yaml`
- `docs/swagger.json`
- `docs/docs.go`

### Dependent Files

- `engine/infra/server/reg_components.go`
- `engine/infra/server/router/*`

## Success Criteria

- Lint/tests pass on current branch.
- Swagger artifacts regenerate successfully.
- Header and Problem Details examples are supported in annotations.
