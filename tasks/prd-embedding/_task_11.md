---
status: pending
parallelizable: false
blocked_by: ["10.0"]
---

<task_context>
<domain>cli/cmd/knowledge</domain>
<type>implementation|testing</type>
<scope>cli</scope>
<complexity>low</complexity>
<dependencies>http_server</dependencies>
<unblocks>"13.0","14.0","15.0"</unblocks>
</task_context>

# Task 11.0: CLI Commands

## Overview
Add `compozy knowledge` command group with subcommands: list, get, apply, delete, ingest, query. Align flags and JSON output with API.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Use existing resource helpers/patterns; stable JSON for golden tests.
- Include minimal unit tests for flag parsing and output shaping (mock API client).
- Run `make fmt && make lint && make test` before completion.
</requirements>

## Subtasks
- [ ] 11.1 Implement CLI group and subcommands
- [ ] 11.2 Unit tests (w/ mocked API client)

## Sequencing
- Blocked by: 10.0
- Unblocks: 13.0, 14.0, 15.0
- Parallelizable: No

## Implementation Details
Ensure pagination flags match server; support `--output json`.

### Relevant Files
- `cli/cmd/knowledge/*`
- `cli/helpers/*`

### Dependent Files
- `test/integration/knowledge/cli_test.go`

## Success Criteria
- CLI compiles; unit tests pass; outputs are stable for goldens.
